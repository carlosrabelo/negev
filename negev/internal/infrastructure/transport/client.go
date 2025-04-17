package transport

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

type Client interface {
	Connect() error
	Disconnect()
	ExecuteCommand(cmd string) (string, error)
	IsConnected() bool
}

type AuthConfigurable interface {
	SetAuthSequence([]entities.AuthPrompt)
}

var (
	clientCache = make(map[string]Client)
	cacheMu     sync.Mutex
)

type cacheEntry struct {
	Transport      string `json:"transport"`
	Target         string `json:"target"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	EnablePassword string `json:"enable_password"`
}

func cacheKey(cfg entities.SwitchConfig) string {
	entry := cacheEntry{
		Transport:      cfg.Transport,
		Target:         cfg.Target,
		Username:       cfg.Username,
		Password:       cfg.Password,
		EnablePassword: cfg.EnablePassword,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		log.Printf("WARNING: failed to marshal cache key: %v", err)
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

func CloseAll() {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	for key, c := range clientCache {
		c.Disconnect()
		delete(clientCache, key)
	}
}
