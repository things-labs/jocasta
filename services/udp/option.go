package udp

import (
	"github.com/thinkgos/jocasta/core/idns"
	"github.com/thinkgos/jocasta/lib/logger"
	"github.com/thinkgos/jocasta/pkg/sword"
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

// WithGPool 配置goroutine池,默认不用池
func WithGPool(pool sword.GoPool) Option {
	return func(t *UDP) { t.gPool = pool }
}

// WithDNSResolver 配置DNS服务器
func WithDNSResolver(dns *idns.Resolver) Option {
	return func(t *UDP) { t.dnsResolver = dns }
}
