package dmswitch

import "testing"

func TestName(t *testing.T) {
	if got := (&Driver{}).Name(); got != "dmswitch" {
		t.Fatalf("Name() = %q; expected %q", got, "dmswitch")
	}
}
