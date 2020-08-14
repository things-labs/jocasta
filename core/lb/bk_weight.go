package lb

var _ Selector = (*Weight)(nil)

type Weight struct {
	upstreams Upstreams
}

// NewWeight 根据上级的权重和连接数,选一个上级
func NewWeight(backends []*Backend) Selector {
	return &Weight{upstreams: backends}
}

// Select implement Selector
func (sf *Weight) Select(srcAddr string) (addr string) {
	return sf.SelectBackend(srcAddr).Address
}

// SelectBackend implement Selector
func (sf *Weight) SelectBackend(srcAddr string) (b *Backend) {
	if sf.upstreams.Len() == 0 {
		return
	}
	if sf.upstreams.Len() == 1 {
		return sf.upstreams[0]
	}

	found := sf.upstreams.HasActive()
	if !found {
		return sf.upstreams[0]
	}

	min := sf.upstreams[0].Connections() / int64(sf.upstreams[0].Weight)
	index := 0
	for i, b := range sf.upstreams {
		if b.Active() {
			min = b.Connections() / int64(b.Weight)
			index = i
			break
		}
	}
	for i, b := range sf.upstreams {
		if b.Active() && b.Connections()/int64(b.Weight) <= min {
			min = b.Connections()
			index = i
		}
	}
	return sf.upstreams[index]
}

// IncreaseConns implement Selector
func (sf *Weight) IncreaseConns(addr string) {
	sf.upstreams.IncreaseConns(addr)
}

// DecreaseConns implement Selector
func (sf *Weight) DecreaseConns(addr string) {
	sf.upstreams.DecreaseConns(addr)
}

// Stop implement Selector
func (sf *Weight) Stop() {
	sf.upstreams.Stop()
}

// HasActive implement Selector
func (sf *Weight) HasActive() bool {
	return sf.upstreams.HasActive()
}

// ActiveCount implement Selector
func (sf *Weight) ActiveCount() int {
	return sf.upstreams.ActiveCount()
}
