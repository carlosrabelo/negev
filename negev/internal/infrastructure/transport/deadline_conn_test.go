package transport

import (
	"bytes"
	"errors"
	"io"
	"net"
	"os"
	"testing"
	"time"
)

type noDeadlineConn struct {
	net.Conn
}

func (n *noDeadlineConn) SetDeadline(time.Time) error {
	return errors.New("ssh: tcpChan: deadline not supported")
}

func (n *noDeadlineConn) SetReadDeadline(time.Time) error {
	return errors.New("ssh: tcpChan: deadline not supported")
}

func (n *noDeadlineConn) SetWriteDeadline(time.Time) error {
	return errors.New("ssh: tcpChan: deadline not supported")
}

func TestDeadlineConnPartialReadsAndWrites(t *testing.T) {
	c1, c2 := net.Pipe()
	dc := newDeadlineConn(&noDeadlineConn{Conn: c1})
	defer dc.Close()
	defer c2.Close()

	payload := []byte("hello-jump-tunnel")
	go func() {
		_, _ = c2.Write(payload)
	}()

	buf := make([]byte, 5)
	var got bytes.Buffer
	for got.Len() < len(payload) {
		n, err := dc.Read(buf)
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}
		got.Write(buf[:n])
	}
	if got.String() != string(payload) {
		t.Fatalf("got %q; want %q", got.String(), string(payload))
	}

	readBack := make(chan string, 1)
	go func() {
		b := make([]byte, 32)
		n, _ := c2.Read(b)
		readBack <- string(b[:n])
	}()
	if _, err := dc.Write([]byte("ping")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if msg := <-readBack; msg != "ping" {
		t.Fatalf("peer read %q; want ping", msg)
	}
}

func TestDeadlineConnReadTimeoutPreservesLaterData(t *testing.T) {
	c1, c2 := net.Pipe()
	dc := newDeadlineConn(&noDeadlineConn{Conn: c1})
	defer dc.Close()
	defer c2.Close()

	if err := dc.SetReadDeadline(time.Now().Add(30 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}

	buf := make([]byte, 32)
	_, err := dc.Read(buf)
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("Read error = %v; want os.ErrDeadlineExceeded", err)
	}

	if err := dc.SetReadDeadline(time.Time{}); err != nil {
		t.Fatalf("clear SetReadDeadline: %v", err)
	}

	go func() {
		_, _ = c2.Write([]byte("after-timeout"))
	}()

	n, err := dc.Read(buf)
	if err != nil {
		t.Fatalf("Read after clearing deadline failed: %v", err)
	}
	if string(buf[:n]) != "after-timeout" {
		t.Fatalf("got %q; want after-timeout", string(buf[:n]))
	}
}

func TestDeadlineConnCloseUnblocksPendingRead(t *testing.T) {
	c1, c2 := net.Pipe()
	dc := newDeadlineConn(&noDeadlineConn{Conn: c1})
	defer c2.Close()

	errCh := make(chan error, 1)
	go func() {
		buf := make([]byte, 16)
		_, err := dc.Read(buf)
		errCh <- err
	}()

	time.Sleep(20 * time.Millisecond)
	if err := dc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	select {
	case err := <-errCh:
		if !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.EOF) {
			t.Fatalf("Read after Close error = %v; want net.ErrClosed or EOF", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Read remained blocked after Close")
	}
}
