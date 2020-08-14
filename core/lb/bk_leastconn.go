package lb

var _ Selector = (*LeastConn)(nil)

type LeastConn struct {
	upstreams Upstreams
}

// NewLeastConn new 使用最小连接数的
func NewLeastConn(backends []*Backend) Selector {
	return &LeastConn{upstreams: backends}
}

// Select implement Selector
func (sf *LeastConn) Select(srcAddr string) (addr string) {
	return sf.SelectBackend(srcAddr).Address
}

// SelectBackend implement Selector
func (sf *LeastConn) SelectBackend(srcAddr string) (b *Backend) {
	if sf.upstreams.Len() == 0 {
		return
	}
	if sf.upstreams.Len() == 1 {
		return sf.upstreams[0]
	}

	if found := sf.upstreams.HasActive(); !found {
		return sf.upstreams[0]
	}

	min := sf.upstreams[0].Connections()
	index := 0
	for i, b := range sf.upstreams {
		if b.Active() {
			min = b.Connections()
			index = i
			break
		}
	}
	for i, b := range sf.upstreams {
		if b.Active() && b.Connections() <= min {
			min = b.Connections()
			index = i
		}
	}
	return sf.upstreams[index]
}

// IncreaseConns implement Selector
func (sf *LeastConn) IncreaseConns(addr string) {
	sf.upstreams.IncreaseConns(addr)
}

// DecreaseConns implement Selector
func (sf *LeastConn) DecreaseConns(addr string) {
	sf.upstreams.DecreaseConns(addr)
}

// Stop implement Selector
func (sf *LeastConn) Stop() {
	sf.upstreams.Stop()
}

// HasActive implement Selector
func (sf *LeastConn) HasActive() bool {
	return sf.upstreams.HasActive()
}

// ActiveCount implement Selector
func (sf *LeastConn) ActiveCount() int {
	return sf.upstreams.ActiveCount()
}
