package dm1200

import "testing"

func TestName(t *testing.T) {
	if got := (&Driver{}).Name(); got != "dm1200" {
		t.Fatalf("Name() = %q; expected %q", got, "dm1200")
	}
}
