package udp

import (
	"github.com/thinkgos/go-core-package/lib/logger"
	"github.com/thinkgos/jocasta/core/idns"
)

// Option 配置选项
type Option func(udp *UDP)

// WithLogger 配置日志
func WithLogger(l logger.Logger) Option {
	return func(t *UDP) {
		if l != nil {
			t.log = l
		}
	}
}

// WithDNSResolver 配置DNS服务器
func WithDNSResolver(dns *idns.Resolver) Option {
	return func(t *UDP) { t.dnsResolver = dns }
}

func WithUDPIdleTime(sec int64) Option {
	return func(t *UDP) {
		if sec > 0 {
			t.udpIdleTime = sec
		}
	}
}
