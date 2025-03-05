package transport

import "testing"

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
