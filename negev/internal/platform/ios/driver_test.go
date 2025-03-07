package ios

import (
	"reflect"
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

func TestDriverName(t *testing.T) {
	if (&Driver{}).Name() != "ios" {
		t.Fatal("driver name")
	}
}

func TestIOSAuthSequence(t *testing.T) {
	seq := (&Driver{}).GetAuthenticationSequence()
	if len(seq) != 6 || seq[0].WaitFor != "Username:" || seq[2].SendCmd != "enable\n" {
		t.Fatalf("auth sequence: %+v", seq)
	}
	_ = entities.AuthPrompt{}
}

func TestParseVLANs(t *testing.T) {
	output := `
VLAN Name                             Status    Ports
---- -------------------------------- --------- -------------------------------
1    default                          active    Gi1/0/1, Gi1/0/2
10   VLAN_10                          active    Gi1/0/3
20   VLAN_20                          active
`
	got := parseVLANs(output)
	expected := []string{"1", "10", "20"}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("parseVLANs() = %v; expected %v", got, expected)
	}
}
