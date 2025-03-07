package transport

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

type cachedClient interface {
	Disconnect()
}

var (
	clientCache = make(map[string]cachedClient)
	cacheMu     sync.Mutex
)

func cacheSize() int {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	return len(clientCache)
}

func putCached(key string, c cachedClient) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	clientCache[key] = c
}

func getCached(key string) cachedClient {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	return clientCache[key]
}


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
		slog.Warn("Failed to marshal cache key", "error", err, "target", cfg.Target)
		return ""
	}
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}
