package transport

import "testing"

func TestClientCacheEmpty(t *testing.T) {
	if cacheSize() != 0 && getCached("missing") != nil {
		t.Fatal("expected empty cache lookup to miss")
	}
}
