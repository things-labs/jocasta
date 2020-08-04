package lb

import (
	"sync"

	"github.com/thinkgos/jocasta/core/idns"
	"github.com/thinkgos/jocasta/lib/logger"
)

var _ Selector = (*LeastConn)(nil)

type LeastConn struct {
	sync.Mutex
	backends []*Backend
	log      logger.Logger
	debug    bool
}

// NewLeastConn 合用最小连接数的
func NewLeastConn(backends []*Backend, log logger.Logger, debug bool) *LeastConn {
	lc := LeastConn{
		backends: backends,
		log:      log,
		debug:    debug,
	}
	return &lc
}

func (sf *LeastConn) Select(srcAddr string) (addr string) {
	return sf.SelectBackend(srcAddr).Address
}

func (sf *LeastConn) SelectBackend(srcAddr string) (b *Backend) {
	sf.Lock()
	defer func() {
		printDebug(sf.debug, sf.log, b, srcAddr, sf.backends)
		sf.Unlock()
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
	min := sf.backends[0].Connections()
	index := 0
	for i, b := range sf.backends {
		if b.Active() {
			min = b.Connections()
			index = i
			break
		}
	}
	for i, b := range sf.backends {
		if b.Active() && b.Connections() <= min {
			min = b.Connections()
			index = i
		}
	}
	return sf.backends[index]
}

func (sf *LeastConn) IncreaseConns(addr string) {
	sf.Lock()
	defer sf.Unlock()
	for _, a := range sf.backends {
		if a.Address == addr {
			a.IncreaseConns()
			return
		}
	}
}

func (sf *LeastConn) DecreaseConns(addr string) {
	sf.Lock()
	defer sf.Unlock()
	for _, a := range sf.backends {
		if a.Address == addr {
			a.DecreaseConns()
			return
		}
	}
}

func (sf *LeastConn) Stop() {
	sf.Lock()
	defer sf.Unlock()
	sf.stop()
}

func (sf *LeastConn) IsActive() bool {
	sf.Lock()
	defer sf.Unlock()
	for _, b := range sf.backends {
		if b.Active() {
			return true
		}
	}
	return false
}

func (sf *LeastConn) ActiveCount() (count int) {
	sf.Lock()
	defer sf.Unlock()
	for _, b := range sf.backends {
		if b.Active() {
			count++
		}
	}
	return
}

func (sf *LeastConn) Reset(configs []Config, dr *idns.Resolver, log logger.Logger) {
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

func (sf *LeastConn) Backends() []*Backend {
	return sf.backends
}

func (sf *LeastConn) stop() {
	for _, b := range sf.backends {
		b.StopHeartCheck()
	}
}
