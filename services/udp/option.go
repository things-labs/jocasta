package udp

import (
	"github.com/thinkgos/jocasta/core/idns"
	"github.com/thinkgos/jocasta/lib/logger"
	"github.com/thinkgos/jocasta/pkg/sword"
)

type Option func(u *UDP)

func WithLogger(l logger.Logger) Option {
	return func(t *UDP) {
		if l != nil {
			t.log = l
		}
	}
}

func WithGPool(pool sword.GoPool) Option {
	return func(t *UDP) {
		t.gPool = pool
	}
}

func WithDNSResolver(dns *idns.Resolver) Option {
	return func(t *UDP) {
		if dns != nil {
			t.dnsResolver = dns
		}
	}
}
