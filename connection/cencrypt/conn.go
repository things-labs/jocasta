package cencrypt

import (
	"crypto/cipher"
	"io"
	"net"

	"github.com/thinkgos/ppcore/lib/encrypt"
)

type Conn struct {
	net.Conn
	w io.Writer
	r io.Reader
}

// New a connection with encrypt cipher
func New(c net.Conn, cip *encrypt.Cipher) *Conn {
	return &Conn{
		c,
		&cipher.StreamWriter{S: cip.Write, W: c},
		&cipher.StreamReader{S: cip.Read, R: c},
	}
}

func (sf *Conn) Read(b []byte) (n int, err error) {
	return sf.r.Read(b)
}

func (sf *Conn) Write(b []byte) (n int, err error) {
	return sf.w.Write(b)
}
