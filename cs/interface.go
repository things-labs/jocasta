package cs

import (
	"net"
)

// Handler handler conn interface
type Handler interface {
	ServerConn(c net.Conn)
}

// HandlerFunc function implement Handler interface
type HandlerFunc func(c net.Conn)

// ServerConn implement Handler interface
func (f HandlerFunc) ServerConn(c net.Conn) { f(c) }
