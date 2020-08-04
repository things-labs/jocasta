package lb

import (
	"sync"

	"github.com/thinkgos/jocasta/core/idns"
	"github.com/thinkgos/jocasta/lib/logger"
)

var _ Selector = (*Weight)(nil)

type Weight struct {
	sync.Mutex
	backends []*Backend
	log      logger.Logger
	debug    bool
}

// NewWeight 根据上级的权重和连接数,选一个上级
func NewWeight(backends []*Backend, log logger.Logger, debug bool) *Weight {
	return &Weight{
		backends: backends,
		log:      log,
		debug:    debug,
	}
}

func (sf *Weight) Select(srcAddr string) (addr string) {
	return sf.SelectBackend(srcAddr).Address
}

func (sf *Weight) SelectBackend(srcAddr string) (b *Backend) {
	sf.Lock()
	defer sf.Unlock()
	defer func() {
		printDebug(sf.debug, sf.log, b, srcAddr, sf.backends)
	}()
	if len(sf.backends) == 0 {
		return
	}
	if len(sf.backends) == 1 {
		return sf.backends[0]
	}

	found := false
	for _, b := range sf.backends {
		if b.Active() {
			found = true
			break
		}
	}
	if !found {
		return sf.backends[0]
	}

	min := sf.backends[0].Connections() / int64(sf.backends[0].Weight)
	index := 0
	for i, b := range sf.backends {
		if b.Active() {
			min = b.Connections() / int64(b.Weight)
			index = i
			break
		}
	}
	for i, b := range sf.backends {
		if b.Active() && b.Connections()/int64(b.Weight) <= min {
			min = b.Connections()
			index = i
		}
	}
	return sf.backends[index]
}

func (sf *Weight) IncreaseConns(addr string) {
	sf.Lock()
	defer sf.Unlock()
	for _, a := range sf.backends {
		if a.Address == addr {
			a.IncreaseConns()
			return
		}
	}
}

func (sf *Weight) DecreaseConns(addr string) {
	sf.Lock()
	defer sf.Unlock()
	for _, a := range sf.backends {
		if a.Address == addr {
			a.DecreaseConns()
			return
		}
	}
}
func (sf *Weight) Stop() {
	sf.Lock()
	defer sf.Unlock()
	sf.stop()
}

func (sf *Weight) IsActive() bool {
	sf.Lock()
	defer sf.Unlock()
	for _, b := range sf.backends {
		if b.Active() {
			return true
		}
	}
	return false
}
func (sf *Weight) ActiveCount() (count int) {
	sf.Lock()
	defer sf.Unlock()
	for _, b := range sf.backends {
		if b.Active() {
			count++
		}
	}
	return
}

func (sf *Weight) Reset(configs []Config, dr *idns.Resolver, log logger.Logger) {
	sf.Lock()
	defer sf.Unlock()
	sf.stop()
	bks := make([]*Backend, 0)
	for _, c := range configs {
		b, err := New(c, dr, log)
		if err != nil {
			continue
		}
		b.StartHeartCheck()
		bks = append(bks, b)
	}
	sf.backends = bks
}

func (sf *Weight) Backends() []*Backend {
	return sf.backends
}

func (sf *Weight) stop() {
	for _, b := range sf.backends {
		b.StopHeartCheck()
	}
}
