package transport

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"

	"github.com/carlosrabelo/negev/internal/config"
)

// Client abstracts a switch transport session
type Client interface {
	Connect() error
	Disconnect()
	ExecuteCommand(cmd string) (string, error)
	IsConnected() bool
}

var (
	clientCache   = make(map[string]Client)
	clientCacheMu sync.Mutex
)

func cacheKey(cfg config.SwitchConfig) string {
	keyData := struct {
		Transport      string
		Target         string
		Username       string
		Password       string
		EnablePassword string
	}{
		Transport:      cfg.Transport,
		Target:         cfg.Target,
		Username:       cfg.Username,
		Password:       cfg.Password,
		EnablePassword: cfg.EnablePassword,
	}
	bytes, _ := json.Marshal(keyData)
	hash := sha256.Sum256(bytes)
	return hex.EncodeToString(hash[:])
}

// Get returns a cached client for the provided configuration or creates a new one
func Get(cfg config.SwitchConfig) Client {
	clientCacheMu.Lock()
	defer clientCacheMu.Unlock()
	key := cacheKey(cfg)
	if client, exists := clientCache[key]; exists {
		return client
	}
	client := newClient(cfg)
	clientCache[key] = client
	return client
}

// CloseAll releases every cached client session
func CloseAll() {
	clientCacheMu.Lock()
	defer clientCacheMu.Unlock()
	for key, client := range clientCache {
		client.Disconnect()
		delete(clientCache, key)
	}
}

func newClient(cfg config.SwitchConfig) Client {
	if cfg.Transport == "ssh" {
		return NewSSHClient(cfg)
	}
	return NewTelnetClient(cfg)
}
