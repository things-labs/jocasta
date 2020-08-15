package loadbalance

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"time"
)

// Config 后端配置
type Config struct {
	Addr             string        // 后端地址
	Weight           int           // 权重 default: 1
	MaxConnections   int           // 最大连接数,<= 0表示不限制, default: 0
	SuccessThreshold uint32        // liveness成功阀值 default: 3
	FailureThreshold uint32        // liveness失败阀值 default: 3
	Period           time.Duration // 检查时间间隔  default: 2s TODO: Not used
	Timeout          time.Duration // dial 连接超时时间 default: 1s
	LivenessProbe    func(ctx context.Context, addr string, timeout time.Duration) error
}

// Upstream 后端
type Upstream struct {
	Config
	health       uint32       // 是否健康
	successCount uint32       // success count
	failureCount uint32       // failure count
	connections  int64        // 连接数
	leastTime    atomic.Value // time.Duration 最小响应时间
}

// NewUpstream new a upstream
func NewUpstream(config Config) (*Upstream, error) {
	_, _, err := net.SplitHostPort(config.Addr)
	if err != nil {
		return nil, errors.New("address required like host:port")
	}
	if config.SuccessThreshold == 0 {
		config.SuccessThreshold = 3
	}
	if config.FailureThreshold == 0 {
		config.FailureThreshold = 3
	}
	if config.Weight <= 0 {
		config.Weight = 1
	}
	if config.Timeout <= 0 {
		config.Timeout = time.Second * 1
	}
	if config.Period <= 0 {
		config.Period = time.Second * 30
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
func (sf *Upstream) Healthy() bool { return atomic.LoadUint32(&sf.health) == 1 }

// Available return health and not connections not full
func (sf *Upstream) Available() bool { return sf.Healthy() && !sf.Full() }

// LeastTime return the least time connect
func (sf *Upstream) LeastTime() time.Duration { return sf.leastTime.Load().(time.Duration) }

// Full return connections is full or not.
func (sf *Upstream) Full() bool {
	return sf.MaxConnections > 0 && sf.connections >= int64(sf.MaxConnections)
}

// Monitoring the backend
func (sf *Upstream) healthyCheck(addr string) {
	livenessProbe := tcpLivenessProbe
	if sf.LivenessProbe != nil {
		livenessProbe = sf.LivenessProbe
	}
	start := time.Now()
	err := livenessProbe(context.TODO(), addr, sf.Timeout)
	sf.leastTime.Store(time.Since(start))
	if err != nil {
		// Max tries larger than consider max inactive, health failed
		if failure := atomic.AddUint32(&sf.failureCount, 1); failure >= sf.FailureThreshold {
			atomic.StoreUint32(&sf.successCount, 0)
			atomic.StoreUint32(&sf.health, 0)
		}
	} else {
		// Max tries larger than consider max health, health success
		if success := atomic.AddUint32(&sf.successCount, 1); success >= sf.SuccessThreshold {
			atomic.StoreUint32(&sf.failureCount, 0)
			atomic.StoreUint32(&sf.health, 1)
		}
	}
}

func tcpLivenessProbe(_ context.Context, addr string, timeout time.Duration) error {
	c, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return err
	}
	c.Close() // nolint: errcheck
	return nil
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
