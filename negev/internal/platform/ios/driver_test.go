package ios

import "testing"

func TestDriverName(t *testing.T) {
	if (&Driver{}).Name() != "ios" {
		t.Fatal("driver name")
	}
}
