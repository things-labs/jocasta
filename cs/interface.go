package cs

import (
	"context"
	"io"
	"net"
)

// Dialer A Dialer is a means to establish a connection.
type Dialer interface {
	Dial(network, address string) (net.Conn, error)
}

// ContextDialer A ContextDialer dials using a context.
type ContextDialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
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
