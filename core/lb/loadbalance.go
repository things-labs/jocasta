package lb

import (
	"strings"
	"sync"

	"github.com/thinkgos/jocasta/core/idns"
	"github.com/thinkgos/jocasta/lib/logger"
)

var selectorFactories = map[string]func(upstreams Upstreams) Selector{
	"roundrobin": NewRoundRobin,
	"leastconn":  NewLeastConn,
	"hash":       NewHash,
	"leasttime":  NewLeastTime,
	"weight":     NewWeight,
}

// IsSupport return the method support or not
func IsSupport(method string) bool {
	_, ok := selectorFactories[strings.ToLower(method)]
	return ok
}

// getSelectorFactory return selector factory function
func getSelectorFactory(method string) func(Upstreams) Selector {
	newSelector, ok := selectorFactories[strings.ToLower(method)]
	if !ok {
		newSelector = NewRoundRobin
	}
	return newSelector
}

type Group struct {
	method string
	dns    *idns.Resolver
	last   *Backend
	debug  bool
	log    logger.Logger

	mu       sync.Mutex
	selector Selector
}

// if method not supprt,it will use roundrobin method.
// support method:
// 		roundrobin
// 		leastconn
// 		hash
// 		leasttime
// 		weight
func NewGroup(method string, configs []Config, dns *idns.Resolver, log logger.Logger, debug bool) *Group {
	newSelector := getSelectorFactory(method)
	return &Group{
		selector: newSelector(NewUpstreams(configs, dns, log)),
		dns:      dns,
		debug:    debug,
		log:      log,
	}
}

func (sf *Group) Select(srcAddr string, onlyHa bool) (addr string) {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	addr = ""
	streams := sf.selector.Backends()
	if len(streams) == 1 {
		return streams[0].Address
	}
	if onlyHa {
		if sf.last != nil && (sf.last.Active() || sf.last.ConnectUsedTime() == 0) {
			if sf.debug {
				sf.log.Infof("############ choosed %s from lastest ############", sf.last.Address)
				printDebug(true, sf.log, nil, srcAddr, streams)
			}
			return sf.last.Address
		}
		sf.last = sf.selector.SelectBackend(srcAddr)
		if !sf.last.Active() && sf.last.ConnectUsedTime() > 0 {
			sf.log.Infof("###warn### lb selected empty , return default , for : %s", srcAddr)
		}
		return sf.last.Address
	}
	b := sf.selector.SelectBackend(srcAddr)
	return b.Address

}

func (sf *Group) IncreaseConns(addr string) {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	sf.selector.ConnsIncrease(addr)
}

func (sf *Group) DecreaseConns(addr string) {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	sf.selector.ConnsDecrease(addr)
}

func (sf *Group) Stop() {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	if sf.selector != nil {
		sf.selector.Stop()
	}
}

func (sf *Group) IsActive() bool {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	return sf.selector.HasActive()
}

func (sf *Group) ActiveCount() int {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	return sf.selector.ActiveCount()
}

func (sf *Group) Reset(addrs []string) {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	bks := sf.selector.Backends()
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
	newSelector := getSelectorFactory(sf.method)
	sf.selector = newSelector(NewUpstreams(configs, sf.dns, sf.log))
}

func printDebug(isDebug bool, log logger.Logger, selected *Backend, srcAddr string, backends []*Backend) {
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
