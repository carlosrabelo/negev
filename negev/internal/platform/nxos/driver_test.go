package nxos

import "testing"

func TestName(t *testing.T) {
	d := &Driver{}
	if got := d.Name(); got != "nxos" {
		t.Fatalf("Name() = %q; expected %q", got, "nxos")
	}
}
