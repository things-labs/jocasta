package lb

import (
	"fmt"
	"strings"
	"sync"

	"github.com/thinkgos/jocasta/core/idns"
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

// IsSupport return the method support or not
func IsSupport(method string) bool {
	selectorMux.Lock()
	defer selectorMux.Unlock()
	_, ok := selectors[strings.ToLower(method)]
	return ok
}

type Group struct {
	method string
	dns    *idns.Resolver
	last   *Upstream
	debug  bool
	log    logger.Logger

	mu        sync.Mutex
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
func NewGroup(method string, configs []Config, dns *idns.Resolver, log logger.Logger, debug bool) *Group {
	return &Group{
		selector:  getNewSelectorFunction(method)(),
		upstreams: NewUpstreamPool(configs, dns, log),
		dns:       dns,
		debug:     debug,
		log:       log,
	}
}

func (sf *Group) Select(srcAddr string, onlyHa bool) (addr string) {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	addr = ""

	if len(sf.upstreams) == 1 {
		return sf.upstreams[0].Address
	}
	if onlyHa {
		if sf.last != nil && (sf.last.Active() || sf.last.ConnectUsedTime() == 0) {
			if sf.debug {
				sf.log.Infof("############ choosed %s from lastest ############", sf.last.Address)
				printDebug(true, sf.log, nil, srcAddr, sf.upstreams)
			}
			return sf.last.Address
		}
		sf.last = sf.selector.Select(sf.upstreams, srcAddr)
		if !sf.last.Active() && sf.last.ConnectUsedTime() > 0 {
			sf.log.Infof("###warn### lb selected empty , return default , for : %s", srcAddr)
		}
		return sf.last.Address
	}
	b := sf.selector.Select(sf.upstreams, srcAddr)
	return b.Address

}

func (sf *Group) IncreaseConns(addr string) {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	sf.upstreams.ConnsIncrease(addr)
}

func (sf *Group) DecreaseConns(addr string) {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	sf.upstreams.ConnsDecrease(addr)
}

func (sf *Group) Stop() {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	if sf.selector != nil {
		sf.upstreams.Stop()
	}
}

func (sf *Group) IsActive() bool {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	return sf.upstreams.HasActive()
}

func (sf *Group) ActiveCount() int {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	return sf.upstreams.ActiveCount()
}

func (sf *Group) Reset(addrs []string) {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	bks := sf.upstreams.Backends()
	if len(bks) == 0 {
		return
	}
	cfg := bks[0].Config
	configs := make([]Config, 0, len(addrs))
	for _, addr := range addrs {
		c := cfg
		c.Address = addr
		configs = append(configs, c)
	}
	// stop all old backends
	bks.Stop()
	// create new
	sf.upstreams = NewUpstreamPool(configs, sf.dns, sf.log)
	sf.selector = getNewSelectorFunction(sf.method)()
}

func printDebug(isDebug bool, log logger.Logger, selected *Upstream, srcAddr string, backends []*Upstream) {
	if isDebug {
		log.Debugf("############ LB start ############\n")
		if selected != nil {
			log.Debugf("choosed %s for %s\n", selected.Address, srcAddr)
		}
		for _, v := range backends {
			log.Debugf("addr:%s,conns:%d,time:%d,weight:%d,active:%v\n", v.Address, v.ConnsCount(), v.ConnectUsedTime(), v.Weight, v.Active())
		}
		log.Debugf("############ LB end ############\n")
	}
}
