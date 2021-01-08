package filter

import (
	"context"
	"time"

	"github.com/thinkgos/x/gopool"
	"github.com/thinkgos/x/lib/logger"
)

// 选项
type Option func(f *Filter)

// WithLogger 使用自定义logger
func WithLogger(log logger.Logger) Option {
	return func(f *Filter) {
		if log != nil {
			f.log = log
		}
	}
}

// WithGPool 使用协程池
func WithGPool(pool gopool.Pool) Option {
	return func(f *Filter) {
		if pool != nil {
			f.gPool = pool
		}
	}
}

// WithTimeout 域名检测超时时间, default: 1s
func WithTimeout(timeout time.Duration) Option {
	return func(f *Filter) {
		f.timeout = timeout
	}
}

// WithLivenessPeriod 域名存活控测周期, default: 30s
func WithLivenessPeriod(period time.Duration) Option {
	return func(f *Filter) {
		f.livenessPeriod = period
	}
}

// WithLivenessProbe 域名探针接口, default: tcp dail(tcp 连接测试)
func WithLivenessProbe(livenessProbe func(ctx context.Context, addr string, timeout time.Duration) error) Option {
	return func(f *Filter) {
		f.livenessProbe = livenessProbe
	}
}

// WithAliveThreshold 当探测成功后,多久时间内表示此域名均是通的.
// < 60: use defaultAliveThreshold 1800s
func WithAliveThreshold(sec int64) Option {
	return func(f *Filter) {
		if sec < 60 {
			sec = defaultAliveThreshold
		}
		f.aliveThreshold = sec
	}
}

// WithSuccessThreshold 成功次数达到指定阀值才算成功,默认3次
// 0: use defaultThreshold 3次
func WithSuccessThreshold(cnt uint) Option {
	return func(f *Filter) {
		if cnt == 0 {
			cnt = defaultThreshold
		}
		f.successThreshold = cnt
	}
}

// WithFailureThreshold 失败次数达到指定阀值才算失败,默认3次
// 0: use defaultThreshold 3次
func WithFailureThreshold(cnt uint) Option {
	return func(f *Filter) {
		if cnt == 0 {
			cnt = defaultThreshold
		}
		f.failureThreshold = cnt
	}
}
