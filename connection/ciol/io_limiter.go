// 实现 net.conn 网络io限速器接口
package ciol

import (
	"context"
	"net"
	"time"

	"golang.org/x/time/rate"
)

// 最大容量
const maxBurst = 1000 * 1000 * 1000

type Conn struct {
	net.Conn
	rLimiter *rate.Limiter
	wLimiter *rate.Limiter
	ctx      context.Context
}

// New new a rate limit (bytes/sec) with option to the Conn read and write.
// if not set,it will not any limit
func New(c net.Conn, opts ...Options) *Conn {
	s := &Conn{
		Conn: c,
		ctx:  context.Background(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Read ...
func (sf *Conn) Read(p []byte) (int, error) {
	n, err := sf.Conn.Read(p)
	if err != nil || sf.rLimiter == nil {
		return n, err
	}

	return n, sf.rLimiter.WaitN(sf.ctx, n)
}

// Write ...
func (sf *Conn) Write(p []byte) (int, error) {
	n, err := sf.Conn.Write(p)
	if err != nil || sf.wLimiter == nil {
		return n, err
	}

	return n, sf.wLimiter.WaitN(sf.ctx, n)
}

func (sf *Conn) Close() (err error) {
	if sf.Conn != nil {
		err = sf.Conn.Close()
		sf.Conn = nil
		sf.rLimiter = nil
		sf.wLimiter = nil
		sf.ctx = nil
	}
	return
}

func (sf *Conn) ReadLimit() rate.Limit {
	return sf.rLimiter.Limit()
}

func (sf *Conn) SetReadLimit(newLimit rate.Limit) {
	sf.rLimiter.SetLimit(newLimit)
}

func (sf *Conn) SetReadLimitAt(now time.Time, newLimit rate.Limit) {
	sf.rLimiter.SetLimitAt(now, newLimit)
}

func (sf *Conn) SetReadBurst(newBurst int) {
	sf.rLimiter.SetBurst(newBurst)
}
func (sf *Conn) SetReadBurstAt(now time.Time, newBurst int) {
	sf.rLimiter.SetBurstAt(now, newBurst)
}

func (sf *Conn) WriteLimit() rate.Limit {
	return sf.wLimiter.Limit()
}

func (sf *Conn) SetWriteLimit(newLimit rate.Limit) {
	sf.wLimiter.SetLimit(newLimit)
}

func (sf *Conn) SetWriteLimitAt(now time.Time, newLimit rate.Limit) {
	sf.wLimiter.SetLimitAt(now, newLimit)
}

func (sf *Conn) SetWriteBurst(newBurst int) {
	sf.wLimiter.SetBurst(newBurst)
}
func (sf *Conn) SetWriteBurstAt(now time.Time, newBurst int) {
	sf.wLimiter.SetBurstAt(now, newBurst)
}

type Options func(*Conn)

// WithReadLimiter 读限速
func WithReadLimiter(bytesPerSec rate.Limit, bursts ...int) Options {
	return func(c *Conn) {
		burst := maxBurst
		if len(bursts) > 0 {
			burst = bursts[0]
		}
		c.rLimiter = rate.NewLimiter(bytesPerSec, burst)
		c.rLimiter.AllowN(time.Now(), burst) // spend initial burst
	}
}

// WithWriteLimiter 写限速
func WithWriteLimiter(bytesPerSec rate.Limit, bursts ...int) Options {
	return func(c *Conn) {
		burst := maxBurst
		if len(bursts) > 0 {
			burst = bursts[0]
		}
		c.wLimiter = rate.NewLimiter(bytesPerSec, burst)
		c.wLimiter.AllowN(time.Now(), burst) // spend initial burst
	}
}
