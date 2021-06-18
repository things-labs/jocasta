// Package shadowsocks implement package shadowsocks protocol
package shadowsocks

import (
	"io"
	"net"

	"github.com/thinkgos/jocasta/pkg/bpool"
)

const bufferSize = 4108 // data.len(2) + hmacsha1(10) + data(4096)

var bufferPool = bpool.NewPool(bufferSize)

// Conn is a generic stream-oriented connection.
type Conn struct {
	net.Conn
	*Cipher
}

// New new with a connection and cipher
func New(c net.Conn, cipher *Cipher) *Conn {
	return &Conn{
		Conn:   c,
		Cipher: cipher,
	}
}

// Close implement closer interface.
func (sf *Conn) Close() error {
	return sf.Conn.Close()
}

// NewConnWithRawAddr This is intended for use by users implementing a local socks proxy.
// rawaddr shoud contain part of the data in socks request, starting from the
// ATYP field. (Refer to rfc1928 for more information.)
func NewConnWithRawAddr(rawConn net.Conn, rawaddr []byte, cipher *Cipher) (c *Conn, err error) {
	c = New(rawConn, cipher)
	if _, err = c.Write(rawaddr); err != nil {
		c.Close()
		return nil, err
	}
	return
}

// DialWithRawAddr This is intended for use by users implementing a local socks proxy.
// rawaddr shoud contain part of the data in socks request, starting from the
// ATYP field. (Refer to rfc1928 for more information.)
func DialWithRawAddr(rawAddr []byte, server string, cipher *Cipher) (*Conn, error) {
	conn, err := net.Dial("tcp", server)
	if err != nil {
		return nil, err
	}
	return NewConnWithRawAddr(conn, rawAddr, cipher)
}

// Dial This is intended for use by users implementing a local socks proxy.
// rawaddr shoud contain part of the data in socks request, starting from the
// ATYP field. (Refer to rfc1928 for more information.)
// addr should be in the form of host:port
func Dial(addr, server string, cipher *Cipher) (c *Conn, err error) {
	ra, err := ParseAddrSpec(addr)
	if err != nil {
		return
	}
	return DialWithRawAddr(ra, server, cipher)
}

// Iv return cipher iv
func (sf *Conn) Iv() (iv []byte) {
	iv = make([]byte, len(sf.iv))
	copy(iv, sf.iv)
	return
}

// Key return cipher key
func (sf *Conn) Key() (key []byte) {
	key = make([]byte, len(sf.key))
	copy(key, sf.key)
	return
}

// Read reads data from the connection.
func (sf *Conn) Read(b []byte) (n int, err error) {
	if sf.reader == nil {
		iv := make([]byte, sf.ivLen)
		if _, err = io.ReadFull(sf.Conn, iv); err != nil {
			return
		}
		// init decrypt
		if err = sf.initDecrypt(iv); err != nil {
			return
		}
	}

	bp := bufferPool.Get()
	defer bufferPool.Put(bp)

	cipherData := bp
	if cap(cipherData) < len(b) {
		cipherData = make([]byte, len(b))
	} else {
		cipherData = cipherData[:len(b)]
	}

	n, err = sf.Conn.Read(cipherData)
	if n > 0 {
		sf.decrypt(b[:n], cipherData[:n])
	}
	return
}

// Write writes data to the connection.
func (sf *Conn) Write(b []byte) (n int, err error) {
	var iv []byte

	if sf.writer == nil {
		if iv, err = sf.initEncrypt(); err != nil {
			return 0, err
		}
	}

	bp := bufferPool.Get()
	defer bufferPool.Put(bp)

	cipherData := bp
	dataSize := len(b) + len(iv)
	if cap(cipherData) < dataSize {
		cipherData = make([]byte, dataSize)
	} else {
		cipherData = cipherData[:dataSize]
	}

	if iv != nil {
		// Put initialization vector in buffer, do a single write to send both
		// iv and data.
		copy(cipherData, iv)
	}

	sf.encrypt(cipherData[len(iv):], b)
	n, err = sf.Conn.Write(cipherData)
	return n - len(iv), err
}
