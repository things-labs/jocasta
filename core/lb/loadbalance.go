package lb

import (
	"strings"
	"sync"

	"github.com/thinkgos/jocasta/core/idns"
	"github.com/thinkgos/jocasta/lib/logger"
)

// 负载均衡工作模式
type Mode byte

// 定义工作模式
const (
	ModeRoundRobin Mode = iota // 轮询
	ModeLeastConn              // 最小连接
	ModeHash                   // 根据客户端地址计算出一个固定上级
	ModeWeight                 // 根据权重和连接数,选一个上级
	ModeLeastTime              // 使用连接时间最小的
)

func Method(method string) Mode {
	modes := map[string]Mode{
		"weight":     ModeWeight,
		"leasttime":  ModeLeastTime,
		"leastconn":  ModeLeastConn,
		"hash":       ModeHash,
		"roundrobin": ModeRoundRobin,
	}
	return modes[strings.ToLower(method)]
}

// Selector 后端选择接口
type Selector interface {
	// 根据源地址,获得后端连接地址
	Select(srcAddr string) (addr string)
	// 根据源地址,获得后端连接
	SelectBackend(srcAddr string) *Backend
	// 增加一个连接
	IncreaseConns(addr string)
	// 减少一个连接
	DecreaseConns(addr string)
	// 是否有活动的后端
	HasActive() bool
	// 活动的后端数量
	ActiveCount() int
	// 停止所有后端
	Stop()
}

type Group struct {
	dns   *idns.Resolver
	last  *Backend
	debug bool
	log   logger.Logger

	mu        sync.Mutex
	selector  Selector
	upstreams Upstreams

	newSelector func(backends []*Backend) Selector
}

func NewGroup(selectType Mode, configs []Config, dns *idns.Resolver, log logger.Logger, debug bool) *Group {
	bks := NewUpstreams(configs, dns, log)

	var newSelector func(backends []*Backend) Selector

	switch selectType {
	case ModeRoundRobin:
		newSelector = NewRoundRobin
	case ModeLeastConn:
		newSelector = NewLeastConn
	case ModeHash:
		newSelector = NewHash
	case ModeWeight:
		newSelector = NewWeight
	case ModeLeastTime:
		newSelector = NewLeastTime
	}
	return &Group{
		selector:  newSelector(bks),
		dns:       dns,
		upstreams: bks,
		debug:     debug,
		log:       log,
	}
}

func (g *Group) Select(srcAddr string, onlyHa bool) (addr string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	addr = ""
	if len(g.upstreams) == 1 {
		return g.upstreams[0].Address
	}
	if onlyHa {

		if g.last != nil && (g.last.Active() || g.last.ConnectUsedTime() == 0) {
			if g.debug {
				g.log.Infof("############ choosed %s from lastest ############", g.last.Address)
				printDebug(true, g.log, nil, srcAddr, g.upstreams)
			}
			return g.last.Address
		}
		g.last = g.selector.SelectBackend(srcAddr)
		if !g.last.Active() && g.last.ConnectUsedTime() > 0 {
			g.log.Infof("###warn### lb selected empty , return default , for : %s", srcAddr)
		}
		return g.last.Address
	}
	b := g.selector.SelectBackend(srcAddr)
	return b.Address

}

func (g *Group) IncreaseConns(addr string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.selector.IncreaseConns(addr)
}

func (g *Group) DecreaseConns(addr string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.selector.DecreaseConns(addr)
}

func (g *Group) Stop() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.selector != nil {
		g.selector.Stop()
	}
}

func (g *Group) IsActive() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.selector.HasActive()
}

func (g *Group) ActiveCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.selector.ActiveCount()
}

func (g *Group) Reset(addrs []string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	bks := g.upstreams
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
	g.selector.Stop()

	// create new
	bks = NewUpstreams(configs, g.dns, g.log)
	g.upstreams = bks
	g.selector = g.newSelector(bks)
}

func printDebug(isDebug bool, log logger.Logger, selected *Backend, srcAddr string, backends []*Backend) {
	if isDebug {
		log.Debugf("############ LB start ############\n")
		if selected != nil {
			log.Debugf("choosed %s for %s\n", selected.Address, srcAddr)
		}
		for _, v := range backends {
			log.Debugf("addr:%s,conns:%d,time:%d,weight:%d,active:%v\n", v.Address, v.Connections(), v.ConnectUsedTime(), v.Weight, v.Active())
		}
		log.Debugf("############ LB end ############\n")
	}
}
