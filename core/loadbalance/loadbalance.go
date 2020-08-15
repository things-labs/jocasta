package loadbalance

import (
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/thinkgos/jocasta/core/idns"
	"github.com/thinkgos/jocasta/lib/gpool"
	"github.com/thinkgos/jocasta/lib/logger"
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

type Group struct {
	method   string
	Interval time.Duration
	debug    bool
	last     *Upstream
	dns      *idns.Resolver
	goPool   gpool.Pool
	log      logger.Logger

	mu        sync.Mutex
	closeChan chan struct{}
	upstreams UpstreamPool
	selector  Selector
}

// if method not supprt,it will use roundrobin method.
// support method:
// 		roundrobin
// 		leastconn
// 		hash
// 		leasttime
// 		weight
func NewGroup(method string, configs []Config, opts ...Option) *Group {
	g := &Group{
		selector:  getNewSelectorFunction(method)(),
		upstreams: NewUpstreamPool(configs),
		log:       logger.NewDiscard(),
		closeChan: make(chan struct{}),
	}

	for _, opt := range opts {
		opt(g)
	}
	go g.activeHealthChecker()
	return g
}

func (sf *Group) Select(srcAddr string, onlyHa bool) (addr string) {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	addr = ""

	if len(sf.upstreams) == 1 {
		return sf.upstreams[0].Addr
	}
	if onlyHa {
		if sf.last != nil && (sf.last.Healthy() || sf.last.LeastTime() == 0) {
			if sf.debug {
				sf.log.Infof("############ choosed %s from lastest ############", sf.last.Addr)
				printDebug(true, sf.log, nil, srcAddr, sf.upstreams)
			}
			return sf.last.Addr
		}
		sf.last = sf.selector.Select(sf.upstreams, srcAddr)
		if !sf.last.Healthy() && sf.last.LeastTime() > 0 {
			sf.log.Infof("###warn### lb selected empty , return default , for : %s", srcAddr)
		}
		return sf.last.Addr
	}
	b := sf.selector.Select(sf.upstreams, srcAddr)
	return b.Addr
}

func (sf *Group) ConnsIncrease(addr string) {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	sf.upstreams.ConnsIncrease(addr)
}

func (sf *Group) ConnsDecrease(addr string) {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	sf.upstreams.ConnsDecrease(addr)
}

func (sf *Group) Close() error {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	select {
	case <-sf.closeChan:
	default:
		close(sf.closeChan)
	}
	return nil
}

func (sf *Group) HasHealthy() bool {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	return sf.upstreams.HasHealthy()
}

func (sf *Group) HealthyCount() int {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	return sf.upstreams.HealthyCount()
}

func (sf *Group) Reset(addrs []string) {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	bks := sf.upstreams
	if len(bks) == 0 {
		return
	}
	cfg := bks[0].Config
	configs := make([]Config, 0, len(addrs))
	for _, addr := range addrs {
		c := cfg
		c.Addr = addr
		configs = append(configs, c)
	}
	// create new
	sf.upstreams = NewUpstreamPool(configs)
	sf.selector = getNewSelectorFunction(sf.method)()
}

func (sf *Group) resolve(addr string) string {
	if sf.dns != nil && sf.dns.PublicDNSAddr() != "" {
		addr = sf.dns.MustResolve(addr)
	}
	return addr
}

func (sf *Group) activeHealthChecker() {
	defer func() {
		if err := recover(); err != nil {
			sf.log.DPanicf("active health checks: %v\n%s", err, debug.Stack())
		}
	}()
	ticker := time.NewTicker(sf.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
		case <-sf.closeChan:
			return
		}
		for _, upstream := range sf.upstreams {
			ups := upstream
			gpool.Go(sf.goPool, func() { ups.healthyCheck(sf.resolve(ups.Addr)) })
		}
	}
}

func printDebug(isDebug bool, log logger.Logger, selected *Upstream, srcAddr string, backends []*Upstream) {
	if isDebug {
		log.Debugf("############ LB start ############\n")
		if selected != nil {
			log.Debugf("choosed %s for %s\n", selected.Addr, srcAddr)
		}
		for _, v := range backends {
			log.Debugf("addr:%s,conns:%d,time:%d,weight:%d,health:%v\n", v.Addr, v.ConnsCount(), v.LeastTime(), v.Weight, v.Healthy())
		}
		log.Debugf("############ LB end ############\n")
	}
}
