package transport

import (
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

func TestGetClientCachesByCredentials(t *testing.T) {
	t.Cleanup(CloseAll)
	cfg := entities.SwitchConfig{
		Target:    "203.0.113.20",
		Transport: "telnet",
		Username:  "admin",
		Password:  "secret",
	}
	c1 := GetClient(cfg)
	c2 := GetClient(cfg)
	if c1 != c2 {
		t.Fatal("expected cached client reuse")
	}
	if _, ok := c1.(*TelnetClient); !ok {
		t.Fatalf("expected *TelnetClient, got %T", c1)
	}

	sshCfg := cfg
	sshCfg.Transport = "ssh"
	sshCfg.Target = "203.0.113.21"
	c3 := GetClient(sshCfg)
	if _, ok := c3.(*SSHClient); !ok {
		t.Fatalf("expected *SSHClient, got %T", c3)
	}
	if c3 == c1 {
		t.Fatal("ssh client must be distinct from telnet client")
	}

	CloseAll()
	c4 := GetClient(cfg)
	if c4 == c1 {
		t.Fatal("expected new client after CloseAll")
	}
}

func TestNewClientTransportSelection(t *testing.T) {
	ssh := newClient(entities.SwitchConfig{Transport: "ssh", Target: "203.0.113.30"})
	if _, ok := ssh.(*SSHClient); !ok {
		t.Fatalf("expected SSH client, got %T", ssh)
	}
	tel := newClient(entities.SwitchConfig{Transport: "telnet", Target: "203.0.113.31"})
	if _, ok := tel.(*TelnetClient); !ok {
		t.Fatalf("expected Telnet client, got %T", tel)
	}
	def := newClient(entities.SwitchConfig{Target: "203.0.113.32"})
	if _, ok := def.(*TelnetClient); !ok {
		t.Fatalf("expected default Telnet client, got %T", def)
	}
}

func TestGetClientDifferentiatesJumpHostAndCloseClient(t *testing.T) {
	t.Cleanup(CloseAll)
	direct := entities.SwitchConfig{
		Target:    "203.0.113.40",
		Transport: "telnet",
		Username:  "admin",
		Password:  "secret",
	}
	viaJump := direct
	viaJump.JumpHost = &entities.JumpHostConfig{
		Host:     "192.0.2.5",
		Port:     22,
		Username: "ops",
	}
	viaJumpOther := direct
	viaJumpOther.JumpHost = &entities.JumpHostConfig{
		Host:     "192.0.2.6",
		Port:     22,
		Username: "ops",
	}

	cDirect := GetClient(direct)
	cJump := GetClient(viaJump)
	cJumpOther := GetClient(viaJumpOther)

	if cDirect == cJump {
		t.Fatal("direct client and jump-host client must have distinct cache entries")
	}
	if cJump == cJumpOther {
		t.Fatal("clients using different jump hosts must have distinct cache entries")
	}

	CloseClient(viaJump)
	if GetClient(viaJump) == cJump {
		t.Fatal("CloseClient should evict only the targeted switch client")
	}
	if GetClient(direct) != cDirect {
		t.Fatal("CloseClient must not evict unrelated cached clients")
	}
}
