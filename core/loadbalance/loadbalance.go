package loadbalance

import (
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/thinkgos/jocasta/core/idns"
	"github.com/thinkgos/jocasta/pkg/gopool"
	"github.com/thinkgos/jocasta/pkg/logger"
)

// Selector select the upstream interface
type Selector interface {
	Select(upstreams UpstreamPool, srcAddr string) *Upstream
}

// for selector
var (
	selectorMux sync.Mutex
	selectors   = make(map[string]func() Selector)
)

// RegisterSelector register selector method
func RegisterSelector(method string, newSelector func() Selector) {
	if newSelector == nil {
		panic("missing new selector function")
	}
	if newSelector() == nil {
		panic("new selector function must return a non-nil selector instance")
	}

	selectorMux.Lock()
	defer selectorMux.Unlock()
	if _, ok := selectors[method]; ok {
		panic(fmt.Sprintf("method already registered: %s", method))
	}
	selectors[method] = newSelector
}

// getSelectorFactory return selector factory function, if method not found, it will use roundrobin.
func getNewSelectorFunction(method string) func() Selector {
	selectorMux.Lock()
	defer selectorMux.Unlock()
	newSelector, ok := selectors[strings.ToLower(method)]
	if !ok {
		newSelector = selectors["roundrobin"]
	}
	return newSelector
}

// HasSupportMethod return the method support or not
func HasSupportMethod(method string) bool {
	selectorMux.Lock()
	defer selectorMux.Unlock()
	_, ok := selectors[strings.ToLower(method)]
	return ok
}

// Methods get a copy support method
func Methods() []string {
	selectorMux.Lock()
	defer selectorMux.Unlock()
	method := make([]string, 0, len(selectors))
	for k := range selectors {
		method = append(method, k)
	}
	return method
}

// Balanced load balance
type Balanced struct {
	method   string
	interval time.Duration
	debug    bool
	dns      *idns.Resolver
	goPool   gopool.Pool
	log      logger.Logger

	rw        sync.RWMutex
	closeChan chan struct{}
	upstreams UpstreamPool
	selector  Selector
}

// New new a load balance with method and upstream config
// if method not supprt,it will use roundrobin method.
// support method:
//      random
// 		roundrobin
// 		leastconn
// 		hash
//      addrhash
// 		leasttime
// 		weight
func New(method string, configs []Config, opts ...Option) *Balanced {
	lb := &Balanced{
		method:    method,
		interval:  time.Second * 30,
		selector:  getNewSelectorFunction(method)(),
		upstreams: NewUpstreamPool(configs),
		log:       logger.NewDiscard(),
		closeChan: make(chan struct{}),
	}
	for _, opt := range opts {
		opt(lb)
	}
	if lb.interval > 0 {
		go lb.activeHealthChecker()
	}
	return lb
}

// Select select the upstream with srcAddr and ha mode then return the upstream addr
func (sf *Balanced) Select(srcAddr string) string {
	sf.rw.RLock()
	defer sf.rw.RUnlock()

	b := sf.selector.Select(sf.upstreams, srcAddr)
	if b == nil {
		return ""
	}
	if sf.debug {
		sf.log.Infof("#########--> choose %s <--#########", b.Addr)
		sf.log.Debugf("############ Load Balance start ############")
		for _, ups := range sf.upstreams {
			sf.log.Debugf("addr: %s,conns: %d,time: %d,weight: %d,health: %v\n",
				ups.Addr, ups.ConnsCount(), ups.LeastTime(), ups.Weight, ups.Healthy())
		}
		sf.log.Debugf("############ Load Balance end ############")
	}
	return b.Addr
}

// ConnsIncrease increase the addr conns count
func (sf *Balanced) ConnsIncrease(addr string) {
	sf.rw.Lock()
	defer sf.rw.Unlock()
	sf.upstreams.ConnsIncrease(addr)
}

// ConnsDecrease decrease the addr conns count
func (sf *Balanced) ConnsDecrease(addr string) {
	sf.rw.Lock()
	defer sf.rw.Unlock()
	sf.upstreams.ConnsDecrease(addr)
}

// Close close the balanced
func (sf *Balanced) Close() error {
	sf.rw.Lock()
	defer sf.rw.Unlock()
	select {
	case <-sf.closeChan:
	default:
		close(sf.closeChan)
	}
	return nil
}

// HasHealthy has any health upstream
func (sf *Balanced) HasHealthy() bool {
	sf.rw.RLock()
	defer sf.rw.RUnlock()
	return sf.upstreams.HasHealthy()
}

// HealthyCount health backend count
func (sf *Balanced) HealthyCount() int {
	sf.rw.RLock()
	defer sf.rw.RUnlock()
	return sf.upstreams.HealthyCount()
}

// Reset reset to the new upstream
func (sf *Balanced) Reset(configs []Config) {
	sf.rw.Lock()
	defer sf.rw.Unlock()
	sf.upstreams = NewUpstreamPool(configs)
	sf.selector = getNewSelectorFunction(sf.method)()
}

// resolve resolve the addr to ip:port
func (sf *Balanced) resolve(addr string) string {
	if sf.dns != nil && sf.dns.PublicDNSAddr() != "" {
		addr = sf.dns.MustResolve(addr)
	}
	return addr
}

// activeHealthChecker healthy checker
// it must be run in a goroutine
func (sf *Balanced) activeHealthChecker() {
	ticker := time.NewTicker(sf.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
		case <-sf.closeChan:
			return
		}
		sf.rw.Lock()
		for _, upstream := range sf.upstreams {
			ups := upstream
			gopool.Go(sf.goPool, func() {
				defer func() {
					if err := recover(); err != nil {
						sf.log.DPanicf("active health checks: %v\n%s", err, debug.Stack())
					}
				}()
				ups.healthyCheck(sf.resolve(ups.Addr))
			})
		}
		sf.rw.Unlock()
	}
}
