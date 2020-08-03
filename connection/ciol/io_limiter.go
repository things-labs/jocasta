// 实现 net.conn 网络io限速器接口
package ciol

import (
	"context"
	"net"
	"time"

	"golang.org/x/time/rate"
)

// Conn limiter conn
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

// Read reads data from the connection.
func (sf *Conn) Read(p []byte) (int, error) {
	n, err := sf.Conn.Read(p)
	if err != nil || sf.rLimiter == nil {
		return n, err
	}
	return n, sf.rLimiter.WaitN(sf.ctx, n)
}

// Write writes data to the connection.
func (sf *Conn) Write(p []byte) (int, error) {
	n, err := sf.Conn.Write(p)
	if err != nil || sf.wLimiter == nil {
		return n, err
	}
	return n, sf.wLimiter.WaitN(sf.ctx, n)
}

// Close close the Conn
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

// ReadLimit returns the maximum overall event read rate.
func (sf *Conn) ReadLimit() rate.Limit {
	return sf.rLimiter.Limit()
}

// SetReadLimit sets a new read Limit for the limiter.
func (sf *Conn) SetReadLimit(newLimit rate.Limit) {
	sf.rLimiter.SetLimit(newLimit)
}

// SetReadLimitAt sets a new read Limit for the limiter.
func (sf *Conn) SetReadLimitAt(now time.Time, newLimit rate.Limit) {
	sf.rLimiter.SetLimitAt(now, newLimit)
}

// SetReadBurst sets a new read burst size for the limiter.
func (sf *Conn) SetReadBurst(newBurst int) {
	sf.rLimiter.SetBurst(newBurst)
}

// SetReadBurstAt sets a new read read size for the limiter.
func (sf *Conn) SetReadBurstAt(now time.Time, newBurst int) {
	sf.rLimiter.SetBurstAt(now, newBurst)
}

// WriteLimit returns the maximum overall event write rate.
func (sf *Conn) WriteLimit() rate.Limit {
	return sf.wLimiter.Limit()
}

// SetWriteLimit sets a new write Limit for the limiter.
func (sf *Conn) SetWriteLimit(newLimit rate.Limit) {
	sf.wLimiter.SetLimit(newLimit)
}

// SetWriteLimitAt sets a new write Limit for the limiter.
func (sf *Conn) SetWriteLimitAt(now time.Time, newLimit rate.Limit) {
	sf.wLimiter.SetLimitAt(now, newLimit)
}

// SetWriteBurst sets a new read write size for the limiter.
func (sf *Conn) SetWriteBurst(newBurst int) {
	sf.wLimiter.SetBurst(newBurst)
}

// SetWriteBurstAt sets a new read write size for the limiter.
func (sf *Conn) SetWriteBurstAt(now time.Time, newBurst int) {
	sf.wLimiter.SetBurstAt(now, newBurst)
}
