package platform

import "testing"

func TestEmptyRegistry(t *testing.T) {
	if Get("ios") != nil {
		t.Fatal("expected no drivers registered yet")
	}
	if len(Available()) != 0 {
		t.Fatal("expected empty Available()")
	}
}
