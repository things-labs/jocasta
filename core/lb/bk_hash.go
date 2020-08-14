package lb

import (
	"crypto/md5"
	"net"
)

// Hash ip hash 实现
type Hash struct {
	upstreams Upstreams
}

// NewHash new hash
func NewHash(backends []*Backend) Selector {
	return &Hash{upstreams: backends}
}

// Select implement Selector
func (sf *Hash) Select(srcAddr string) (addr string) {
	return sf.SelectBackend(srcAddr).Address
}

// SelectBackend implement Selector
func (sf *Hash) SelectBackend(srcAddr string) *Backend {
	if sf.upstreams.Len() == 0 {
		return nil
	}
	if sf.upstreams.Len() == 1 {
		return sf.upstreams[0]
	}

	host, _, err := net.SplitHostPort(srcAddr)
	if err != nil {
		host = srcAddr
	}

	i := 0
	for _, b := range md5.Sum([]byte(host)) {
		i += int(b)
	}

	if active := sf.upstreams.HasActive(); !active {
		return sf.upstreams[0]
	}
RETRY:
	k := i % len(sf.upstreams)
	if !sf.upstreams[k].Active() {
		i++
		goto RETRY
	}
	return sf.upstreams[k]
}

// IncreaseConns implement Selector
func (sf *Hash) IncreaseConns(string) {}

// DecreaseConns implement Selector
func (sf *Hash) DecreaseConns(string) {}

// Stop implement Selector
func (sf *Hash) Stop() {
	sf.upstreams.Stop()
}

// HasActive implement Selector
func (sf *Hash) HasActive() bool {
	return sf.upstreams.HasActive()
}

// ActiveCount implement Selector
func (sf *Hash) ActiveCount() int {
	return sf.upstreams.ActiveCount()
}
