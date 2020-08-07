package shadowsocks

import (
	"fmt"
	"io"
	"net"
	"strconv"

	"github.com/thinkgos/jocasta/lib/bpool"
)

const bufferSize = 4108 // data.len(2) + hmacsha1(10) + data(4096)

var bufferPool = bpool.NewPool(bufferSize)

type Conn struct {
	net.Conn
	*Cipher
	readBuf  []byte
	writeBuf []byte
}

func New(c net.Conn, cipher *Cipher) *Conn {
	return &Conn{
		Conn:     c,
		Cipher:   cipher,
		readBuf:  bufferPool.Get(),
		writeBuf: bufferPool.Get(),
	}
}

// Close implement closer interface
func (c *Conn) Close() error {
	bufferPool.Put(c.readBuf)
	bufferPool.Put(c.writeBuf)
	return c.Conn.Close()
}

// ToRawAddr convert addr to protocol raw address []byte
func ToRawAddr(addr string) (buf []byte, err error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("shadowsocks: address error %s %v", addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("shadowsocks: invalid port %s", addr)
	}

	length := 1 + 1 + len(host) + 2 // addrType(1) + hostLen(1) + host + port(2)
	buf = make([]byte, 0, length)
	// 3 means the address is domain name
	// host address length  followed by host address
	buf = append(buf, 3, byte(len(host)))
	buf = append(buf, []byte(host)...)           // host
	buf = append(buf, byte(port>>8), byte(port)) // port
	return
}

// This is intended for use by users implementing a local socks proxy.
// rawaddr shoud contain part of the data in socks request, starting from the
// ATYP field. (Refer to rfc1928 for more information.)
func DialWithRawAddr(rawaddr []byte, server string, cipher *Cipher) (*Conn, error) {
	conn, err := net.Dial("tcp", server)
	if err != nil {
		return nil, err
	}

	c := New(conn, cipher)
	if _, err = c.write(rawaddr); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

func NewConnWithRawAddr(rawConn net.Conn, rawaddr []byte, cipher *Cipher) (c *Conn, err error) {
	c = New(rawConn, cipher)
	if _, err = c.write(rawaddr); err != nil {
		c.Close()
		return nil, err
	}
	return
}

// addr should be in the form of host:port
func Dial(addr, server string, cipher *Cipher) (c *Conn, err error) {
	ra, err := ToRawAddr(addr)
	if err != nil {
		return
	}
	return DialWithRawAddr(ra, server, cipher)
}

func (c *Conn) Iv() (iv []byte) {
	iv = make([]byte, len(c.iv))
	copy(iv, c.iv)
	return
}

func (c *Conn) Key() (key []byte) {
	key = make([]byte, len(c.key))
	copy(key, c.key)
	return
}

func (c *Conn) Ota() bool {
	return c.ota
}

func (c *Conn) Read(b []byte) (n int, err error) {
	if c.reader == nil {
		iv := make([]byte, c.info.IvLen)
		if _, err = io.ReadFull(c.Conn, iv); err != nil {
			return
		}
		// init decrypt
		if err = c.initDecrypt(iv); err != nil {
			return
		}
	}

	cipherData := c.readBuf
	if cap(cipherData) < len(b) {
		cipherData = make([]byte, len(b))
	} else {
		cipherData = cipherData[:len(b)]
	}

	n, err = c.Conn.Read(cipherData)
	if n > 0 {
		c.decrypt(b[:n], cipherData[:n])
	}
	return
}

func (c *Conn) Write(b []byte) (n int, err error) {
	nn := len(b)

	headerLen := len(b) - nn

	n, err = c.write(b)
	// Make sure <= 0 <= len(b), where b is the slice passed in.
	if n >= headerLen {
		n -= headerLen
	}
	return
}

func (c *Conn) write(b []byte) (int, error) {
	var iv []byte
	var err error

	if c.writer == nil {
		if iv, err = c.initEncrypt(); err != nil {
			return 0, err
		}
	}

	cipherData := c.writeBuf
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

	c.encrypt(cipherData[len(iv):], b)
	return c.Conn.Write(cipherData)
}
