package lb

import (
	"crypto/md5"
	"net"
)

// Selector select the backend
type Selector interface {
	// 根据源地址,获得后端连接
	SelectBackend(srcAddr string) *Backend
	// Backends return all upstreams
	Backends() Upstreams
	// ConnsIncrease increase the addr conns count
	ConnsIncrease(addr string)
	// ConnsDecrease decrease the addr conns count
	ConnsDecrease(addr string)
	// HasActive has any active a backend
	HasActive() bool
	// Stop stop all the backend
	Stop()
	// ActiveCount active backend count
	ActiveCount() int
}

// RoundRobin round robin 轮询模式
type RoundRobin struct {
	backendIndex int
	Upstreams
}

// NewRoundRobin new round robin
func NewRoundRobin(upstreams Upstreams) Selector {
	return &RoundRobin{Upstreams: upstreams}
}

// SelectBackend implement Selector
func (sf *RoundRobin) SelectBackend(srcAddr string) (b *Backend) {
	if len(sf.Upstreams) == 0 {
		return
	}
	if len(sf.Upstreams) == 1 {
		return sf.Upstreams[0]
	}

RETRY:
	if found := sf.HasActive(); !found {
		return sf.Upstreams[0]
	}
	sf.backendIndex++
	if sf.backendIndex > len(sf.Upstreams)-1 {
		sf.backendIndex = 0
	}
	if !sf.Upstreams[sf.backendIndex].Active() {
		goto RETRY
	}
	return sf.Upstreams[sf.backendIndex]
}

// LeastConn least conn 使用最小连接数的
type LeastConn struct {
	Upstreams
}

// NewLeastConn new least conn
func NewLeastConn(upstreams Upstreams) Selector {
	return &LeastConn{upstreams}
}

// SelectBackend implement Selector
func (sf *LeastConn) SelectBackend(srcAddr string) (b *Backend) {
	if len(sf.Upstreams) == 0 {
		return
	}
	if len(sf.Upstreams) == 1 {
		return sf.Upstreams[0]
	}

	if found := sf.HasActive(); !found {
		return sf.Upstreams[0]
	}

	min := sf.Upstreams[0].ConnsCount()
	index := 0
	for i, b := range sf.Upstreams {
		if b.Active() {
			min = b.ConnsCount()
			index = i
			break
		}
	}
	for i, b := range sf.Upstreams {
		if b.Active() && b.ConnsCount() <= min {
			min = b.ConnsCount()
			index = i
		}
	}
	return sf.Upstreams[index]
}

// Hash ip hash 实现
type Hash struct {
	Upstreams
}

// NewHash new hash
func NewHash(upstreams Upstreams) Selector {
	return &Hash{upstreams}
}

// SelectBackend implement Selector
func (sf *Hash) SelectBackend(srcAddr string) *Backend {
	if len(sf.Upstreams) == 0 {
		return nil
	}
	if len(sf.Upstreams) == 1 {
		return sf.Upstreams[0]
	}

	host, _, err := net.SplitHostPort(srcAddr)
	if err != nil {
		host = srcAddr
	}

	i := 0
	for _, b := range md5.Sum([]byte(host)) {
		i += int(b)
	}

	if active := sf.HasActive(); !active {
		return sf.Upstreams[0]
	}
RETRY:
	k := i % len(sf.Upstreams)
	if !sf.Upstreams[k].Active() {
		i++
		goto RETRY
	}
	return sf.Upstreams[k]
}

// Weight weight 根据权重和连接数
type Weight struct {
	Upstreams
}

// NewWeight new a weight
func NewWeight(upstreams Upstreams) Selector {
	return &Weight{upstreams}
}

// SelectBackend implement Selector
func (sf *Weight) SelectBackend(srcAddr string) (b *Backend) {
	if len(sf.Upstreams) == 0 {
		return
	}
	if len(sf.Upstreams) == 1 {
		return sf.Upstreams[0]
	}

	found := sf.HasActive()
	if !found {
		return sf.Upstreams[0]
	}

	min := sf.Upstreams[0].ConnsCount() / int64(sf.Upstreams[0].Weight)
	index := 0
	for i, b := range sf.Upstreams {
		if b.Active() {
			min = b.ConnsCount() / int64(b.Weight)
			index = i
			break
		}
	}
	for i, b := range sf.Upstreams {
		if b.Active() && b.ConnsCount()/int64(b.Weight) <= min {
			min = b.ConnsCount()
			index = i
		}
	}
	return sf.Upstreams[index]
}

// LeastTime 使用连接时间最小的
type LeastTime struct {
	Upstreams
}

// NewLeastTime new a least time selector
func NewLeastTime(upstreams Upstreams) Selector {
	return &LeastTime{upstreams}
}

// SelectBackend implement Selector
func (sf *LeastTime) SelectBackend(srcAddr string) (b *Backend) {
	if len(sf.Upstreams) == 0 {
		return
	}
	if len(sf.Upstreams) == 1 {
		return sf.Upstreams[0]
	}

	if found := sf.HasActive(); !found {
		return sf.Upstreams[0]
	}

	min := sf.Upstreams[0].ConnectUsedTime()
	index := 0
	for i, b := range sf.Upstreams {
		if b.Active() {
			min = b.ConnectUsedTime()
			index = i
			break
		}
	}
	for i, b := range sf.Upstreams {
		if b.Active() && b.ConnectUsedTime() > 0 && b.ConnectUsedTime() <= min {
			min = b.ConnectUsedTime()
			index = i
		}
	}
	return sf.Upstreams[index]
}
