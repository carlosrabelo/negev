package ios

import (
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
