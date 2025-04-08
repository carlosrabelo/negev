package transport

import (
	"net"
	"strings"
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

func TestTelnetClientBasicsAndConnectFailure(t *testing.T) {
	cfg := entities.SwitchConfig{
		Target:         "127.0.0.1",
		Username:       "admin",
		Password:       "pass",
		EnablePassword: "enable",
		VerbosityLevel: 3,
	}
	tc := NewTelnetClient(cfg)
	tc.SetAuthSequence([]entities.AuthPrompt{
		{WaitFor: "Username:", SendCmd: "USERNAME_PLACEHOLDER\n"},
	})
	if tc.IsConnected() {
		t.Fatal("expected not connected")
	}
	tc.Disconnect()

	err := tc.Connect()
	if err == nil {
		t.Fatal("expected Telnet connect failure against localhost")
	}
	if !strings.Contains(err.Error(), "failed to connect") {
		t.Fatalf("unexpected error: %v", err)
	}
	if tc.IsConnected() {
		t.Fatal("failed dial must not leave the client connected")
	}
}

func TestTelnetExecuteCommandRequiresConnection(t *testing.T) {
	tc := NewTelnetClient(entities.SwitchConfig{Target: "127.0.0.1"})
	if _, err := tc.ExecuteCommand("show version"); err == nil {
		t.Fatal("expected error when executing without a connection")
	}
}

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

func TestTelnetAuthSkipsEnablePasswordWhenPrivilegedAfterEnable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	got := make(chan []string, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 256)
		var reads []string
		_, _ = conn.Write([]byte("User:"))
		n, _ := conn.Read(buf)
		reads = append(reads, string(buf[:n]))
		_, _ = conn.Write([]byte("Password:"))
		n, _ = conn.Read(buf)
		reads = append(reads, string(buf[:n]))
		_, _ = conn.Write([]byte("\r\n(DM1200E) >"))
		n, _ = conn.Read(buf)
		reads = append(reads, string(buf[:n]))
		_, _ = conn.Write([]byte("\r\n(DM1200E) #"))
		n, _ = conn.Read(buf)
		reads = append(reads, string(buf[:n]))
		_, _ = conn.Write([]byte("\r\n(DM1200E) #"))
		got <- reads
	}()

	tc := NewTelnetClient(entities.SwitchConfig{
		Target:         ln.Addr().String(),
		Username:       "admin",
		Password:       "pass",
		EnablePassword: "enable-secret",
	})
	tc.SetAuthSequence([]entities.AuthPrompt{
		{WaitFor: "User:", SendCmd: "USERNAME_PLACEHOLDER\n"},
		{WaitFor: "Password:", SendCmd: "PASSWORD_PLACEHOLDER\n"},
		{WaitFor: ">", SendCmd: "enable\n"},
		{WaitFor: "Password:", SendCmd: "ENABLE_PASSWORD_PLACEHOLDER\n"},
		{WaitFor: "#", SendCmd: "terminal length 0\n"},
		{WaitFor: "#", SendCmd: ""},
	})
	if err := tc.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	tc.Disconnect()
	<-done

	select {
	case reads := <-got:
		if len(reads) < 4 {
			t.Fatalf("expected at least 4 client writes, got %#v", reads)
		}
		if !strings.Contains(reads[2], "enable") {
			t.Fatalf("expected enable, got %q", reads[2])
		}
		if strings.Contains(reads[3], "enable-secret") {
			t.Fatalf("must not send enable password when already privileged; got %#v", reads)
		}
		if !strings.Contains(reads[3], "terminal length 0") {
			t.Fatalf("expected terminal length 0 after enable; got %#v", reads)
		}
	default:
		t.Fatal("server did not observe client commands")
	}
}

func TestTelnetAuthSkipsEnableWhenAlreadyPrivileged(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	got := make(chan string, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 256)
		_, _ = conn.Write([]byte("User:"))
		n, _ := conn.Read(buf)
		_, _ = conn.Write([]byte("Password:"))
		n, _ = conn.Read(buf)
		_, _ = conn.Write([]byte("\r\nDM1200E#"))
		n, _ = conn.Read(buf)
		got <- string(buf[:n])
		_, _ = conn.Write([]byte("\r\nDM1200E#"))
		_, _ = conn.Read(buf) // final empty wait may block until client closes
	}()

	tc := NewTelnetClient(entities.SwitchConfig{
		Target:         ln.Addr().String(),
		Username:       "admin",
		Password:       "pass",
		EnablePassword: "enable-secret",
	})
	tc.SetAuthSequence([]entities.AuthPrompt{
		{WaitFor: "User:", SendCmd: "USERNAME_PLACEHOLDER\n"},
		{WaitFor: "Password:", SendCmd: "PASSWORD_PLACEHOLDER\n"},
		{WaitFor: ">", SendCmd: "enable\n"},
		{WaitFor: "Password:", SendCmd: "ENABLE_PASSWORD_PLACEHOLDER\n"},
		{WaitFor: "#", SendCmd: "terminal length 0\n"},
		{WaitFor: "#", SendCmd: ""},
	})
	if err := tc.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	tc.Disconnect()
	<-done

	select {
	case cmd := <-got:
		if strings.Contains(cmd, "enable") {
			t.Fatalf("must not send enable when already privileged; got %q", cmd)
		}
		if !strings.Contains(cmd, "terminal length 0") {
			t.Fatalf("expected terminal length 0; got %q", cmd)
		}
	default:
		t.Fatal("server did not observe a post-login command")
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
		Target:         ln.Addr().String(),
		Username:       "admin",
		Password:       "pass",
		EnablePassword: "enable",
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
