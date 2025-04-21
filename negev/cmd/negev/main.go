package main

import "os"

var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	os.Exit(run(os.Args[1:]))
}
