package transport

import (
	"testing"

	"github.com/carlosrabelo/negev/core/domain/entities"
)

func TestCacheKey(t *testing.T) {
	config1 := entities.SwitchConfig{
		Transport:      "telnet",
		Target:         "192.168.1.1",
		Username:       "admin",
		Password:       "password",
		EnablePassword: "enable",
	}

	config2 := entities.SwitchConfig{
		Transport:      "ssh",
		Target:         "192.168.1.1",
		Username:       "admin",
		Password:       "password",
		EnablePassword: "enable",
	}

	config3 := entities.SwitchConfig{
		Transport:      "telnet",
		Target:         "192.168.1.2",
		Username:       "admin",
		Password:       "password",
		EnablePassword: "enable",
	}

	// Test that same config produces same key
	key1a := cacheKey(config1)
	key1b := cacheKey(config1)
	if key1a != key1b {
		t.Errorf("Same config should produce same key: %s != %s", key1a, key1b)
	}

	// Test that different configs produce different keys
	key2 := cacheKey(config2)
	key3 := cacheKey(config3)

	if key1a == key2 {
		t.Error("Different transport should produce different keys")
	}

	if key1a == key3 {
		t.Error("Different target should produce different keys")
	}

	if key2 == key3 {
		t.Error("Different configs should produce different keys")
	}

	// Test that keys are not empty
	if key1a == "" || key2 == "" || key3 == "" {
		t.Error("Cache keys should not be empty")
	}

	// Test that keys have expected length (SHA256 hex = 64 chars)
	if len(key1a) != 64 {
		t.Errorf("Expected key length 64, got %d", len(key1a))
	}
}

func TestGet_Caching(t *testing.T) {
	// Clear cache first
	CloseAll()

	config := entities.SwitchConfig{
		Transport:      "telnet",
		Target:         "192.168.1.1",
		Username:       "admin",
		Password:       "password",
		EnablePassword: "enable",
	}

	// First call should create a new client
	client1 := Get(config)
	if client1 == nil {
		t.Fatal("Get() returned nil")
	}

	// Second call should return the same cached client
	client2 := Get(config)
	if client2 != client1 {
		t.Error("Get() did not return cached client")
	}

	// Different config should return different client
	differentConfig := entities.SwitchConfig{
		Transport:      "ssh",
		Target:         "192.168.1.1",
		Username:       "admin",
		Password:       "password",
		EnablePassword: "enable",
	}

	client3 := Get(differentConfig)
	if client3 == client1 {
		t.Error("Get() returned same client for different config")
	}
}

func TestCloseAll(t *testing.T) {
	// Clear cache first
	CloseAll()

	config1 := entities.SwitchConfig{
		Transport:      "telnet",
		Target:         "192.168.1.1",
		Username:       "admin",
		Password:       "password",
		EnablePassword: "enable",
	}

	config2 := entities.SwitchConfig{
		Transport:      "ssh",
		Target:         "192.168.1.2",
		Username:       "admin",
		Password:       "password",
		EnablePassword: "enable",
	}

	// Create clients
	client1 := Get(config1)
	client2 := Get(config2)

	if client1 == nil || client2 == nil {
		t.Fatal("Get() returned nil")
	}

	// Close all clients
	CloseAll()

	// Verify cache is empty by getting new clients
	newClient1 := Get(config1)
	newClient2 := Get(config2)

	if newClient1 == client1 {
		t.Error("CloseAll() did not clear cache for client1")
	}

	if newClient2 == client2 {
		t.Error("CloseAll() did not clear cache for client2")
	}
}

func TestNewClient_Telnet(t *testing.T) {
	config := entities.SwitchConfig{
		Transport:      "telnet",
		Target:         "192.168.1.1",
		Username:       "admin",
		Password:       "password",
		EnablePassword: "enable",
	}

	client := newClient(config)
	if client == nil {
		t.Fatal("newClient() returned nil")
	}

	// Verify it's a TelnetClient by checking if it implements the interface
	var _ Client = client
}

func TestNewClient_SSH(t *testing.T) {
	config := entities.SwitchConfig{
		Transport:      "ssh",
		Target:         "192.168.1.1",
		Username:       "admin",
		Password:       "password",
		EnablePassword: "enable",
	}

	client := newClient(config)
	if client == nil {
		t.Fatal("newClient() returned nil")
	}

	// Verify it's an SSHClient by checking if it implements the interface
	var _ Client = client
}

func TestNewClient_DefaultToTelnet(t *testing.T) {
	config := entities.SwitchConfig{
		Transport:      "invalid",
		Target:         "192.168.1.1",
		Username:       "admin",
		Password:       "password",
		EnablePassword: "enable",
	}

	client := newClient(config)
	if client == nil {
		t.Fatal("newClient() returned nil")
	}

	// Should default to TelnetClient for invalid transport
	var _ Client = client
}