package lb

import (
	"sync"

	"github.com/thinkgos/jocasta/core/idns"
	"github.com/thinkgos/jocasta/lib/logger"
)

var _ Selector = (*LeastTime)(nil)

type LeastTime struct {
	sync.Mutex
	backends []*Backend
	log      logger.Logger
	debug    bool
}

// NewLeastTime 使用连接时间最小的
func NewLeastTime(backends []*Backend, log logger.Logger, debug bool) *LeastTime {
	return &LeastTime{
		backends: backends,
		log:      log,
		debug:    debug,
	}
}

func (sf *LeastTime) Select(srcAddr string) (addr string) {
	return sf.SelectBackend(srcAddr).Address
}

func (sf *LeastTime) SelectBackend(srcAddr string) (b *Backend) {
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
	min := sf.backends[0].ConnectUsedTime()
	index := 0
	for i, b := range sf.backends {
		if b.Active() {
			min = b.ConnectUsedTime()
			index = i
			break
		}
	}
	for i, b := range sf.backends {
		if b.Active() && b.ConnectUsedTime() > 0 && b.ConnectUsedTime() <= min {
			min = b.ConnectUsedTime()
			index = i
		}
	}
	return sf.backends[index]
}

func (sf *LeastTime) IncreaseConns(string) {}

func (sf *LeastTime) DecreaseConns(string) {}

func (sf *LeastTime) Stop() {
	sf.Lock()
	defer sf.Unlock()
	sf.stop()
}

func (sf *LeastTime) IsActive() bool {
	sf.Lock()
	defer sf.Unlock()
	for _, b := range sf.backends {
		if b.Active() {
			return true
		}
	}
	return false
}

func (sf *LeastTime) ActiveCount() (count int) {
	sf.Lock()
	defer sf.Unlock()
	for _, b := range sf.backends {
		if b.Active() {
			count++
		}
	}
	return
}

func (sf *LeastTime) Reset(configs []Config, dr *idns.Resolver, log logger.Logger) {
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

func (sf *LeastTime) Backends() []*Backend {
	return sf.backends
}

func (sf *LeastTime) stop() {
	for _, b := range sf.backends {
		b.StopHeartCheck()
	}
}
