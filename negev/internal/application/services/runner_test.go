package services

import "testing"

func TestRunTargetMissingDriver(t *testing.T) {
	// ios is registered only when the ios package is imported by the test binary.
	err := RunTarget("192.168.1.10", "", false, 0, false)
	if err == nil {
		t.Fatal("expected error without a live switch / registered driver in this package")
	}
}
