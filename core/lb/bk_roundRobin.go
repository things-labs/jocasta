package lb

var _ Selector = (*RoundRobin)(nil)

type RoundRobin struct {
	backendIndex int
	upstreams    Upstreams
}

// NewRoundRobin new 轮询模式
func NewRoundRobin(backends []*Backend) Selector {
	return &RoundRobin{upstreams: backends}
}

// Select implement Selector
func (sf *RoundRobin) Select(srcAddr string) (addr string) {
	return sf.SelectBackend(srcAddr).Address
}

// SelectBackend implement Selector
func (sf *RoundRobin) SelectBackend(srcAddr string) (b *Backend) {
	if sf.upstreams.Len() == 0 {
		return
	}
	if sf.upstreams.Len() == 1 {
		return sf.upstreams[0]
	}

RETRY:
	if found := sf.upstreams.HasActive(); !found {
		return sf.upstreams[0]
	}
	sf.backendIndex++
	if sf.backendIndex > len(sf.upstreams)-1 {
		sf.backendIndex = 0
	}
	if !sf.upstreams[sf.backendIndex].Active() {
		goto RETRY
	}
	return sf.upstreams[sf.backendIndex]
}

func (sf *RoundRobin) IncreaseConns(string) {}

// DecreaseConns implement Selector
func (sf *RoundRobin) DecreaseConns(string) {}

// Stop implement Selector
func (sf *RoundRobin) Stop() {
	sf.upstreams.Stop()
}

// HasActive implement Selector
func (sf *RoundRobin) HasActive() bool {
	return sf.upstreams.HasActive()
}

// ActiveCount implement Selector
func (sf *RoundRobin) ActiveCount() int {
	return sf.upstreams.ActiveCount()
}
