package comware

import "testing"

func TestName(t *testing.T) {
	if got := (&Driver{}).Name(); got != "comware" {
		t.Fatalf("Name() = %q; expected %q", got, "comware")
	}
}
