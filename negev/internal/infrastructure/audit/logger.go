package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/carlosrabelo/negev/negev/internal/domain/ports"
)

// Logger writes JSON Lines audit records with mode 0644.
type Logger struct {
	path string
	mu   sync.Mutex
	now  func() time.Time
	user func() string
}

// New opens (or creates) an audit log at path. Parent directories are created as needed.
func New(path string) (*Logger, error) {
	if path == "" {
		return nil, fmt.Errorf("audit log path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create audit log directory: %w", err)
	}
	// Ensure the file exists with 0644 even before the first append.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open audit log: %w", err)
	}
	_ = f.Close()
	return &Logger{
		path: path,
		now:  func() time.Time { return time.Now().UTC() },
		user: currentUser,
	}, nil
}

type jsonEntry struct {
	Timestamp  string `json:"timestamp"`
	User       string `json:"user"`
	Target     string `json:"target"`
	SwitchName string `json:"switch_name"`
	Port       string `json:"port"`
	MAC        string `json:"mac"`
	OldVLAN    int    `json:"old_vlan"`
	NewVLAN    int    `json:"new_vlan"`
	Action     string `json:"action"`
	Sandbox    bool   `json:"sandbox"`
	Status     string `json:"status"`
}

// AppendPortVLAN writes one JSONL record and syncs to disk.
func (l *Logger) AppendPortVLAN(entry ports.PortVLANAudit) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if entry.Timestamp == "" {
		entry.Timestamp = l.now().Format(time.RFC3339)
	}
	if entry.User == "" {
		entry.User = l.user()
	}
	if entry.Action == "" {
		entry.Action = "SET_PORT_VLAN"
	}

	rec := jsonEntry{
		Timestamp:  entry.Timestamp,
		User:       entry.User,
		Target:     entry.Target,
		SwitchName: entry.SwitchName,
		Port:       entry.Port,
		MAC:        entry.MAC,
		OldVLAN:    entry.OldVLAN,
		NewVLAN:    entry.NewVLAN,
		Action:     entry.Action,
		Sandbox:    entry.Sandbox,
		Status:     entry.Status,
	}
	line, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	line = append(line, '\n')

	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync audit log: %w", err)
	}
	return nil
}

func currentUser() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	if u := os.Getenv("USERNAME"); u != "" {
		return u
	}
	return "unknown"
}

var _ ports.Auditor = (*Logger)(nil)
