package transport

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

type Client interface {
	Connect() error
	Disconnect()
	ExecuteCommand(cmd string) (string, error)
	IsConnected() bool
	SetSessionProfile(profile entities.SessionProfile)
}

type AuthConfigurable interface {
	SetAuthSequence([]entities.AuthPrompt)
}

var (
	clientCache = make(map[string]Client)
	cacheMu     sync.Mutex
)

type cacheEntry struct {
	Transport         string `json:"transport"`
	Target            string `json:"target"`
	Username          string `json:"username"`
	Password          string `json:"password"`
	EnablePassword    string `json:"enable_password"`
	JumpAddr          string `json:"jump_addr,omitempty"`
	JumpUsername      string `json:"jump_username,omitempty"`
	JumpPassword      string `json:"jump_password,omitempty"`
	JumpKeyFile       string `json:"jump_key_file,omitempty"`
	JumpKeyPassphrase string `json:"jump_key_passphrase,omitempty"`
	JumpKnownHosts    string `json:"jump_known_hosts,omitempty"`
}

func cacheKey(cfg entities.SwitchConfig) string {
	entry := cacheEntry{
		Transport:      cfg.Transport,
		Target:         cfg.Target,
		Username:       cfg.Username,
		Password:       cfg.Password,
		EnablePassword: cfg.EnablePassword,
	}
	if cfg.JumpHost.IsEnabled() {
		entry.JumpAddr = cfg.JumpHost.Addr()
		entry.JumpUsername = cfg.JumpHost.Username
		entry.JumpPassword = cfg.JumpHost.Password
		entry.JumpKeyFile = cfg.JumpHost.KeyFile
		entry.JumpKeyPassphrase = cfg.JumpHost.KeyPassphrase
		entry.JumpKnownHosts = cfg.JumpHost.KnownHosts
	}
	data, err := json.Marshal(entry)
	if err != nil {
		slog.Warn("Failed to marshal cache key", "error", err, "target", cfg.Target)
		return ""
	}
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

func GetClient(cfg entities.SwitchConfig) Client {
	key := cacheKey(cfg)
	if key == "" {
		return newClient(cfg)
	}

	cacheMu.Lock()
	defer cacheMu.Unlock()

	if c, ok := clientCache[key]; ok {
		return c
	}

	c := newClient(cfg)
	clientCache[key] = c
	return c
}

func newClient(cfg entities.SwitchConfig) Client {
	if cfg.Transport == "ssh" {
		return NewSSHClient(cfg)
	}
	return NewTelnetClient(cfg)
}

// CloseClient disconnects and removes the cached client for cfg without
// closing other targets' clients or shared jump host tunnels.
func CloseClient(cfg entities.SwitchConfig) {
	key := cacheKey(cfg)
	if key == "" {
		return
	}

	cacheMu.Lock()
	c, ok := clientCache[key]
	if ok {
		delete(clientCache, key)
	}
	cacheMu.Unlock()

	if ok && c != nil {
		c.Disconnect()
	}
}

func CloseAll() {
	cacheMu.Lock()
	clients := make([]Client, 0, len(clientCache))
	for key, c := range clientCache {
		clients = append(clients, c)
		delete(clientCache, key)
	}
	cacheMu.Unlock()

	for _, c := range clients {
		c.Disconnect()
	}
	closeJumpPool()
}
