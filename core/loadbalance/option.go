package loadbalance

import (
	"time"

	"github.com/thinkgos/go-core-package/gopool"
	"github.com/thinkgos/go-core-package/lib/logger"
	"github.com/thinkgos/jocasta/core/idns"
)

// Option 配置选项
type Option func(*Balanced)

// WithLogger 配置日志
func WithLogger(log logger.Logger) Option {
	return func(g *Balanced) {
		g.log = log
	}
}

// WithDNSServer 设置DNS服务器,用于解析url
func WithDNSServer(dns *idns.Resolver) Option {
	return func(g *Balanced) {
		g.dns = dns
	}
}

// WithEnableDebug 使能debug输出
func WithEnableDebug(b bool) Option {
	return func(g *Balanced) {
		g.debug = b
	}
}

// WithGPool 使用协程池
func WithGPool(pool gopool.Pool) Option {
	return func(g *Balanced) {
		g.goPool = pool
	}
}

// WithInterval 活性探测间隔
func WithInterval(interval time.Duration) Option {
	return func(g *Balanced) {
		g.interval = interval
	}
}
