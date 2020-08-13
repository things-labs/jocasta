package cs

import (
	"io"
	"net"
	"time"
)

// Dialer client dialer interface
type Dialer interface {
	DialTimeout(address string, timeout time.Duration) (net.Conn, error)
}

// Server server interface
type Server interface {
	io.Closer
	LocalAddr() string
	ListenAndServe() error
}

// Handler handler conn interface
type Handler interface {
	ServerConn(c net.Conn)
}

// HandlerFunc function implement Handler interface
type HandlerFunc func(c net.Conn)

// ServerConn implement Handler interface
func (f HandlerFunc) ServerConn(c net.Conn) { f(c) }

func setStatus(Status chan<- error, err error) {
	if Status != nil {
		Status <- err
	}
}
