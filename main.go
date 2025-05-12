package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

// main is the entry point of the program
func main() {
	yamlFile := flag.String("y", "config.yaml", "YAML configuration file")
	write := flag.Bool("w", false, "Apply changes (disables sandbox mode)")
	verbose := flag.Bool("v", false, "Enable debug logs (DEBUG: messages)")
	extra := flag.Bool("e", false, "Enable display of raw switch outputs")
	daemon := flag.Bool("d", false, "Enable daemon mode for SNMP traps")
	host := flag.String("t", "", "Switch target (must match a target in YAML, required)")
	skipVlanCheck := flag.Bool("s", false, "Skip VLAN existence check (use with caution)")
	createVLANs := flag.Bool("c", false, "Create missing VLANs on the switch")
	flag.Parse()

	// Check if -t parameter is provided, but only if not in daemon mode
	if !*daemon && *host == "" {
		fmt.Fprintf(os.Stderr, "Error: the -t parameter is required outside daemon mode. Specify the switch target with -t <target>\n")
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
				if *verbose {
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
	cfg, err := loadConfig(configPath, *host, *write, *verbose, *extra, *skipVlanCheck, *createVLANs)
	if err != nil {
		log.Fatal(err)
	}

	if *daemon {
		// Daemon mode: listen for SNMP traps
		fmt.Println("Starting Negev in daemon mode for SNMP traps...")
		err = RunSNMP(cfg, *verbose, *extra)
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	// Process only the switch specified by -t
	found := false
	for _, switchCfg := range cfg.Switches {
		if switchCfg.Target == *host {
			found = true
			fmt.Printf("Starting Negev for switch %s\n", switchCfg.Target)
			sm := NewSwitchManager(switchCfg, *cfg)
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
