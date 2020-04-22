package connection

import (
	"context"
	"time"

	cmap "github.com/orcaman/concurrent-map"
)

type Manager struct {
	cmap.ConcurrentMap
	interval time.Duration
	// gc回调,返回true将删除对应的key
	gcIterCb func(key string, value interface{}, now time.Time) bool
}

func New(gcInterval time.Duration, gcIterCb func(key string, value interface{}, now time.Time) bool) *Manager {
	return &Manager{
		cmap.New(),
		gcInterval,
		gcIterCb,
	}
}

func (sf *Manager) RunWatch(ctx context.Context) {
	if sf.interval <= 0 {
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

		for k, v := range sf.Items() {
			if sf.gcIterCb(k, v, now) {
				sf.Remove(k)
			}
		}
	}
}
