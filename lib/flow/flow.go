// package flow 实现字节统计,读,写,读写统计,以字节为准. 三个参数为空时,无任何统计
package flow

import (
	"io"

	"go.uber.org/atomic"
)

type Flow struct {
	io.ReadWriter
	Wc *atomic.Uint64 // 写统计
	Rc *atomic.Uint64 // 读统计
	Tc *atomic.Uint64 // 读写统计
}

// Read ...
func (sf *Flow) Read(p []byte) (int, error) {
	n, err := sf.ReadWriter.Read(p)
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
func (sf *Flow) Write(p []byte) (int, error) {
	n, err := sf.ReadWriter.Write(p)
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
