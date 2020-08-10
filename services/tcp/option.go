package tcp

import (
	"github.com/thinkgos/jocasta/core/idns"
	"github.com/thinkgos/jocasta/lib/logger"
	"github.com/thinkgos/jocasta/pkg/sword"
)

type Option func(t *TCP)

func WithLogger(l logger.Logger) Option {
	return func(t *TCP) {
		if l != nil {
			t.log = l
		}
	}
}

func WithGPool(pool sword.GoPool) Option {
	return func(t *TCP) {
		t.gPool = pool
	}
}

func WithDNSResolver(dns *idns.Resolver) Option {
	return func(t *TCP) {
		if dns != nil {
			t.dnsResolver = dns
		}
	}
}

func WithUDPIdleTime(sec int64) Option {
	return func(t *TCP) {
		if sec > 0 {
			t.udpIdleTime = sec
		}
	}
}
