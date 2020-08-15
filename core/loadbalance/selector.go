package loadbalance

import (
	"hash/fnv"
	"math/rand"
	"net"
	"sync/atomic"
	"time"

	"github.com/thinkgos/jocasta/lib/outil"
)

func init() {
	RegisterSelector("random", func() Selector { return new(Random) })
	RegisterSelector("roundrobin", func() Selector { return new(RoundRobin) })
	RegisterSelector("leastconn", func() Selector { return new(LeastConn) })
	RegisterSelector("hash", func() Selector { return new(IPHash) })
	RegisterSelector("addrhash", func() Selector { return new(AddrHash) })
	RegisterSelector("leasttime", func() Selector { return new(LeastTime) })
	RegisterSelector("weight", func() Selector { return NewWeight() })
}

// Random is a policy that selects an available backend at random.
type Random struct{}

// Select implement Selector
func (Random) Select(pool UpstreamPool, _ string) *Upstream {
	// use reservoir sampling because the number of available
	// hosts isn't known: https://en.wikipedia.org/wiki/Reservoir_sampling
	var b *Upstream
	var count int

	for _, upstream := range pool {
		if upstream.Available() {
			// (n % 1 == 0) holds for all n, therefore a
			// upstream will always be chosen if there is at
			// least one available
			count++
			if (rand.Int() % count) == 0 {
				b = upstream
			}
		}
	}
	return b
}

// RoundRobin round robin 轮询模式
type RoundRobin struct {
	robin uint32
}

// Select implement Selector
func (sf *RoundRobin) Select(pool UpstreamPool, _ string) *Upstream {
	for i, n := uint32(0), uint32(len(pool)); i < n; i++ {
		newRobin := atomic.AddUint32(&sf.robin, 1)
		b := pool[newRobin%n]
		if b.Available() {
			return b
		}
	}
	return nil
}

// LeastConn least conn 使用最小连接数的
type LeastConn struct{}

// Select implement Selector
func (LeastConn) Select(pool UpstreamPool, _ string) *Upstream {
	var best *Upstream

	min, count := int64(-1), 0
	for _, b := range pool {
		if b.Available() {
			numConns := b.ConnsCount()
			if min == -1 || numConns < min {
				min = numConns
				count = 0
			}
			// among hosts with same least connections, perform a reservoir
			// sample: https://en.wikipedia.org/wiki/Reservoir_sampling
			if numConns == min {
				count++
				if rand.Int()%count == 0 {
					best = b
				}
			}
		}
	}
	return best
}

// IPHash ip hash 实现
type IPHash struct{}

// Select implement Selector,  if srcAddr is empty it will use random mode
func (IPHash) Select(pool UpstreamPool, srcAddr string) *Upstream {
	if srcAddr == "" {
		return Random{}.Select(pool, srcAddr)
	}
	host, _, err := net.SplitHostPort(srcAddr)
	if err != nil {
		host = srcAddr
	}
	return hashing(pool, host)
}

// AddrHash host:port hash 实现
type AddrHash struct{}

// Select implement Selector, if srcAddr is empty it will use random mode
func (AddrHash) Select(pool UpstreamPool, srcAddr string) *Upstream {
	if srcAddr == "" {
		return Random{}.Select(pool, srcAddr)
	}
	return hashing(pool, srcAddr)
}

func hashing(pool UpstreamPool, s string) *Upstream {
	poolLen := uint32(len(pool))
	index := hash(s) % poolLen
	for i := uint32(0); i < poolLen; i++ {
		upstream := pool[index%poolLen]
		if upstream.Available() {
			return upstream
		}
		index++
	}
	return nil
}

// hash calculates a fast hash based on s.
func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s)) // nolint: errcheck
	return h.Sum32()
}

// Weight weight 平滑权重轮询调度
type Weight struct {
	index     int
	curWeight int
}

// NewWeight new weight
func NewWeight() *Weight {
	return &Weight{index: -1}
}

// Select implement Selector
func (sf *Weight) Select(pool UpstreamPool, _ string) *Upstream {
	if len(pool) == 0 {
		return nil
	}
	if len(pool) == 1 {
		return pool[0]
	}
	maxWeight, gcd := getMaxWeightAndGCD(pool)
	// 轮询加权调度算法
	for i := 0; i < len(pool); i++ {
		//  index = (index + 1) mod n
		// 当 (index + 1) mod n 的余数为0时,有两种情况:
		//      1. 首次被调用执行时
		//      2. 已经轮询完一整轮, 又回到起点时
		sf.index = (sf.index + 1) % len(pool)
		if sf.index == 0 {
			// 最新权重值 = 当前权重值 - 最大公约数
			sf.curWeight = sf.curWeight - gcd
			if sf.curWeight <= 0 {
				// curWeight <= 0 时, 有两种情况:
				//      1. 首次被调用执行时, curWeight值初始化为0, 只要maxWeight最大权重数值在0以上, cw的最新值一定< 0
				//      2. 当所有的元素按照权重都被调度/选择一遍之后, curWeight的值一定为0
				//  此时需要将最大权重值(重新)赋值到curWeight
				sf.curWeight = maxWeight
			}
		}

		// 当索引值 >= 最新权重值时, 返回名称
		if pool[sf.index].Weight >= sf.curWeight && pool[sf.index].Available() {
			return pool[sf.index]
		}
	}
	return nil
}

// GetMaxWeight 获取Slice中最大权重值
func getMaxWeightAndGCD(pool UpstreamPool) (int, int) {
	maxWeight, g := pool[0].Weight, pool[0].Weight
	for i := 1; i < len(pool); i++ {
		if pool[i].Weight > maxWeight {
			maxWeight = pool[i].Weight
		}
		g = outil.Gcdx(g, pool[i].Weight)
	}
	return maxWeight, g
}

// LeastTime 最小响应时间
type LeastTime struct{}

// Select implement Selector
func (LeastTime) Select(pool UpstreamPool, _ string) *Upstream {
	var best *Upstream

	min, count := time.Duration(-1), 0
	for _, b := range pool {
		if b.Available() {
			tm := b.LeastTime()
			if min == -1 || tm < min {
				min = tm
				count = 0
			}
			// among hosts with same least connections, perform a reservoir
			// sample: https://en.wikipedia.org/wiki/Reservoir_sampling
			if tm == min {
				count++
				if rand.Int()%count == 0 {
					best = b
				}
			}
		}
	}
	return best
}
