// Package connection 管理key
package connection

import (
	"context"
	"time"

	cmap "github.com/orcaman/concurrent-map"
)

// Manager 管理key
type Manager struct {
	cmap.ConcurrentMap
	interval time.Duration
	// gc回调,返回true将删除对应的key
	gcIterCb func(key string, value interface{}, now time.Time) bool
}

// New a manager
// gcInterval: 回收间隔, <=0将不启动
// gcIterCb: 回收回调函数, 当间隔到后,检查所有的key,value,返回true将删除对应的key
func New(gcInterval time.Duration, gcIterCb func(key string, value interface{}, now time.Time) bool) *Manager {
	return &Manager{
		cmap.New(),
		gcInterval,
		gcIterCb,
	}
}

// Watch watch conn interval,should run in a goroutine
func (sf *Manager) Watch(ctx context.Context) {
	if sf.interval <= 0 || sf.gcIterCb == nil {
		return
	}
	var now time.Time

	timer := time.NewTicker(sf.interval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now = <-timer.C:
		}
		// TODO: 优化删除策略
		for k, v := range sf.Items() {
			if sf.gcIterCb(k, v, now) {
				sf.Remove(k)
			}
		}
	}
}
