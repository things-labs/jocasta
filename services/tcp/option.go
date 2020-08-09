package tcp

import (
	"github.com/thinkgos/jocasta/core/idns"
	"github.com/thinkgos/jocasta/lib/gpool"
	"github.com/thinkgos/jocasta/lib/logger"
)

type Option func(t *TCP)

func WithLogger(l logger.Logger) Option {
	return func(t *TCP) {
		if l != nil {
			t.log = l
		}
	}
}

func WithGPool(pool gpool.Pool) Option {
	return func(t *TCP) {
		if pool != nil {
			t.gPool = pool
		}
	}
}

func WithDNSResolver(dns *idns.Resolver) Option {
	return func(t *TCP) {
		if dns != nil {
			t.dnsResolver = dns
		}
	}
}
