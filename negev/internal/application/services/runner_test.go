package services

import "testing"

func TestRunTargetRequiresConfig(t *testing.T) {
	err := RunTarget("192.168.1.10", "/tmp/negev-missing-config.yaml", false, 0, false)
	if err == nil {
		t.Fatal("expected load error")
	}
}
