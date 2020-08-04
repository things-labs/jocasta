package lb

import (
	"crypto/md5"
	"net"
	"sync"

	"github.com/thinkgos/jocasta/core/idns"
	"github.com/thinkgos/jocasta/lib/logger"
)

var _ Selector = (*Hash)(nil)

type Hash struct {
	sync.Mutex
	backends []*Backend
	debug    bool
	log      logger.Logger
}

func NewHash(backends []*Backend, log logger.Logger, debug bool) *Hash {
	return &Hash{
		backends: backends,
		log:      log,
		debug:    debug,
	}
}

func (sf *Hash) Select(srcAddr string) (addr string) {
	return sf.SelectBackend(srcAddr).Address
}

func (sf *Hash) SelectBackend(srcAddr string) (b *Backend) {
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

	i := 0
	host, _, err := net.SplitHostPort(srcAddr)
	if err != nil {
		host = srcAddr
	}

	for _, b := range md5.Sum([]byte(host)) {
		i += int(b)
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
RETRY:
	k := i % len(sf.backends)
	if !sf.backends[k].Active() {
		i++
		goto RETRY
	}
	return sf.backends[k]
}

func (sf *Hash) IncreaseConns(string) {}

func (sf *Hash) DecreaseConns(string) {}

func (sf *Hash) Stop() {
	sf.Lock()
	defer sf.Unlock()
	sf.stop()
}

func (sf *Hash) IsActive() bool {
	sf.Lock()
	defer sf.Unlock()
	for _, b := range sf.backends {
		if b.Active() {
			return true
		}
	}
	return false
}

func (sf *Hash) ActiveCount() (count int) {
	sf.Lock()
	defer sf.Unlock()
	for _, b := range sf.backends {
		if b.Active() {
			count++
		}
	}
	return
}

func (sf *Hash) Reset(configs []Config, dr *idns.Resolver, log logger.Logger) {
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

func (sf *Hash) Backends() []*Backend {
	return sf.backends
}

func (sf *Hash) stop() {
	for _, b := range sf.backends {
		b.StopHeartCheck()
	}
}
