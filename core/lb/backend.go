package lb

import (
	"errors"
	"fmt"
	"net"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/thinkgos/jocasta/core/idns"
	"github.com/thinkgos/jocasta/lib/logger"
)

// Config 后端配置
type Config struct {
	Address     string        // 后端地址
	MinActive   int           // 最大测试已激活次数
	MaxInactive int           // 最大测试未激活次数
	Weight      int           // 权重
	Timeout     time.Duration // 连接超时时间
	RetryTime   time.Duration // 检查时间间隔
	IsMuxCheck  bool          // TODO: not used
	ConnFactory func(address string, timeout time.Duration) (net.Conn, error)
}

// Backend 后端
type Backend struct {
	Config
	active          int32        // 是否处于活动状态
	connections     int64        // 连接数
	connectUsedTime atomic.Value // time.Duration dial 连接使用的时间 单位ms
	mu              sync.Mutex
	hasClosed       bool
	stop            chan struct{}
	dns             *idns.Resolver
	log             logger.Logger
}

func NewBackend(config Config, dns *idns.Resolver, log logger.Logger) (*Backend, error) {
	if config.Address == "" {
		return nil, errors.New("address required")
	}
	if config.MinActive == 0 {
		config.MinActive = 3
	}
	if config.MaxInactive == 0 {
		config.MaxInactive = 3
	}
	if config.Weight == 0 {
		config.Weight = 1
	}
	if config.Timeout == 0 {
		config.Timeout = time.Millisecond * 1500
	}
	if config.RetryTime == 0 {
		config.RetryTime = time.Second * 2
	}

	b := &Backend{
		Config: config,
		stop:   make(chan struct{}, 1),
		dns:    dns,
		log:    log,
	}
	b.connectUsedTime.Store(time.Duration(0))
	return b, nil
}

// ConnsCount connection count
func (b *Backend) ConnsCount() int64 { return atomic.LoadInt64(&b.connections) }

// ConnsIncrease connection count increase one
func (b *Backend) ConnsIncrease() { atomic.AddInt64(&b.connections, 1) }

// ConnsDecrease connection count decrease one
func (b *Backend) ConnsDecrease() { atomic.AddInt64(&b.connections, -1) }

// Active return active or not
func (b *Backend) Active() bool { return atomic.LoadInt32(&b.active) == 1 }

func (b *Backend) ConnectUsedTime() time.Duration { return b.connectUsedTime.Load().(time.Duration) }

func (b *Backend) StartHeartCheck() {
	if b.IsMuxCheck {
		go b.runMuxHeartCheck()
	} else {
		go b.runTCPHeartCheck()
	}
}

func (b *Backend) StopHeartCheck() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.hasClosed {
		b.hasClosed = true
		close(b.stop)
		return
	}
}

func (b *Backend) runMuxHeartCheck() {
	var t = time.NewTicker(b.RetryTime)

	defer func() {
		t.Stop()
		if e := recover(); e != nil {
			fmt.Printf("crashed %s\nstack:\n%s", e, string(debug.Stack()))
		}
	}()

	buf := make([]byte, 1)
	for {
		start := time.Now()
		c, err := b.getConn()
		b.connectUsedTime.Store(time.Since(start))
		active := int32(1)
		if err != nil {
			active = 0
		}
		atomic.StoreInt32(&b.active, active)
		if err == nil {
			c.Read(buf)
		}

		select {
		case <-b.stop:
			return
		case <-t.C:
		}
	}
}

// Monitoring the backend
func (b *Backend) runTCPHeartCheck() {
	var activeTries int
	var inactiveTries int
	var t = time.NewTicker(b.RetryTime)

	defer func() {
		t.Stop()
		if e := recover(); e != nil {
			b.log.DPanicf("crashed %s\nstack:\n%s", e, string(debug.Stack()))
		}
	}()

	for {
		start := time.Now()
		c, err := b.getConn()
		b.connectUsedTime.Store(time.Since(start))
		if err != nil {
			// Max tries larger than consider max inactive, active failed
			if inactiveTries++; inactiveTries >= b.MaxInactive {
				activeTries = 0
				atomic.StoreInt32(&b.active, 0)
			}
		} else {
			c.Close()
			// Max tries larger than consider max active, active success
			if activeTries++; activeTries >= b.MinActive {
				inactiveTries = 0
				atomic.StoreInt32(&b.active, 1)
			}
		}
		select {
		case <-b.stop:
			return
		case <-t.C:
		}
	}
}

func (b *Backend) getConn() (conn net.Conn, err error) {
	address := b.Address
	if b.dns != nil && b.dns.PublicDNSAddr() != "" {
		if address, err = b.dns.Resolve(b.Address); err != nil {
			b.log.Errorf("dns resolve address: %s, %+v", b.Address, err)
		}
	}
	if b.ConnFactory != nil {
		return b.ConnFactory(address, b.Timeout)
	}
	return net.DialTimeout("tcp", address, b.Timeout)
}

/******************************************************************************/

// Upstreams upstream backend
type Upstreams []*Backend

// NewUpstreams new stream
func NewUpstreams(configs []Config, dr *idns.Resolver, log logger.Logger) Upstreams {
	bks := make([]*Backend, 0, len(configs))
	for _, c := range configs {
		b, err := NewBackend(c, dr, log)
		if err != nil {
			continue
		}
		b.StartHeartCheck()
		bks = append(bks, b)
	}
	return bks
}

// Len return upstreams total length
func (ups Upstreams) Len() int { return len(ups) }

// Backends return all upstreams
func (ups Upstreams) Backends() Upstreams { return ups }

// ConnsIncrease increase the addr conns count
func (ups Upstreams) ConnsIncrease(addr string) {
	for _, bk := range ups {
		if bk.Address == addr {
			bk.ConnsIncrease()
			return
		}
	}
}

// ConnsDecrease decrease the addr conns count
func (ups Upstreams) ConnsDecrease(addr string) {
	for _, bk := range ups {
		if bk.Address == addr {
			bk.ConnsDecrease()
			return
		}
	}
}

// HasActive has any active a backend
func (ups Upstreams) HasActive() bool {
	for _, b := range ups {
		if b.Active() {
			return true
		}
	}
	return false
}

// Stop stop all the backend
func (ups Upstreams) Stop() {
	for _, b := range ups {
		b.StopHeartCheck()
	}
}

// ActiveCount active backend count
func (ups Upstreams) ActiveCount() (count int) {
	for _, b := range ups {
		if b.Active() {
			count++
		}
	}
	return
}
