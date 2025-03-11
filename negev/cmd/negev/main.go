package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/carlosrabelo/negev/negev/internal/application/services"
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
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\nVLAN automation tool for network switches.\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if *showVersion {
		fmt.Printf("Negev %s (built %s)\n", version, buildTime)
		return
	}
	if err := services.RunTarget(*target, *configPath, *write, *verbose, *createVLANs); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}
