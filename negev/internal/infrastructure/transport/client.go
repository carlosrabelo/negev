package transport

import (
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

var _ = entities.SwitchConfig{}
