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

// NopHandler nop handler
type NopHandler struct{}

// ServerConn NopHandler implement Handler interface
func (NopHandler) ServerConn(c net.Conn) { c.Close() } // nolint: errcheck
