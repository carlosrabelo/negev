package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
)

type SwitchClient interface {
	Connect() error
	Disconnect()
	ExecuteCommand(cmd string) (string, error)
	IsConnected() bool
}

var (
	clientCache   = make(map[string]SwitchClient)
	clientCacheMu sync.Mutex
)

func cacheKey(config SwitchConfig) string {
	keyData := struct {
		Transport      string
		Target         string
		Username       string
		Password       string
		EnablePassword string
	}{
		Transport:      config.Transport,
		Target:         config.Target,
		Username:       config.Username,
		Password:       config.Password,
		EnablePassword: config.EnablePassword,
	}
	bytes, _ := json.Marshal(keyData)
	hash := sha256.Sum256(bytes)
	return hex.EncodeToString(hash[:])
}

func getSwitchClient(config SwitchConfig) SwitchClient {
	clientCacheMu.Lock()
	defer clientCacheMu.Unlock()
	key := cacheKey(config)
	if client, exists := clientCache[key]; exists {
		return client
	}
	client := newSwitchClient(config)
	clientCache[key] = client
	return client
}

func CloseAllClients() {
	clientCacheMu.Lock()
	defer clientCacheMu.Unlock()
	for key, client := range clientCache {
		client.Disconnect()
		delete(clientCache, key)
	}
}

func newSwitchClient(config SwitchConfig) SwitchClient {
	if config.Transport == "ssh" {
		return NewSSHClient(config)
	}
	return NewTelnetClient(config)
}
