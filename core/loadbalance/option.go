package loadbalance

import (
	"time"

	"github.com/thinkgos/jocasta/core/idns"
	"github.com/thinkgos/jocasta/lib/gpool"
	"github.com/thinkgos/jocasta/lib/logger"
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
func WithGPool(pool gpool.Pool) Option {
	return func(g *Balanced) {
		g.goPool = pool
	}
}

func WithInterval(interval time.Duration) Option {
	return func(g *Balanced) {
		g.interval = interval
	}
}
