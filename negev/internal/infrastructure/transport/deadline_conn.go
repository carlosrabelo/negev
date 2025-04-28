package transport

import (
	"io"
	"net"
	"os"
	"sync"
	"time"
)

type readChunk struct {
	data []byte
	err  error
}

// deadlineConn wraps a net.Conn (such as ssh.Client.Dial's chanConn) to provide
// functioning SetReadDeadline/SetWriteDeadline/SetDeadline semantics.
type deadlineConn struct {
	inner net.Conn

	mu                  sync.Mutex
	readDeadline        time.Time
	writeDeadline       time.Time
	readDeadlineChanged chan struct{}

	readMu  sync.Mutex
	pending []byte
	readErr error
	chunks  chan readChunk

	closeOnce sync.Once
	closed    chan struct{}
}

func newDeadlineConn(inner net.Conn) *deadlineConn {
	d := &deadlineConn{
		inner:               inner,
		readDeadlineChanged: make(chan struct{}, 1),
		chunks:              make(chan readChunk, 4),
		closed:              make(chan struct{}),
	}
	go d.readLoop()
	return d
}

func (d *deadlineConn) readLoop() {
	defer close(d.chunks)
	buf := make([]byte, BufferSize)
	for {
		n, err := d.inner.Read(buf)
		var data []byte
		if n > 0 {
			data = make([]byte, n)
			copy(data, buf[:n])
		}
		if n > 0 || err != nil {
			select {
			case d.chunks <- readChunk{data: data, err: err}:
			case <-d.closed:
				return
			}
		}
		if err != nil {
			return
		}
	}
}

func (d *deadlineConn) Read(p []byte) (int, error) {
	d.readMu.Lock()
	defer d.readMu.Unlock()

	select {
	case <-d.closed:
		return 0, net.ErrClosed
	default:
	}

	d.mu.Lock()
	dl := d.readDeadline
	d.mu.Unlock()
	if !dl.IsZero() && !time.Now().Before(dl) {
		return 0, os.ErrDeadlineExceeded
	}

	if len(d.pending) > 0 {
		n := copy(p, d.pending)
		if n < len(d.pending) {
			d.pending = d.pending[n:]
		} else {
			d.pending = nil
		}
		return n, nil
	}

	if d.readErr != nil {
		return 0, d.readErr
	}

	if len(p) == 0 {
		return 0, nil
	}

	for {
		d.mu.Lock()
		dl = d.readDeadline
		d.mu.Unlock()

		var timer *time.Timer
		var timerC <-chan time.Time
		if !dl.IsZero() {
			wait := time.Until(dl)
			if wait <= 0 {
				return 0, os.ErrDeadlineExceeded
			}
			timer = time.NewTimer(wait)
			timerC = timer.C
		}

		select {
		case <-d.closed:
			if timer != nil {
				timer.Stop()
			}
			return 0, net.ErrClosed
		case chunk, ok := <-d.chunks:
			if timer != nil {
				timer.Stop()
			}
			if !ok {
				return 0, io.EOF
			}
			if len(chunk.data) > 0 {
				n := copy(p, chunk.data)
				if n < len(chunk.data) {
					d.pending = append([]byte(nil), chunk.data[n:]...)
				}
				if chunk.err != nil {
					d.readErr = chunk.err
				}
				return n, nil
			}
			d.readErr = chunk.err
			return 0, chunk.err
		case <-timerC:
			return 0, os.ErrDeadlineExceeded
		case <-d.readDeadlineChanged:
			if timer != nil {
				timer.Stop()
			}
		}
	}
}

func (d *deadlineConn) Write(p []byte) (int, error) {
	select {
	case <-d.closed:
		return 0, net.ErrClosed
	default:
	}

	d.mu.Lock()
	dl := d.writeDeadline
	d.mu.Unlock()

	if dl.IsZero() {
		return d.inner.Write(p)
	}

	wait := time.Until(dl)
	if wait <= 0 {
		return 0, os.ErrDeadlineExceeded
	}

	type writeResult struct {
		n   int
		err error
	}
	ch := make(chan writeResult, 1)
	go func() {
		n, err := d.inner.Write(p)
		ch <- writeResult{n: n, err: err}
	}()

	timer := time.NewTimer(wait)
	defer timer.Stop()

	select {
	case res := <-ch:
		return res.n, res.err
	case <-timer.C:
		_ = d.Close()
		<-ch
		return 0, os.ErrDeadlineExceeded
	case <-d.closed:
		_ = d.Close()
		<-ch
		return 0, net.ErrClosed
	}
}

func (d *deadlineConn) Close() error {
	var err error
	d.closeOnce.Do(func() {
		close(d.closed)
		err = d.inner.Close()
	})
	return err
}

func (d *deadlineConn) LocalAddr() net.Addr {
	return d.inner.LocalAddr()
}

func (d *deadlineConn) RemoteAddr() net.Addr {
	return d.inner.RemoteAddr()
}

func (d *deadlineConn) SetDeadline(t time.Time) error {
	d.mu.Lock()
	d.readDeadline = t
	d.writeDeadline = t
	d.mu.Unlock()
	select {
	case d.readDeadlineChanged <- struct{}{}:
	default:
	}
	return nil
}

func (d *deadlineConn) SetReadDeadline(t time.Time) error {
	d.mu.Lock()
	d.readDeadline = t
	d.mu.Unlock()
	select {
	case d.readDeadlineChanged <- struct{}{}:
	default:
	}
	return nil
}

func (d *deadlineConn) SetWriteDeadline(t time.Time) error {
	d.mu.Lock()
	d.writeDeadline = t
	d.mu.Unlock()
	return nil
}
