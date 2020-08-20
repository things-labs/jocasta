package binding

import (
	"github.com/thinkgos/jocasta/lib/gopool"
)

// Option for Forward
type Option func(c *Forward)

// WithGPool with gpool.Pool
func WithGPool(pool gopool.Pool) Option {
	return func(c *Forward) {
		if pool != nil {
			c.gPool = pool
		}
	}
}
