// Package filter 过滤器, proxy代理表,direct直连表,
// 如果域名在代理表和直连表都存在,只有代理表起作用
package filter

import (
	"context"
	"io/ioutil"
	"net"
	"os"
	"runtime/debug"
	"strings"
	"time"

	cmap "github.com/orcaman/concurrent-map"

	"github.com/thinkgos/jocasta/lib/gpool"
	"github.com/thinkgos/jocasta/lib/logger"
)

// 默认存活探测成功,失败阀值,3次
const defaultThreshold = 3
const defaultAliveThreshold = 30 * 60 // 30分钟(unix时间),单位秒

// Filter 过滤器
type Filter struct {
	//  过滤模式
	// direct: 直连,不走代理,
	// proxy:  走代理
	// intelligent <default> 智能选择
	intelligent      string
	cache            cmap.ConcurrentMap                                                  // cache表是动态添加的,周期检查连通性,如果不通,将走代理
	proxies          cmap.ConcurrentMap                                                  // 代理表
	directs          cmap.ConcurrentMap                                                  // 直连表
	timeout          time.Duration                                                       // 域名检测超时时间, default: 1s
	livenessPeriod   time.Duration                                                       // 域名存活控测周期, default: 30s
	livenessProbe    func(ctx context.Context, addr string, timeout time.Duration) error // 域名探针接口, default: tcp dial
	successThreshold uint
	failureThreshold uint
	aliveThreshold   int64
	cancel           context.CancelFunc
	ctx              context.Context
	gPool            gpool.Pool
	log              logger.Logger
}

// Item table cache item
type Item struct {
	addr           string
	successCount   uint
	failureCount   uint
	lastActiveTime int64
}

// isNeedLivenessProde 是否需要存活探测
// 仅检查cache表,在proxy表或direct表中,无需检查
// 无需活性探测条件:
//    successCount < successThreshold 且 successCount > failureCount 且 探测时隔小于activeDiff
func (sf *Item) isNeedLivenessProde(successThreshold, failureThreshold uint, aliveThreshold int64) bool {
	return !((sf.successCount >= successThreshold || sf.failureCount >= failureThreshold) &&
		(sf.successCount > sf.failureCount) && (time.Now().Unix()-sf.lastActiveTime < aliveThreshold))
}

// New new a filter for proxy
// 	intelligent : direct|proxy|intelligent, default: intelligent
// 	proxyFile和directFile的域名条目一行一条.
func New(intelligent string, opts ...Option) *Filter {
	ctx, cancel := context.WithCancel(context.Background())
	f := &Filter{
		intelligent,
		cmap.New(),
		cmap.New(),
		cmap.New(),
		time.Second * 1,
		time.Second * 30,
		nil,
		defaultThreshold,
		defaultThreshold,
		defaultAliveThreshold,
		cancel,
		ctx,
		nil,
		logger.NewDiscard(),
	}

	for _, opt := range opts {
		opt(f)
	}

	if f.livenessPeriod > 0 {
		go f.run()
	}
	return f
}

func (sf *Filter) Close() error {
	sf.cancel()
	return nil
}

// LoadProxyFile load proxy file with filename line byte line,return the count.
func (sf *Filter) LoadProxyFile(filename string) (int, error) {
	return loadfile2ConcurrentMap(&sf.proxies, filename)
}

// LoadDirectFile load direct file with filename line byte line,return the count.
func (sf *Filter) LoadDirectFile(filename string) (int, error) {
	return loadfile2ConcurrentMap(&sf.directs, filename)
}

// ProxyItemCount return proxy item count.
func (sf *Filter) ProxyItemCount() int {
	return sf.proxies.Count()
}

// DirectItemCount return direct item count.
func (sf *Filter) DirectItemCount() int {
	return sf.directs.Count()
}

// Add 增加一个域名-->地址(host:port)映射到过滤表, 如果proxy表且direct表都不存在时才进行添加
func (sf *Filter) Add(domain, addr string) {
	domain = hostname(domain)
	if !sf.Match(domain, false) && !sf.Match(domain, true) {
		sf.cache.SetIfAbsent(domain, Item{addr: addr})
	}
}

