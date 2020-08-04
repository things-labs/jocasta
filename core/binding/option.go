package binding

import (
	"github.com/thinkgos/jocasta/lib/gpool"
)

// Option for Forward
type Option func(c *Forward)

// WithGPool with gpool.Pool
func WithGPool(pool gpool.Pool) Option {
	return func(c *Forward) {
		if pool != nil {
			c.gPool = pool
		}
	}
}
