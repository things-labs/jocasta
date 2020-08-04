package lb

import (
	"sync"

	"github.com/thinkgos/jocasta/core/idns"
	"github.com/thinkgos/jocasta/lib/logger"
)

var _ Selector = (*RoundRobin)(nil)

type RoundRobin struct {
	sync.Mutex
	backendIndex int
	backends     []*Backend
	log          logger.Logger
	debug        bool
}

// NewRoundRobin 轮询模式
func NewRoundRobin(backends []*Backend, log logger.Logger, debug bool) *RoundRobin {
	return &RoundRobin{
		backends: backends,
		log:      log,
		debug:    debug,
	}

}
func (sf *RoundRobin) Select(srcAddr string) (addr string) {
	return sf.SelectBackend(srcAddr).Address
}
func (sf *RoundRobin) SelectBackend(srcAddr string) (b *Backend) {
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
RETRY:
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
	sf.backendIndex++
	if sf.backendIndex > len(sf.backends)-1 {
		sf.backendIndex = 0
	}
	if !sf.backends[sf.backendIndex].Active() {
		goto RETRY
	}
	return sf.backends[sf.backendIndex]
}
func (sf *RoundRobin) IncreaseConns(string) {

}
func (sf *RoundRobin) DecreaseConns(string) {}

func (sf *RoundRobin) Stop() {}

func (sf *RoundRobin) IsActive() bool {
	sf.Lock()
	defer sf.Unlock()
	for _, b := range sf.backends {
		if b.Active() {
			return true
		}
	}
	return false
}
func (sf *RoundRobin) ActiveCount() (count int) {
	sf.Lock()
	defer sf.Unlock()
	for _, b := range sf.backends {
		if b.Active() {
			count++
		}
	}
	return
}
func (sf *RoundRobin) Reset(configs []Config, dr *idns.Resolver, log logger.Logger) {
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

func (sf *RoundRobin) Backends() []*Backend {
	return sf.backends
}

func (sf *RoundRobin) stop() {
	for _, b := range sf.backends {
		b.StopHeartCheck()
	}
}
