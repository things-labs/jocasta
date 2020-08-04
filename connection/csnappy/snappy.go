// Package csnappy 采用snappy压缩实现的net.conn接口
package csnappy

import (
	"net"

	"github.com/golang/snappy"
)

type Conn struct {
	net.Conn
	w *snappy.Writer
	r *snappy.Reader
}

func New(conn net.Conn) *Conn {
	return &Conn{
		conn,
		snappy.NewBufferedWriter(conn),
		snappy.NewReader(conn),
	}
}

func (sf *Conn) Read(p []byte) (int, error) {
	return sf.r.Read(p)
}

func (sf *Conn) Write(p []byte) (int, error) {
	n, _ := sf.w.Write(p)
	err := sf.w.Flush()
	return n, err
}
