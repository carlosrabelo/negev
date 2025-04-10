package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/carlosrabelo/negev/negev/internal/application/services"
	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
	"github.com/carlosrabelo/negev/negev/internal/infrastructure/config"

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

	target := fs.String("target", "", "Switch IP(s) and/or name(s), comma-separated")
	all := fs.Bool("all", false, "Run against all switches in the config")
	group := fs.String("group", "", "Run against switches with the given tag")
	concurrency := fs.Int("concurrency", services.DefaultConcurrency, "Max concurrent switch runs")
	configPath := fs.String("config", "", "Path to YAML config file")
	write := fs.Bool("write", false, "Apply changes (disables sandbox)")
	verbose := fs.Int("verbose", 0, "Verbosity level: 0=none, 1=debug, 2=raw, 3=both")
	createVLANs := fs.Bool("create-vlans", false, "Synchronize VLANs (create missing, delete extras)")
	askPass := fs.Bool("ask-pass", false, "Prompt interactively for switch passwords")
	backup := fs.Bool("backup", false, "Backup running-config even in sandbox mode")
	backupDir := fs.String("backup-dir", "", "Directory for configuration backups (default: ./backups or YAML backup_dir)")
	showVersion := fs.Bool("version", false, "Show version and exit")
	fs.Usage = func() { printUsage(fs) }

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *showVersion {
		fmt.Printf("Negev %s (built %s)\n", version, buildTime)
		return 0
	}

	if *concurrency < 1 {
		fmt.Fprintf(os.Stderr, "ERROR: --concurrency must be >= 1\n")
		return 1
	}

	cfgPath := *configPath
	if cfgPath == "" {
		var err error
		cfgPath, err = config.FindPath("config.yaml", *verbose)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			return 1
		}
	}

	cfg, err := config.Load(cfgPath, "", *verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: failed to load config: %v\n", err)
		return 1
	}

	selected, err := services.ResolveTargets(cfg, services.TargetSelect{
		Target: *target,
		All:    *all,
		Group:  *group,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		if *target == "" && !*all && *group == "" {
			fs.Usage()
		}
		return 1
	}

	var ask services.AskPassFunc
	if *askPass {
		password, enable, err := promptAskPass()
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			return 1
		}
		ask = func() (string, string, error) { return password, enable, nil }
	}

	result := services.RunMulti(selected, services.MultiRunOptions{
		ConfigPath:  cfgPath,
		Write:       *write,
		Verbose:     *verbose,
		CreateVLANs: *createVLANs,
		Concurrency: *concurrency,
		AskPass:     ask,
		Backup: services.BackupOpts{
			Force: *backup,
			Dir:   *backupDir,
		},
	})
	return result.ExitCode(*write)
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
	if len(os.Args) > 0 {
		name = os.Args[0]
	}
	fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\nVLAN automation tool for network switches.\n\nOptions:\n", name)
	fmt.Fprintf(os.Stderr, "  --target <ip|name>[,...]\n        Switch IP(s) and/or name(s), comma-separated\n")
	fmt.Fprintf(os.Stderr, "  --all\n        Run against all switches in the config\n")
	fmt.Fprintf(os.Stderr, "  --group <tag>\n        Run against switches with the given tag\n")
	fmt.Fprintf(os.Stderr, "  --concurrency <n>\n        Max concurrent switch runs (default %d)\n", services.DefaultConcurrency)
	fmt.Fprintf(os.Stderr, "  --config <path>\n        Path to YAML config file\n")
	fmt.Fprintf(os.Stderr, "  --write\n        Apply changes (disables sandbox)\n")
	fmt.Fprintf(os.Stderr, "  --verbose <0-3>\n        Verbosity level: 0=none, 1=debug, 2=raw, 3=both (default 0)\n")
	fmt.Fprintf(os.Stderr, "  --create-vlans\n        Synchronize VLANs (create missing, delete extras)\n")
	fmt.Fprintf(os.Stderr, "  --ask-pass\n        Prompt interactively for switch passwords\n")
	fmt.Fprintf(os.Stderr, "  --backup\n        Backup running-config even in sandbox mode\n")
	fmt.Fprintf(os.Stderr, "  --backup-dir <dir>\n        Directory for configuration backups (default: ./backups)\n")
	fmt.Fprintf(os.Stderr, "  --version\n        Show version and exit\n")
}
