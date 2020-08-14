package loadbalance

import (
	"errors"
	"net"
	"sync/atomic"
	"time"
)

// Config 后端配置
type Config struct {
	Addr        string        // 后端地址
	MinActive   int           // 最大测试已激活次数
	MaxInactive int           // 最大测试未激活次数
	Weight      int           // 权重
	Timeout     time.Duration // 连接超时时间
	RetryTime   time.Duration // 检查时间间隔
	Dial        func(address string, timeout time.Duration) (net.Conn, error)
}

// Upstream 后端
type Upstream struct {
	Config
	health         int32        // 是否健康
	connections    int64        // 连接数
	maxConnections int64        // 最大连接数,0表示不限制, default: 0
	leastTime      atomic.Value // time.Duration 最小响应时间
}

// NewUpstream new a upstream
func NewUpstream(config Config) (*Upstream, error) {
	if config.Addr == "" {
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

	b := &Upstream{Config: config}
	b.leastTime.Store(time.Duration(0))
	return b, nil
}

// ConnsCount connection count
func (sf *Upstream) ConnsCount() int64 { return atomic.LoadInt64(&sf.connections) }

// ConnsIncrease connection count increase one
func (sf *Upstream) ConnsIncrease() { atomic.AddInt64(&sf.connections, 1) }

// ConnsDecrease connection count decrease one
func (sf *Upstream) ConnsDecrease() { atomic.AddInt64(&sf.connections, -1) }

// Healthy return health or not
func (sf *Upstream) Healthy() bool { return atomic.LoadInt32(&sf.health) == 1 }

func (sf *Upstream) Available() bool { return atomic.LoadInt32(&sf.health) == 1 && !sf.Full() }

func (sf *Upstream) LeastTime() time.Duration { return sf.leastTime.Load().(time.Duration) }

func (sf *Upstream) Full() bool { return sf.maxConnections > 0 && sf.connections >= sf.maxConnections }

// Monitoring the backend
func (sf *Upstream) tcpHealthyCheck(addr string) {
	var activeTries int
	var inactiveTries int
	var c net.Conn
	var err error

	start := time.Now()
	if sf.Dial != nil {
		c, err = sf.Dial(addr, sf.Timeout)
	} else {
		c, err = net.DialTimeout("tcp", addr, sf.Timeout)
	}

	sf.leastTime.Store(time.Since(start))
	if err != nil {
		// Max tries larger than consider max inactive, health failed
		if inactiveTries++; inactiveTries >= sf.MaxInactive {
			activeTries = 0
			atomic.StoreInt32(&sf.health, 0)
		}
	} else {
		c.Close()
		// Max tries larger than consider max health, health success
		if activeTries++; activeTries >= sf.MinActive {
			inactiveTries = 0
			atomic.StoreInt32(&sf.health, 1)
		}
	}
}

/******************************************************************************/

// UpstreamPool upstream pool
type UpstreamPool []*Upstream

// NewUpstreamPool new stream pool
func NewUpstreamPool(configs []Config) UpstreamPool {
	bks := make([]*Upstream, 0, len(configs))
	for _, c := range configs {
		b, err := NewUpstream(c)
		if err != nil {
			continue
		}
		bks = append(bks, b)
	}
	return bks
}

// Len return upstreams total length
func (ups UpstreamPool) Len() int { return len(ups) }

// ConnsIncrease increase the addr conns count
func (ups UpstreamPool) ConnsIncrease(addr string) {
	for _, bk := range ups {
		if bk.Addr == addr {
			bk.ConnsIncrease()
			return
		}
	}
}

// ConnsDecrease decrease the addr conns count
func (ups UpstreamPool) ConnsDecrease(addr string) {
	for _, bk := range ups {
		if bk.Addr == addr {
			bk.ConnsDecrease()
			return
		}
	}
}

// HasHealthy has any health upstream
func (ups UpstreamPool) HasHealthy() bool {
	for _, b := range ups {
		if b.Healthy() {
			return true
		}
	}
	return false
}

// HealthyCount health backend count
func (ups UpstreamPool) HealthyCount() (count int) {
	for _, b := range ups {
		if b.Healthy() {
			count++
		}
	}
	return
}
