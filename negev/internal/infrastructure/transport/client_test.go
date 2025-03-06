package transport

import (
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

func TestClientCacheEmpty(t *testing.T) {
	if cacheSize() != 0 && getCached("missing") != nil {
		t.Fatal("expected empty cache lookup to miss")
	}
}

func TestCacheKeyStable(t *testing.T) {
	cfg := entities.SwitchConfig{Transport: "telnet", Target: "1.1.1.1", Username: "a", Password: "b", EnablePassword: "c"}
	if cacheKey(cfg) == "" || cacheKey(cfg) != cacheKey(cfg) {
		t.Fatal("cacheKey should be stable and non-empty")
	}
	other := cfg
	other.Password = "other"
	if cacheKey(cfg) == cacheKey(other) {
		t.Fatal("different credentials must not share a cache key")
	}
}
