package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/carlosrabelo/negev/negev/internal/application/services"
	"github.com/carlosrabelo/negev/negev/internal/domain/entities"

	_ "github.com/carlosrabelo/negev/negev/internal/platform/comware"
	_ "github.com/carlosrabelo/negev/negev/internal/platform/dm1200"
	_ "github.com/carlosrabelo/negev/negev/internal/platform/dmswitch"
	_ "github.com/carlosrabelo/negev/negev/internal/platform/ios"
	_ "github.com/carlosrabelo/negev/negev/internal/platform/nxos"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("negev", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	target := fs.String("target", "", "Switch IP address (required)")
	configPath := fs.String("config", "", "Path to YAML config file")
	write := fs.Bool("write", false, "Apply changes (disables sandbox)")
	verbose := fs.Int("verbose", 0, "Verbosity level: 0=none, 1=debug, 2=raw, 3=both")
	createVLANs := fs.Bool("create-vlans", false, "Synchronize VLANs (create missing, delete extras)")
	showVersion := fs.Bool("version", false, "Show version and exit")
	fs.Usage = func() { printUsage(fs) }

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *showVersion {
		fmt.Printf("Negev %s (built %s)\n", version, buildTime)
		return 0
	}
	if *target == "" {
		fmt.Fprintf(os.Stderr, "ERROR: --target is required\n\n")
		fs.Usage()
		return 1
	}

	result, err := services.RunTarget(*target, *configPath, *write, *verbose, *createVLANs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		return 1
	}
	return exitStatus(*write, result)
}

// exitStatus maps run outcome to process exit codes:
// 0 success (no changes or written), 2 sandbox simulated changes.
func exitStatus(write bool, result entities.RunResult) int {
	if !write && result.HasChanges() {
		return 2
	}
	return 0
}

func printUsage(fs *flag.FlagSet) {
	name := "negev"
	if fs != nil && fs.Name() != "" {
		name = fs.Name()
	}
	if len(os.Args) > 0 {
		name = os.Args[0]
	}
	fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\nVLAN automation tool for network switches.\n\nOptions:\n", name)
	fmt.Fprintf(os.Stderr, "  --target <ip>\n        Switch IP address (required)\n")
	fmt.Fprintf(os.Stderr, "  --config <path>\n        Path to YAML config file\n")
	fmt.Fprintf(os.Stderr, "  --write\n        Apply changes (disables sandbox)\n")
	fmt.Fprintf(os.Stderr, "  --verbose <0-3>\n        Verbosity level: 0=none, 1=debug, 2=raw, 3=both (default 0)\n")
	fmt.Fprintf(os.Stderr, "  --create-vlans\n        Synchronize VLANs (create missing, delete extras)\n")
	fmt.Fprintf(os.Stderr, "  --version\n        Show version and exit\n")
}
