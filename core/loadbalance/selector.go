package loadbalance

import (
	"hash/fnv"
	"math/rand"
	"net"
	"sync/atomic"
	"time"
)

func init() {
	RegisterSelector("random", func() Selector { return new(Random) })
	RegisterSelector("roundrobin", func() Selector { return new(RoundRobin) })
	RegisterSelector("leastconn", func() Selector { return new(LeastConn) })
	RegisterSelector("hash", func() Selector { return new(IPHash) })
	RegisterSelector("addrhash", func() Selector { return new(AddrHash) })
	RegisterSelector("leasttime", func() Selector { return new(LeastTime) })
	RegisterSelector("weight", func() Selector { return new(Weight) })
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

// Select implement Selector
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

// Select implement Selector
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

// Weight weight 根据权重和连接数
type Weight struct{}

// Select implement Selector
func (Weight) Select(pool UpstreamPool, _ string) (b *Upstream) {
	if len(pool) == 0 {
		return
	}
	if len(pool) == 1 {
		return pool[0]
	}

	min := pool[0].ConnsCount() / int64(pool[0].Weight)
	index := 0
	for i, b := range pool {
		if b.Available() {
			min = b.ConnsCount() / int64(b.Weight)
			index = i
			break
		}
	}
	for i, b := range pool {
		if b.Available() && b.ConnsCount()/int64(b.Weight) <= min {
			min = b.ConnsCount()
			index = i
		}
	}
	return pool[index]
}

// LeastTime 最小响应时间
type LeastTime struct{}

// Select implement Selector
func (LeastTime) Select(pool UpstreamPool, _ string) (b *Upstream) {
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
