package cs

import (
	"io"
	"net"
	"time"
)

type Dialer interface {
	DialTimeout(address string, timeout time.Duration) (net.Conn, error)
}

type Server interface {
	io.Closer
	LocalAddr() string
	ListenAndServe() error
}

type Handler interface {
	ServerConn(c net.Conn)
}

type HandlerFunc func(c net.Conn)

func (f HandlerFunc) ServerConn(c net.Conn) {
	f(c)
}

func setStatus(Status chan<- error, err error) {
	if Status != nil {
		Status <- err
	}
}
