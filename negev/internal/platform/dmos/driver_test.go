package dmos

import "testing"

func TestDriverName(t *testing.T) {
	if (&Driver{}).Name() != "dmos" {
		t.Fatal("name")
	}
}
