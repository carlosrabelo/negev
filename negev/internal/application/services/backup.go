package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
	"github.com/carlosrabelo/negev/negev/internal/infrastructure/transport"
	"github.com/carlosrabelo/negev/negev/internal/platform"
)

const DefaultBackupDir = "backups"

// BackupOpts controls pre-write running-config backups.
type BackupOpts struct {
	Force bool   // --backup: backup even in sandbox
	Dir   string // --backup-dir (overrides YAML backup_dir)
}

// Required reports whether a backup must run for this invocation.
func (o BackupOpts) Required(write bool) bool {
	return write || o.Force
}

// ResolveDir picks --backup-dir, then YAML backup_dir, then ./backups.
func (o BackupOpts) ResolveDir(yamlDir string) string {
	if strings.TrimSpace(o.Dir) != "" {
		return o.Dir
	}
	if strings.TrimSpace(yamlDir) != "" {
		return yamlDir
	}
	return DefaultBackupDir
}

// WriteRunningConfigBackup writes content to dir with mode 0600.
// Filename: <label>_<UTC timestamp>.cfg
func WriteRunningConfigBackup(dir, label, content string, ts time.Time) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}
	name := sanitizeBackupLabel(label) + "_" + ts.UTC().Format("20060102T150405Z") + ".cfg"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("write backup: %w", err)
	}
	return path, nil
}

func sanitizeBackupLabel(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return "switch"
	}
	var b strings.Builder
	for _, r := range label {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', ' ':
			b.WriteByte('_')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func backupLabel(sw entities.SwitchConfig) string {
	if sw.Name != "" {
		return sw.Name
	}
	return sw.Target
}

func takePreWriteBackup(adapter *transport.SwitchAdapter, driver platform.SwitchDriver, sw entities.SwitchConfig, dir string) (string, error) {
	if err := adapter.Connect(); err != nil {
		return "", fmt.Errorf("connect for backup: %w", err)
	}
	defer adapter.Disconnect()

	content, err := driver.GetRunningConfig(adapter)
	if err != nil {
		return "", fmt.Errorf("get running-config: %w", err)
	}
	path, err := WriteRunningConfigBackup(dir, backupLabel(sw), content, time.Now().UTC())
	if err != nil {
		return "", err
	}
	return path, nil
}
