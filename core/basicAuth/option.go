package basicAuth

import (
	"time"

	"github.com/thinkgos/jocasta/core/idns"
)

// Option for Center
type Option func(c *Center)

// WithDNSServer 设置DNS服务器,用于解析url
func WithDNSServer(dns *idns.Resolver) Option {
	return func(c *Center) {
		c.SetDNSServer(dns)
	}
}

// WithAuthURL 设置第三方basic auth 中心认证服务. url, 超时时间, 成功码, 重试次数.
func WithAuthURL(url string, timeout time.Duration, code int, retry uint) Option {
	return func(c *Center) {
		c.SetAuthURL(url, timeout, code, retry)
	}
}
