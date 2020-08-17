// Package cgzip 采用gzip压缩实现的net.conn接口
package cgzip

import (
	"compress/gzip"
	"net"
)

// Conn is a generic stream-oriented network connection with gzip
type Conn struct {
	net.Conn
	w *gzip.Writer
}

// New new a gzip compress
func New(conn net.Conn) net.Conn {
	return &Conn{
		conn,
		gzip.NewWriter(conn),
	}
}

// Read reads data from the connection.
func (sf *Conn) Read(p []byte) (int, error) {
	r, err := gzip.NewReader(sf.Conn)
	if err != nil {
		return 0, err
	}
	return r.Read(p)
}

// Write writes data to the connection.
func (sf *Conn) Write(p []byte) (int, error) {
	n, _ := sf.w.Write(p)
	err := sf.w.Flush()
	return n, err
}
