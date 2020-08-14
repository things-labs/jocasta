package lb

var _ Selector = (*LeastTime)(nil)

type LeastTime struct {
	upstreams Upstreams
}

// NewLeastTime 使用连接时间最小的
func NewLeastTime(backends []*Backend) Selector {
	return &LeastTime{upstreams: backends}
}

// Select implement Selector
func (sf *LeastTime) Select(srcAddr string) (addr string) {
	return sf.SelectBackend(srcAddr).Address
}

// SelectBackend implement Selector
func (sf *LeastTime) SelectBackend(srcAddr string) (b *Backend) {
	if len(sf.upstreams) == 0 {
		return
	}
	if len(sf.upstreams) == 1 {
		return sf.upstreams[0]
	}

	if found := sf.upstreams.HasActive(); !found {
		return sf.upstreams[0]
	}

	min := sf.upstreams[0].ConnectUsedTime()
	index := 0
	for i, b := range sf.upstreams {
		if b.Active() {
			min = b.ConnectUsedTime()
			index = i
			break
		}
	}
	for i, b := range sf.upstreams {
		if b.Active() && b.ConnectUsedTime() > 0 && b.ConnectUsedTime() <= min {
			min = b.ConnectUsedTime()
			index = i
		}
	}
	return sf.upstreams[index]
}

// IncreaseConns implement Selector
func (sf *LeastTime) IncreaseConns(string) {}

// DecreaseConns implement Selector
func (sf *LeastTime) DecreaseConns(string) {}

func (sf *LeastTime) Stop() {
	sf.upstreams.Stop()
}

func (sf *LeastTime) HasActive() bool {
	return sf.upstreams.HasActive()
}

func (sf *LeastTime) ActiveCount() int {
	return sf.upstreams.ActiveCount()
}
