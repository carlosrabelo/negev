package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/carlosrabelo/negev/negev/internal/application/services"
	"github.com/carlosrabelo/negev/negev/internal/infrastructure/config"
	"github.com/carlosrabelo/negev/negev/internal/infrastructure/transport"

	_ "github.com/carlosrabelo/negev/negev/internal/platform/dmos"
	_ "github.com/carlosrabelo/negev/negev/internal/platform/ios"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	target := flag.String("target", "", "Switch IP address (required)")
	configPath := flag.String("config", "", "Path to YAML config file")
	write := flag.Bool("write", false, "Apply changes (disables sandbox)")
	verbose := flag.Int("verbose", 0, "Verbosity level: 0=none, 1=debug, 2=raw, 3=both")
	createVLANs := flag.Bool("create-vlans", false, "Synchronize VLANs (create missing, delete extras)")
	showVersion := flag.Bool("version", false, "Show version and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "VLAN automation tool for network switches.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("Negev %s (built %s)\n", version, buildTime)
		return
	}

	if *target == "" {
		fmt.Fprintf(os.Stderr, "ERROR: --target is required\n\n")
		flag.Usage()
		os.Exit(1)
	}

	if *verbose < 0 || *verbose > 3 {
		fmt.Fprintf(os.Stderr, "ERROR: --verbose must be 0-3\n\n")
		flag.Usage()
		os.Exit(1)
	}

	cfgPath := *configPath
	if cfgPath == "" {
		var err error
		cfgPath, err = config.FindPath("config.yaml", *verbose)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(1)
		}
	}

	cfg, err := config.Load(cfgPath, *target, !*write, *verbose, *createVLANs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: failed to load config: %v\n", err)
		os.Exit(1)
	}

	defer transport.CloseAll()

	svc := services.NewVLANApplicationService(cfg, *target)
	if err := svc.Run(!*write, *verbose, *createVLANs); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}
