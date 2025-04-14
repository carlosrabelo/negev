package main

import (
	"flag"
	"fmt"
	"os"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	flag.Parse()
	fmt.Printf("Negev %s (built %s)\n", version, buildTime)
	os.Exit(0)
}
