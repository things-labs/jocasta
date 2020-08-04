// package idns 本地 dns 解析服务
package idns

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
	cmap "github.com/orcaman/concurrent-map"
)

// Resolver 本地 dns 解析服务
type Resolver struct {
	publicDNSAddr string             // 外部dns地址
	ttl           int                // 单位: 秒
	cache         cmap.ConcurrentMap // 缓存 domain --> Item
}

// Item 缓存条目
type Item struct {
	ip        string // ip地址
	expiredAt int64  // 过期时间,unix时间
}

// New 创建一个本地dns服务,提供公共dns地址和缓存ttl超时时间,单位s
func New(publicDNSAddr string, ttl int) *Resolver {
	return &Resolver{
		publicDNSAddr,
		ttl,
		cmap.New(),
	}
}

// TTL 获取缓存条目超时时间,单位秒
func (sf *Resolver) TTL() int {
	return sf.ttl
}

// PublicDNSAddr 获取公共dns地址
func (sf *Resolver) PublicDNSAddr() string {
	return sf.publicDNSAddr
}

// MustResolve 域名解析,如果地址无法解析,将返回输入值
func (sf *Resolver) MustResolve(address string) string {
	ip, err := sf.Resolve(address)
	if err != nil {
		return address
	}
	return ip
}

// Resolve 域名解析,返回ip地址,返回格式由请求的格式决定
// domain: 域名:port -> ip:port
// domain: 域名 -> ip
// domain: ip -> ip
// domain: ip:port - > ip:port
func (sf *Resolver) Resolve(domain string) (string, error) {
	var err error
	var port string

	dstDomain := domain
	if strings.Contains(domain, ":") {
		if dstDomain, port, err = net.SplitHostPort(domain); err != nil {
			return "", err
		}
	}

	// 本身就是ip
	if net.ParseIP(dstDomain) != nil {
		return joinIPPort(dstDomain, port), nil
	}

	var itm *Item
	// 查缓存
	_itm, ok := sf.cache.Get(dstDomain)
	if ok {
		itm = _itm.(*Item)
		if itm.expiredAt > time.Now().Unix() {
			return joinIPPort(itm.ip, port), nil
		}
	} else {
		itm = &Item{}
	}

	cli := &dns.Client{
		DialTimeout:  time.Millisecond * 5000,
		ReadTimeout:  time.Millisecond * 5000,
		WriteTimeout: time.Millisecond * 5000,
	}

	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(dstDomain), dns.TypeA)
	msg.RecursionDesired = true
	r, _, err := cli.Exchange(msg, sf.publicDNSAddr)
	if err != nil || r == nil {
		return "", err
	}
	if r.Rcode != dns.RcodeSuccess {
		return "", fmt.Errorf("invalid answer name %s after A query for %s", dstDomain, sf.publicDNSAddr)
	}

	for _, answer := range r.Answer {
		if answer.Header().Rrtype == dns.TypeA {
			info := strings.Fields(answer.String())
			if len(info) >= 5 {
				itm.ip = info[4]
				itm.expiredAt = time.Now().Unix() + int64(sf.ttl)
				sf.cache.Set(dstDomain, itm)
				return joinIPPort(info[4], port), nil
			}
		}
	}
	return "", fmt.Errorf("unknow answer")
}

func joinIPPort(ip, port string) string {
	if port != "" {
		return net.JoinHostPort(ip, port)
	}
	return ip
}
