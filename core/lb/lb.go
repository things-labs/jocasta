package lb

import (
	"sync"

	"github.com/thinkgos/jocasta/core/idns"
	"github.com/thinkgos/jocasta/lib/logger"
)

const (
	SELECT_ROUNDROBIN = iota // 轮询
	SELECT_LEASTCONN         // 最小连接
	SELECT_HASH              // 根据客户端地址计算出一个固定上级
	SELECT_WEITHT            // 根据权重和连接数,选一个上级
	SELECT_LEASTTIME         // 使用连地间最小的
)

// Selector 选择器
type Selector interface {
	// 根据源地址,获得上级(后端)连接地址
	Select(srcAddr string) (addr string)
	// 根据源地址,获得上级
	SelectBackend(srcAddr string) (b *Backend)
	// 给上级增加一个连接
	IncreaseConns(addr string)
	// 给上级减少一个连接
	DecreaseConns(addr string)
	// 停止所有后端
	Stop()
	// 重置到新的配置
	Reset(configs []Config, dr *idns.Resolver, log logger.Logger)
	// 是否有活动的上级
	IsActive() bool
	// 活动的上级数量
	ActiveCount() int
	// 获取所有上级
	Backends() []*Backend
}

type Group struct {
	selector Selector
	dns      *idns.Resolver
	mu       sync.Mutex
	last     *Backend
	backends []*Backend
	debug    bool
	log      logger.Logger
}

func NewGroup(selectType int, configs []Config, dns *idns.Resolver, log logger.Logger, debug bool) *Group {
	bks := []*Backend{}
	for _, c := range configs {
		b, err := New(c, dns, log)
		if err != nil {
			continue
		}
		bks = append(bks, b)
		b.StartHeartCheck()
	}

	var selector Selector

	switch selectType {
	case SELECT_ROUNDROBIN:
		selector = NewRoundRobin(bks, log, debug)
	case SELECT_LEASTCONN:
		selector = NewLeastConn(bks, log, debug)
	case SELECT_HASH:
		selector = NewHash(bks, log, debug)
	case SELECT_WEITHT:
		selector = NewWeight(bks, log, debug)
	case SELECT_LEASTTIME:
		selector = NewLeastTime(bks, log, debug)
	}
	return &Group{
		selector: selector,
		dns:      dns,
		backends: bks,
		debug:    debug,
		log:      log,
	}
}

func (g *Group) Select(srcAddr string, onlyHa bool) (addr string) {
	addr = ""
	if len(g.backends) == 1 {
		return g.backends[0].Address
	}
	if onlyHa {
		g.mu.Lock()
		defer g.mu.Unlock()
		if g.last != nil && (g.last.Active() || g.last.ConnectUsedTime() == 0) {
			if g.debug {
				g.log.Infof("############ choosed %s from lastest ############", g.last.Address)
				printDebug(true, g.log, nil, srcAddr, g.selector.Backends())
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
	g.selector.IncreaseConns(addr)
}

func (g *Group) DecreaseConns(addr string) {
	g.selector.DecreaseConns(addr)
}

func (g *Group) Stop() {
	if g.selector != nil {
		g.selector.Stop()
	}
}

func (g *Group) IsActive() bool {
	return g.selector.IsActive()
}

func (g *Group) ActiveCount() (count int) {
	return g.selector.ActiveCount()
}

func (g *Group) Reset(addrs []string) {
	bks := g.selector.Backends()
	if len(bks) == 0 {
		return
	}
	cfg := bks[0].Config
	configs := []Config{}
	for _, addr := range addrs {
		c := cfg
		c.Address = addr
		configs = append(configs, c)
	}
	g.selector.Reset(configs, g.dns, g.log)
	g.backends = g.selector.Backends()
}

func (g *Group) Backends() []*Backend {
	return g.selector.Backends()
}

func Method(method string) int {
	types := map[string]int{
		"weight":     SELECT_WEITHT,
		"leasttime":  SELECT_LEASTTIME,
		"leastconn":  SELECT_LEASTCONN,
		"hash":       SELECT_HASH,
		"roundrobin": SELECT_ROUNDROBIN,
	}
	return types[method]
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
