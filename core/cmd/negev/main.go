package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/carlosrabelo/negev/core/application/services"
	"github.com/carlosrabelo/negev/core/infrastructure/config"
	"github.com/carlosrabelo/negev/core/infrastructure/transport"
	"github.com/carlosrabelo/negev/core/platform"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  --create-vlans     Sync VLANs on switch (create missing, delete extra)\n")
	fmt.Fprintf(os.Stderr, "  --target string    Switch target (must match a target in YAML, required)\n")
	fmt.Fprintf(os.Stderr, "  --verbose int      Verbosity level: 0=none, 1=debug logs, 2=raw switch output, 3=debug+raw output\n")
	fmt.Fprintf(os.Stderr, "  --write            Apply changes (disables sandbox mode)\n")
	fmt.Fprintf(os.Stderr, "  --config string    YAML configuration file (default \"config.yaml\")\n")
}

func main() {
	flag.Usage = printUsage
	yamlFile := flag.String("config", "config.yaml", "YAML configuration file")
	write := flag.Bool("write", false, "Apply changes (disables sandbox mode)")
	verbosity := flag.Int("verbose", 0, "Verbosity level: 0=none, 1=debug logs, 2=raw switch output, 3=debug+raw output")
	host := flag.String("target", "", "Switch target (must match a target in YAML, required)")
	createVLANs := flag.Bool("create-vlans", false, "Sync VLANs on switch (create missing, delete extra)")
	flag.Parse()

	fmt.Printf("Negev %s (built %s)\n", version, buildTime)

	// Validate verbosity level
	if *verbosity < 0 || *verbosity > 3 {
		fmt.Fprintf(os.Stderr, "Error: --verbose must be 0, 1, 2, or 3\n")
		flag.Usage()
		os.Exit(1)
	}

	// Check if --target parameter is provided
	if *host == "" {
		fmt.Fprintf(os.Stderr, "Error: the --target parameter is required. Specify the switch target with --target <target>\n")
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
	cfg, err := config.Load(configPath, *host, *write, *verbosity, *createVLANs)
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

			client := transport.Get(switchCfg)
			adapter := transport.NewSwitchAdapter(client)

			platformName := switchCfg.PlatformID()
			var driver platform.SwitchDriver
			if platformName == "auto" {
				detected, err := platform.Detect(adapter)
				if err != nil {
					log.Fatalf("Failed to auto-detect switch platform: %v", err)
				}
				driver = detected
				platformName = detected.Name()
				if switchCfg.IsDebugEnabled() {
					fmt.Printf("DEBUG: Platform auto-detected as %s\n", platformName)
				}
			} else {
				resolved, err := platform.Get(platformName)
				if err != nil {
					log.Fatalf("%v", err)
				}
				driver = resolved
			}

			switchCfg.Platform = platformName
			vlanAppService := services.NewVLANApplicationService(switchCfg, client, driver)
			err = vlanAppService.ProcessPorts()
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
