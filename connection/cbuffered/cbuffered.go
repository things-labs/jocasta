package cbuffered

import (
	"bufio"
	"net"
)

type Conn struct {
	net.Conn
	r *bufio.Reader
}

func New(c net.Conn, sizes ...int) *Conn {
	if len(sizes) > 0 {
		return &Conn{c, bufio.NewReaderSize(c, sizes[0])}
	}
	return &Conn{c, bufio.NewReader(c)}
}

func (sf *Conn) Peek(n int) ([]byte, error) {
	return sf.r.Peek(n)
}

func (sf *Conn) Read(p []byte) (int, error) {
	return sf.r.Read(p)
}

func (sf *Conn) ReadByte() (byte, error) {
	return sf.r.ReadByte()
}

func (sf *Conn) UnreadByte() error {
	return sf.r.UnreadByte()
}

func (sf *Conn) Buffered() int {
	return sf.r.Buffered()
}