// IsProxy domain代理检查,返回是否需要代理,是否在cache,proxy,direct表中.
// 检查顺序:
// 1. 先检查proxy表,在proxy表中直接返回,否则执行2.
// 2. 再检查direct,在direct表中直接返回,否则执行3
// 3. 检查是否在cache表,不在直接返回.否则执行4.
// 4. 根据策略,如果是direct或proxy策略,直接返回策略规则,否则采用intelligent,进行智能判断.
// intelligent 策略判断规则:
//
func (sf *Filter) IsProxy(domain string) (proxy, inMap bool, failN, successN uint) {
	domain = hostname(domain)
	if sf.Match(domain, true) {
		return true, true, 0, 0
	}
	if sf.Match(domain, false) {
		return false, true, 0, 0
	}

	itm, ok := sf.cache.Get(domain)
	if !ok {
		return true, false, 0, 0
	}
	switch sf.intelligent {
	case "direct":
		return false, true, 0, 0
	case "proxy":
		return true, true, 0, 0
	case "intelligent":
		fallthrough
	default:
		item := itm.(Item)
		return (item.successCount <= item.failureCount) &&
				(time.Now().Unix()-item.lastActiveTime < sf.aliveThreshold),
			true, item.failureCount, item.successCount
	}
}

// Match 匹配域名,后缀型倒序匹配,
// domain: foo.bar.com
//    - foo.bar.com --> return true
//	  - bar.com  --> return true
//    - com  --> return true
func (sf *Filter) Match(domain string, isProxy bool) bool {
	hnSlice := strings.Split(hostname(domain), ".")
	if len(hnSlice) <= 1 {
		return false
	}

	tb := sf.directs
	if isProxy {
		tb = sf.proxies
	}
	for i := len(hnSlice) - 1; i >= 0; i-- {
		if tb.Has(strings.Join(hnSlice[i:], ".")) {
			return true
		}
	}
	return false
}

func (sf *Filter) run() {
	sf.log.Debugf("filter run started")
	tm := time.NewTicker(sf.livenessPeriod)
	defer func() {
		tm.Stop()
		sf.log.Debugf("filter run stopped")
	}()

	livenessProbe := func(ctx context.Context, addr string, timeout time.Duration) error {
		conn, err := net.DialTimeout("tcp", addr, timeout)
		if err != nil {
			return err
		}
		conn.Close() // nolint: errcheck
		return nil
	}
	if sf.livenessProbe != nil {
		livenessProbe = sf.livenessProbe
	}

	for {
		select {
		case <-sf.ctx.Done():
			return
		case <-tm.C:
		}

		for dom, itm := range sf.cache.Items() {
			// 需检查
			domain, item := dom, itm.(Item)
			if item.isNeedLivenessProde(sf.successThreshold, sf.failureThreshold, sf.aliveThreshold) {
				sf.goFunc(func() {
					now := time.Now().Unix()
					if now-item.lastActiveTime > sf.aliveThreshold {
						item.failureCount = 0
						item.successCount = 0
					}
					err := livenessProbe(sf.ctx, item.addr, sf.timeout)
					if err != nil {
						item.failureCount++
					} else {
						item.successCount++
					}
					item.lastActiveTime = now
					sf.cache.Set(domain, item)
				})
			}
		}
	}
}

// loadfile2ConcurrentMap load file with filename line byte line to cmp,return the count in cmp.
func loadfile2ConcurrentMap(cmp *cmap.ConcurrentMap, filename string) (int, error) {
	var n int

	_, err := os.Stat(filename)
	if err == nil || !os.IsNotExist(err) {
		contents, err := ioutil.ReadFile(filename)
		if err != nil {
			return n, err
		}
		// DOS/Windows系统 采用CRLF表示下一行
		// Linux/UNIX系统 采用LF表示下一行
		// MAC系统 采用CR表示下一行
		for _, line := range strings.Split(string(contents), "\n") {
			if line = strings.Trim(line, "\r \t"); line != "" {
				n++
				cmp.Set(line, struct{}{})
			}
		}
	}
	return n, nil
}

func hostname(domain string) string {
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimPrefix(domain, "https://")
	h, _, _ := net.SplitHostPort(domain)
	if h == "" {
		return domain
	}
	return h
}

func (sf *Filter) goFunc(f func()) {
	if sf.gPool == nil || sf.gPool.Submit(f) != nil {
		go func() {
			defer func() {
				if err := recover(); err != nil {
					sf.log.DPanicf("crashed %+v\nstack: %s", err, string(debug.Stack()))
				}
			}()
			f()
		}()
	}
}
