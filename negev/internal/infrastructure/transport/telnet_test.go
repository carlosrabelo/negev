package transport

import (
	"net"
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

func TestTelnetDialAddr(t *testing.T) {
	if got := telnetDialAddr("192.168.1.10"); got != "192.168.1.10:23" {
		t.Fatalf("host only = %q", got)
	}
	if got := telnetDialAddr("127.0.0.1:2323"); got != "127.0.0.1:2323" {
		t.Fatalf("host:port = %q", got)
	}
	if got := telnetDialAddr("2001:db8::1"); got != "[2001:db8::1]:23" {
		t.Fatalf("ipv6 = %q", got)
	}
}

func TestTelnetSetAuthSequence(t *testing.T) {
	tc := NewTelnetClient(entities.SwitchConfig{Target: "127.0.0.1"})
	tc.SetAuthSequence([]entities.AuthPrompt{{WaitFor: "Username:", SendCmd: "admin\n"}})
	if len(tc.authSequence) != 1 || tc.authSequence[0].WaitFor != "Username:" {
		t.Fatal("SetAuthSequence did not store prompts")
	}
}

func TestDefaultIOSAuthSequence(t *testing.T) {
	seq := defaultIOSAuthSequence(entities.SwitchConfig{
		Username: "admin", Password: "p", EnablePassword: "e",
	})
	if len(seq) != 6 || seq[0].WaitFor != PromptUsername || seq[2].SendCmd != "enable\n" {
		t.Fatalf("unexpected IOS sequence: %+v", seq)
	}
}

func TestTelnetAuthFailureClearsConnection(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		_, _ = conn.Write([]byte("Welcome\r\n"))
		conn.Close()
	}()
	tc := NewTelnetClient(entities.SwitchConfig{
		Target: ln.Addr().String(), Username: "admin", Password: "pass", EnablePassword: "enable",
	})
	err = tc.Connect()
	<-done
	if err == nil {
		t.Fatal("expected auth failure against a banner-only server")
	}
	if tc.IsConnected() {
		t.Fatal("auth failure must clear the Telnet connection")
	}
}

func TestTelnetExecuteCommandRequiresConnection(t *testing.T) {
	tc := NewTelnetClient(entities.SwitchConfig{Target: "127.0.0.1"})
	if _, err := tc.ExecuteCommand("show version"); err == nil {
		t.Fatal("expected error when executing without a connection")
	}
}

func TestTelnetClientBasicsAndConnectFailure(t *testing.T) {
	cfg := entities.SwitchConfig{
		Target: "127.0.0.1", Username: "admin", Password: "pass",
		EnablePassword: "enable", VerbosityLevel: 3,
	}
	tc := NewTelnetClient(cfg)
	tc.SetAuthSequence([]entities.AuthPrompt{{WaitFor: "Username:", SendCmd: "admin\n"}})
	if tc.IsConnected() {
		t.Fatal("expected not connected")
	}
	tc.Disconnect()
	err := tc.Connect()
	if err == nil {
		t.Fatal("expected Telnet connect failure against localhost")
	}
	if tc.IsConnected() {
		t.Fatal("failed dial must not leave the client connected")
	}
}
