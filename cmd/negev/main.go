package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/carlosrabelo/negev/internal/config"
	"github.com/carlosrabelo/negev/internal/switchmanager"
	"github.com/carlosrabelo/negev/internal/transport"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	yamlFile := flag.String("y", "config.yaml", "YAML configuration file")
	write := flag.Bool("w", false, "Apply changes (disables sandbox mode)")
	verbosity := flag.Int("v", 0, "Verbosity level: 0=none, 1=debug logs, 2=raw switch output, 3=debug+raw output")
	host := flag.String("t", "", "Switch target (must match a target in YAML, required)")
	skipVlanCheck := flag.Bool("s", false, "Skip VLAN existence check (use with caution)")
	createVLANs := flag.Bool("c", false, "Create missing VLANs on the switch")
	flag.Parse()

	fmt.Printf("Negev %s (built %s)\n", version, buildTime)

	// Validate verbosity level
	if *verbosity < 0 || *verbosity > 3 {
		fmt.Fprintf(os.Stderr, "Error: -v must be 0, 1, 2, or 3\n")
		flag.Usage()
		os.Exit(1)
	}

	// Check if -t parameter is provided
	if *host == "" {
		fmt.Fprintf(os.Stderr, "Error: the -t parameter is required. Specify the switch target with -t <target>\n")
		flag.Usage()
		os.Exit(1)
	}

	// Determine the configuration file path
	configPath := *yamlFile
	if *yamlFile == "config.yaml" {
		// If the default path is not overridden, search in specific locations
		possiblePaths := []string{
			filepath.Join(".", "config.yaml"), // Local directory
		}

		if runtime.GOOS == "linux" {
			// Linux: Add user and global configuration paths
			if userConfigDir, err := os.UserConfigDir(); err == nil {
				possiblePaths = append(possiblePaths, filepath.Join(userConfigDir, "negev", "config.yaml"))
			}
			possiblePaths = append(possiblePaths, "/etc/negev/config.yaml")
		} else if runtime.GOOS == "windows" {
			// Windows: Add user (%APPDATA%) and global (%ProgramData%) paths
			if appDataDir := os.Getenv("APPDATA"); appDataDir != "" {
				possiblePaths = append(possiblePaths, filepath.Join(appDataDir, "negev", "config.yaml"))
			}
			if programDataDir := os.Getenv("ProgramData"); programDataDir != "" {
				possiblePaths = append(possiblePaths, filepath.Join(programDataDir, "negev", "config.yaml"))
			}
		}

		// Try to find the first existing configuration file
		found := false
		for _, path := range possiblePaths {
			if _, err := os.Stat(path); err == nil {
				configPath = path
				found = true
				if *verbosity >= 1 {
					fmt.Printf("DEBUG: Configuration file found at %s\n", path)
				}
				break
			}
		}

		if !found {
			if runtime.GOOS == "windows" {
				log.Fatal("Error: No config.yaml file found in ./, %APPDATA%\\negev\\, or %ProgramData%\\negev\\")
			} else {
				log.Fatal("Error: No config.yaml file found in ./, ~/.config/negev/, or /etc/negev/")
			}
		}
	}

	// Load configuration, passing the target switch IP for log filtering
	cfg, err := config.Load(configPath, *host, *write, *verbosity, *skipVlanCheck, *createVLANs)
	if err != nil {
		log.Fatal(err)
	}
	defer transport.CloseAll()

	// Process only the switch specified by -t
	found := false
	for _, switchCfg := range cfg.Switches {
		if switchCfg.Target == *host {
			found = true
			fmt.Printf("Starting Negev for switch %s\n", switchCfg.Target)
			sm := switchmanager.New(switchCfg, *cfg)
			err = sm.ProcessPorts()
			if err != nil {
				log.Printf("Error processing switch %s: %v", switchCfg.Target, err)
			}
			break
		}
	}
	if !found {
		fmt.Fprintf(os.Stderr, "Error: target %s not registered in the YAML configuration\n", *host)
		os.Exit(1)
	}
}
