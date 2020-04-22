package cflow

import (
	"net"

	"go.uber.org/atomic"
)

type Conn struct {
	net.Conn
	Wc *atomic.Uint64 // 写统计
	Rc *atomic.Uint64 // 读统计
	Tc *atomic.Uint64 // 读写统计
}

// Read ...
func (sf *Conn) Read(p []byte) (int, error) {
	n, err := sf.Conn.Read(p)
	if n != 0 {
		cnt := uint64(n)
		if sf.Rc != nil {
			sf.Rc.Add(cnt)
		}
		if sf.Tc != nil {
			sf.Tc.Add(cnt)
		}
	}
	return n, err
}

// Write ...
func (sf *Conn) Write(p []byte) (int, error) {
	n, err := sf.Conn.Write(p)
	if n != 0 {
		cnt := uint64(n)
		if sf.Wc != nil {
			sf.Wc.Add(cnt)
		}
		if sf.Tc != nil {
			sf.Tc.Add(cnt)
		}
	}
	return n, err
}
