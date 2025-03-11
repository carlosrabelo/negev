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
	showVersion := flag.Bool("version", false, "Show version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("Negev %s (built %s)\n", version, buildTime)
		return
	}
	if err := services.RunTarget("", "", false, 0, false); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}
