// Package binding provide stream binding or package bind
package binding

import (
	"io"
	"net"
	"time"

	"github.com/thinkgos/jocasta/lib/bpool"
	"github.com/thinkgos/jocasta/lib/extnet"
	"github.com/thinkgos/jocasta/lib/gpool"
)

// Forward forward stream
type Forward struct {
	bpool.BufferPool
	gPool gpool.Pool
}

// New binding forward with buffer size 缓冲切片大小
func New(size int, opts ...Option) *Forward {
	c := &Forward{
		BufferPool: bpool.NewPool(size),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Proxy proxy rw1 and rw2 with binding
func (sf *Forward) Proxy(rw1, rw2 io.ReadWriter) (err error) {
	ech1 := make(chan error, 1)
	ech2 := make(chan error, 1)
	sf.Go(func() { ech1 <- sf.Copy(rw1, rw2) })
	sf.Go(func() { ech2 <- sf.Copy(rw2, rw1) })
	select {
	case err = <-ech1:
	case err = <-ech2:
	}
	return
}

// Copy stream src to dst
func (sf *Forward) Copy(dst io.Writer, src io.Reader) error {
	buf := sf.Get()
	defer sf.Put(buf)
	_, err := io.CopyBuffer(dst, src, buf[:cap(buf)])
	return err
}

// RunUDPCopy ...
func (sf *Forward) RunUDPCopy(dst, src *net.UDPConn, dstAddr net.Addr, readTimeout time.Duration, beforeWriteFn func(data []byte) []byte) {
	buf := sf.Get()
	defer sf.Put(buf)
	for {
		if readTimeout > 0 {
			src.SetReadDeadline(time.Now().Add(readTimeout)) // nolint: errcheck
		}
		n, err := src.Read(buf[:cap(buf)])
		if readTimeout > 0 {
			src.SetReadDeadline(time.Time{}) // nolint: errcheck
		}
		if err != nil {
			if extnet.IsErrClosed(err) || extnet.IsErrTimeout(err) || extnet.IsErrRefused(err) {
				return
			}
			continue
		}
		_, err = dst.WriteTo(beforeWriteFn(buf[:n]), dstAddr)
		if err != nil {
			if extnet.IsErrClosed(err) {
				return
			}
			continue
		}
	}
}

// Go 提交任务,如果协程池提交失败,使用goroutine
func (sf *Forward) Go(f func()) {
	if sf.gPool == nil || sf.gPool.Submit(f) != nil {
		go f()
	}
}
