// Package mock simulate a net.Conn with io.ReadWriter
package mock

import (
	"io"
	"net"
	"time"
)

type Mock struct {
	rw io.ReadWriter
}

func New(rw io.ReadWriter) net.Conn                { return &Mock{rw} }
func (m *Mock) Read(b []byte) (n int, err error)   { return m.rw.Read(b) }
func (m *Mock) Write(b []byte) (n int, err error)  { return m.rw.Write(b) }
func (m *Mock) Close() error                       { return nil }
func (m *Mock) LocalAddr() net.Addr                { return nil }
func (m *Mock) RemoteAddr() net.Addr               { return nil }
func (m *Mock) SetDeadline(t time.Time) error      { return nil }
func (m *Mock) SetReadDeadline(t time.Time) error  { return nil }
func (m *Mock) SetWriteDeadline(t time.Time) error { return nil }
