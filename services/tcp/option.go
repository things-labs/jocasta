package tcp

import (
	"github.com/thinkgos/jocasta/core/idns"
	"github.com/thinkgos/jocasta/pkg/logger"
)

type Option func(t *TCP)

func WithLogger(l logger.Logger) Option {
	return func(t *TCP) {
		if l != nil {
			t.log = l
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

func WithUDPIdleTime(sec int64) Option {
	return func(t *TCP) {
		if sec > 0 {
			t.udpIdleTime = sec
		}
	}
}
