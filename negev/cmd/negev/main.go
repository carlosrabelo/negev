package main

import (
	"fmt"
	"os"

	"github.com/carlosrabelo/negev/negev/internal/application/services"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	if err := services.RunTarget("", "", false, 0, false); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}
