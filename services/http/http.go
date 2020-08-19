package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"runtime/debug"
	"strconv"
	"sync/atomic"

	"net"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	cmap "github.com/orcaman/concurrent-map"
	"github.com/thinkgos/meter"
	"github.com/thinkgos/strext"
	"golang.org/x/crypto/ssh"
	"golang.org/x/time/rate"

	"github.com/thinkgos/jocasta/connection/ccrypt"
	"github.com/thinkgos/jocasta/connection/ciol"
	"github.com/thinkgos/jocasta/core/basicAuth"
	"github.com/thinkgos/jocasta/core/filter"
	"github.com/thinkgos/jocasta/core/idns"
	"github.com/thinkgos/jocasta/core/issh"
	"github.com/thinkgos/jocasta/core/loadbalance"
	"github.com/thinkgos/jocasta/cs"
	"github.com/thinkgos/jocasta/lib/cert"
	"github.com/thinkgos/jocasta/lib/extnet"
	"github.com/thinkgos/jocasta/lib/logger"
	"github.com/thinkgos/jocasta/lib/ternary"
	"github.com/thinkgos/jocasta/pkg/ccs"
	"github.com/thinkgos/jocasta/pkg/httpc"
	"github.com/thinkgos/jocasta/pkg/sword"
	"github.com/thinkgos/jocasta/services"
)

type Config struct {
	// parent
	ParentType     string   // 父级协议, tcp|tls|stcp|kcp|ssh, default: empty
	Parent         []string // 父级地址,格式addr:port, default: empty
	ParentCompress bool     // 父级支持压缩传输, default: false
	ParentKey      string   // 父级加密的key, default: empty
	// local
	LocalType     string // 本地协议, tcp|tls|stcp|kcp, default tcp
	Local         string // 本地监听地址, 格式addr:port,多个以','分隔, default `:28080`
	LocalCompress bool   // 本地支持压缩传输, default: false
	LocalKey      string // 本地加密的key default: empty
	// tls 有效
	CertFile   string // cert文件名 default: proxy.crt
	KeyFile    string // key文件名 default: proxy.key
	CaCertFile string // ca文件名 default: empty
	// kcp 有效
	SKCPConfig ccs.SKCPConfig
	// stcp有效
	// stcp 加密方法 default: aes-192-cfb
	// stcp 加密密钥 default: thinkgos's_jocasta
	STCPConfig cs.StcpConfig
	// ssh有效
	SSHKeyFile     string // ssh 私有key文件 default: empty
	SSHKeyFileSalt string // ssh 私有key加盐 default: empty
	SSHUser        string // ssh 用户
	SSHPassword    string // ssh 密码
	// 其它
	Timeout time.Duration // 连接父级或真实服务器超时时间,default: 2s
	Always  bool          // 强制一直使用父级代理,default: false
	// 代理过滤
	ProxyFile   string        // 代理域文件名 default: blocked
	DirectFile  string        // 直连域文件名 default: direct
	HTTPTimeout time.Duration // http连接主机超时时间 default: 3s
	Interval    time.Duration // 检查域名间隔 default: 10s
	// basic auth 配置
	AuthFile       string        // 授权文件,一行一条(格式user:passwrod), default empty
	Auth           []string      // 授权用户密码对, default empty
	AuthURL        string        // 外部认证授权url, default: empty
	AuthURLTimeout time.Duration // 外部认证授权超时时间, default: 3s
	AuthURLOkCode  int           // 外部认证授权成功的code, default: 204
	AuthURLRetry   uint          // 外部认证授权重试次数, default: 1
	// 自定义dns服务
	DNSAddress string // dns 解析服务器地址 default: empty
	DNSTTL     int    // 解析结果缓存时间,单位秒 default: 300
	// 代理过滤 default: intelligent
	//      direct 不在blocked都直连
	//      proxy  不在direct都走代理
	//      intelligent blocked和direct都没有,智能判断
	Intelligent string
	// 负载均衡
	LoadBalanceMethod     string        // 负载均衡方法, random|roundrobin|leastconn|hash|addrhash|leasttime|weight default: roundrobin
	LoadBalanceTimeout    time.Duration // 负载均衡dial超时时间 default 500ms
	LoadBalanceRetryTime  time.Duration // 负载均衡重试时间间隔 default 1000ms
	LoadBalanceHashTarget bool          // hash方法时,选择hash的目标, default: false
	LoadBalanceOnlyHA     bool          // 高可用模式, default false
	// 限速器
	RateLimit  string //  限制速byte/s,可设置为2m, 100k等数值,0表示不限速 default: 0
	LocalIPS   []string
	BindListen bool
	Debug      bool
	// 通过代理 支持tcp,tls,tcp下使用
	//      https://username:password@host:port
	//      https://host:port
	//      socks5://username:password@host:port
	//      socks5://host:port
	RawProxyURL string

	// private
	tcpTlsConfig  cs.TLSConfig
	rateLimit     rate.Limit
	sshAuthMethod ssh.AuthMethod
}

type HTTP struct {
	cfg             Config
	channels        []cs.Server
	filters         *filter.Filter
	basicAuthCenter *basicAuth.Center
	lb              *loadbalance.Balanced
	domainResolver  *idns.Resolver
	sshClient       atomic.Value
	userConns       cmap.ConcurrentMap
	cancel          context.CancelFunc
	ctx             context.Context
	log             logger.Logger
	proxyURL        *url.URL
	goPool          sword.GoPool
}

var _ services.Service = (*HTTP)(nil)

func New(log logger.Logger, cfg Config) *HTTP {
	return &HTTP{
		cfg:       cfg,
		channels:  make([]cs.Server, 0),
		userConns: cmap.New(),
		log:       log,
		goPool:    sword.GPool,
	}
}

func (sf *HTTP) inspectConfig() (err error) {
	if len(sf.cfg.Parent) == 1 && (sf.cfg.Parent)[0] == "" {
		sf.cfg.Parent = []string{}
	}

	if len(sf.cfg.Parent) > 0 {
		if sf.cfg.ParentType == "" {
			return fmt.Errorf("parent type required for %s", sf.cfg.Parent)
		}
		if !strext.Contains([]string{"tcp", "tls", "stcp", "kcp", "ssh"}, sf.cfg.ParentType) {
			return fmt.Errorf("parent type suport <tcp|tls|stcp|kcp|ssh>")
		}
		if !strext.Contains(loadbalance.Methods(), sf.cfg.LoadBalanceMethod) {
			return fmt.Errorf("load balance method should be oneof <%s>", strings.Join(loadbalance.Methods(), ", "))
		}

		// ssh 证书
		if sf.cfg.ParentType == "ssh" {
			if sf.cfg.SSHUser == "" {
				return fmt.Errorf("ssh user required")
			}
			if sf.cfg.SSHKeyFile == "" && sf.cfg.SSHPassword == "" {
				return fmt.Errorf("ssh password or key file required")
			}

			if sf.cfg.SSHPassword != "" {
				sf.cfg.sshAuthMethod = ssh.Password(sf.cfg.SSHPassword)
			} else {
				if sf.cfg.SSHKeyFileSalt != "" {
					sf.cfg.sshAuthMethod, err = issh.ParsePrivateKeyFile2AuthMethod(sf.cfg.SSHKeyFile, []byte(sf.cfg.SSHKeyFileSalt))
				} else {
					sf.cfg.sshAuthMethod, err = issh.ParsePrivateKeyFile2AuthMethod(sf.cfg.SSHKeyFile)
				}
				if err != nil {
					return fmt.Errorf("parse ssh private key file, %+v", err)
				}
			}
		}
	}

	// tls 证书
	if sf.cfg.LocalType == "tls" || (sf.cfg.ParentType == "tls" && len(sf.cfg.Parent) > 0) {
		if sf.cfg.CertFile == "" || sf.cfg.KeyFile == "" {
			return errors.New("cert file and key file required")
		}
		if sf.cfg.tcpTlsConfig.Cert, sf.cfg.tcpTlsConfig.Key, err = cert.LoadPair(sf.cfg.CertFile, sf.cfg.KeyFile); err != nil {
			return err
		}
		if sf.cfg.CaCertFile != "" {
			if sf.cfg.tcpTlsConfig.CaCert, err = ioutil.ReadFile(sf.cfg.CaCertFile); err != nil {
				return fmt.Errorf("read ca file %+v", err)
			}
		}
	}

	if sf.cfg.RateLimit != "0" && sf.cfg.RateLimit != "" {
		size, err := meter.ParseBytes(sf.cfg.RateLimit)
		if err != nil {
			return fmt.Errorf("parse rate limit size, %+v", err)
		}
		sf.cfg.rateLimit = rate.Limit(size)
	}
	if sf.cfg.RawProxyURL != "" {
		if !strext.Contains([]string{"tls", "tcp"}, sf.cfg.ParentType) {
			return fmt.Errorf("proxyURL only support one of <tls|tcp> but %s", sf.cfg.ParentType)
		}
		if sf.proxyURL, err = cs.ParseProxyURL(sf.cfg.RawProxyURL); err != nil {
			return fmt.Errorf("new proxyURL, %+v", err)
		}
	}
	return nil
}

func (sf *HTTP) InitService() (err error) {
	// init domain resolver
	if sf.cfg.DNSAddress != "" {
		sf.domainResolver = idns.New(sf.cfg.DNSAddress, sf.cfg.DNSTTL)
	}
	// init basic auth
	if sf.cfg.AuthFile != "" || len(sf.cfg.Auth) > 0 || sf.cfg.AuthURL != "" {
		var opts []basicAuth.Option

		if sf.domainResolver != nil {
			opts = append(opts, basicAuth.WithDNSServer(sf.domainResolver))
		}
		if sf.cfg.AuthURL != "" {
			opts = append(opts, basicAuth.WithAuthURL(sf.cfg.AuthURL, sf.cfg.AuthURLTimeout, sf.cfg.AuthURLOkCode, sf.cfg.AuthURLRetry))
		}
		sf.basicAuthCenter = basicAuth.New(opts...)

		n := sf.basicAuthCenter.Add(sf.cfg.Auth...)
		sf.log.Debugf("auth data added %d, total:%d", n, sf.basicAuthCenter.Total())

		if sf.cfg.AuthFile != "" {
			n, err := sf.basicAuthCenter.LoadFromFile(sf.cfg.AuthFile)
			if err != nil {
				return fmt.Errorf("load auth-file failed, %v", err)
			}
			sf.log.Debugf("auth data added from file %d , total: %d", n, sf.basicAuthCenter.Total())
		}
	}

	// init lb
	if len(sf.cfg.Parent) > 0 {
		sf.filters = filter.New(sf.cfg.Intelligent,
			filter.WithTimeout(sf.cfg.HTTPTimeout),
			filter.WithLivenessPeriod(sf.cfg.Interval),
			filter.WithGPool(sword.GPool), filter.WithLogger(sf.log),
		)
		var count int
		count, err = sf.filters.LoadProxyFile(sf.cfg.ProxyFile)
		if err != nil {
			sf.log.Warnf("load proxy file(%s) %+v", sf.cfg.ProxyFile, err)
		} else {
			sf.log.Debugf("load proxy file, domains count: %d", count)
		}
		count, err = sf.filters.LoadDirectFile(sf.cfg.DirectFile)
		if err != nil {
			sf.log.Warnf("load direct file(%s) %+v", sf.cfg.ProxyFile, err)
		} else {
			sf.log.Debugf("load direct file, domains count: %d", count)
		}

		// init lb
		configs := []loadbalance.Config{}

		for _, addr := range sf.cfg.Parent {
			_addrInfo := strings.Split(addr, "@")
			_addr := _addrInfo[0]
			weight := 1
			if len(_addrInfo) == 2 {
				weight, _ = strconv.Atoi(_addrInfo[1])
				if weight == 0 {
					weight = 1
				}
			}
			configs = append(configs, loadbalance.Config{
				Addr:             _addr,
				Weight:           weight,
				SuccessThreshold: 1,
				FailureThreshold: 2,
				Period:           sf.cfg.LoadBalanceRetryTime,
				Timeout:          sf.cfg.LoadBalanceTimeout,
			})
		}
		sf.lb = loadbalance.New(sf.cfg.LoadBalanceMethod, configs,
			loadbalance.WithDNSServer(sf.domainResolver),
			loadbalance.WithLogger(sf.log),
			loadbalance.WithEnableDebug(sf.cfg.Debug),
			loadbalance.WithGPool(sf.goPool),
		)
	}

	if sf.cfg.ParentType == "ssh" {
		sshClient, err := sf.dialSSH(sf.resolve(sf.lb.Select("")))
		if err != nil {
			return fmt.Errorf("dial ssh fail, %s", err)
		}
		sf.sshClient.Store(sshClient)
		sf.goPool.Go(func() {
			t := time.NewTicker(time.Second * 10)
			sf.log.Debugf("ssh keepalive started")
			defer func() {
				t.Stop()
				sf.log.Debugf("ssh keepalive stopped")
				if e := recover(); e != nil {
					sf.log.DPanicf("crashed %s\nstack:\n%s", e, string(debug.Stack()))
				}
			}()

			//循环检查ssh网络连通性
			for {
				address := sf.resolve(sf.lb.Select(""))
				conn, err := net.DialTimeout("tcp", address, sf.cfg.Timeout*2)
				if err != nil {
					sf.sshClient.Load().(*ssh.Client).Close()
					sf.log.Infof("ssh disconnect, retrying...")
					sshClient, e := sf.dialSSH(address)
					if e != nil {
						sf.log.Infof("ssh reconnect failed")
					} else {
						sf.log.Infof("<** http **> ssh reconnect success")
						sf.sshClient.Store(sshClient)
					}
				} else {
					_ = extnet.WrapWriteTimeout(conn, sf.cfg.Timeout, func(c net.Conn) error {
						_, err := c.Write([]byte{0})
						return err
					})
					conn.Close()
				}
				select {
				case <-t.C:
				case <-sf.ctx.Done():
					return
				}
			}
		})
	}
	return
}

func (sf *HTTP) Start() (err error) {
	sf.ctx, sf.cancel = context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			sf.Stop()
		}
	}()
	if err = sf.inspectConfig(); err != nil {
		return
	}
	if err = sf.InitService(); err != nil {
		return
	}

	lAddrs := strings.Split(sf.cfg.Local, ",")
	for _, addr := range lAddrs {
		if addr == "" {
			continue
		}

		srv := ccs.Server{
			Protocol: sf.cfg.LocalType,
			Addr:     addr,
			Config: ccs.Config{
				TCPTlsConfig: sf.cfg.tcpTlsConfig,
				StcpConfig:   sf.cfg.STCPConfig,
				KcpConfig:    sf.cfg.SKCPConfig.KcpConfig,
			},
			GoPool:      sf.goPool,
			AfterChains: cs.AdornConnsChain{cs.AdornCsnappy(sf.cfg.LocalCompress)},
			Handler:     cs.HandlerFunc(sf.handle),
		}
		sc, errChan := srv.RunListenAndServe()
		if err = <-errChan; err != nil {
			return err
		}
		sf.channels = append(sf.channels, sc)
		sf.log.Infof("use proxy %s on %s", sf.cfg.LocalType, sc.LocalAddr())
	}
	if len(sf.cfg.Parent) > 0 {
		sf.log.Infof("use parent %s < %v [ %s ] >", sf.cfg.ParentType, sf.cfg.Parent, strings.ToUpper(sf.cfg.LoadBalanceMethod))
	}
	return
}

func (sf *HTTP) Stop() {
	if sf.cancel != nil {
		sf.cancel()
	}
	for _, sc := range sf.channels {
		sc.Close()
	}
	if sf.lb != nil {
		sf.lb.Close()
	}
	if len(sf.cfg.Parent) > 0 {
		sf.filters.Close()
	}
	if sf.cfg.ParentType == "ssh" {
		sf.sshClient.Load().(*ssh.Client).Close()
	}
	for _, c := range sf.userConns.Items() {
		c.(io.Closer).Close()
	}
	sf.log.Infof("service http(s) stopped")
}

func (sf *HTTP) handle(inConn net.Conn) {
	defer inConn.Close()

	if sf.cfg.LocalKey != "" {
		inConn = ccrypt.New(inConn, ccrypt.Config{Password: sf.cfg.LocalKey})
	}

	req, err := httpc.New(inConn, 4096,
		httpc.WithBasicAuth(sf.basicAuthCenter),
		httpc.WithLogger(sf.log),
	)
	if err != nil {
		if err != io.EOF {
			sf.log.Errorf("decoder error , from %s, ERR:%s", inConn.RemoteAddr(), err)
		}
		return
	}

	srcAddr := inConn.RemoteAddr().String()
	localAddr := inConn.LocalAddr().String()
	targetDomainAddr := req.Host

	if sf.IsDeadLoop(localAddr, targetDomainAddr) {
		sf.log.Errorf("dead loop detected , %s", targetDomainAddr)
		return
	}
	var targetConn net.Conn
	var lbAddr string

	useProxy := sf.isUseProxy(targetDomainAddr)
	if useProxy {
		boff := backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Second), 5)
		boff = backoff.WithContext(boff, sf.ctx)
		err = backoff.Retry(func() (er error) {
			dialAddr := targetDomainAddr
			if sf.cfg.ParentType != "ssh" {
				selectAddr := inConn.RemoteAddr().String()
				if sf.cfg.LoadBalanceMethod == "hash" && sf.cfg.LoadBalanceHashTarget {
					selectAddr = targetDomainAddr
				}
				lbAddr = sf.lb.Select(selectAddr)
				dialAddr = lbAddr
			}
			targetConn, er = sf.dialParent(dialAddr)
			return er
		}, boff)
	} else {
		targetConn, err = sf.dialDirect(sf.resolve(targetDomainAddr), localAddr)
	}
	if err != nil {
		sf.log.Errorf("dial conn failed, %v", err)
		return
	}

	if useProxy && sf.cfg.ParentKey != "" {
		targetConn = ccrypt.New(targetConn, ccrypt.Config{Password: sf.cfg.ParentKey})
	}

	if req.IsHTTPS() && (!useProxy || sf.cfg.ParentType == "ssh") {
		//https无上级或者上级非代理,proxy需要响应connect请求,并直连目标
		err = req.HTTPSReply()
		if err != nil {
			sf.log.Errorf("https reply, %s", err)
			return
		}
	} else {
		//https或者http,上级是代理,proxy需要转发
		targetConn.SetDeadline(time.Now().Add(sf.cfg.Timeout))
		//直连目标或上级非代理或非SNI,,清理HTTP头部的代理头信息
		if (!useProxy || sf.cfg.ParentType == "ssh") && !req.IsSNI {
			_, err = targetConn.Write(extnet.RemoveProxyHeaders(req.RawHeader))
		} else {
			_, err = targetConn.Write(req.RawHeader)
		}
		targetConn.SetDeadline(time.Time{})
		if err != nil {
			sf.log.Errorf("write to %s , err:%s", lbAddr, err)
			return
		}
	}

	sf.log.Infof("< %s > use %s", targetDomainAddr, ternary.IfString(useProxy, "PROXY", "DIRECT"))

	if sf.cfg.rateLimit > 0 {
		targetConn = ciol.New(targetConn, ciol.WithReadLimiter(sf.cfg.rateLimit))
	}

	targetAddr := targetConn.RemoteAddr().String()

	sf.userConns.Upsert(srcAddr, inConn, func(exist bool, valueInMap interface{}, newValue interface{}) interface{} {
		if exist {
			valueInMap.(net.Conn).Close()
		}
		return newValue
	})
	if len(sf.cfg.Parent) > 0 {
		sf.lb.ConnsIncrease(lbAddr)
	}

	sf.log.Debugf("conn %s - %s connected [%s]", srcAddr, targetAddr, req.Host)
	defer func() {
		sf.userConns.Remove(srcAddr)
		if len(sf.cfg.Parent) > 0 {
			sf.lb.ConnsDecrease(lbAddr)
		}
		sf.log.Infof("conn %s - %s released [%s]", srcAddr, targetAddr, req.Host)
	}()

	err = sword.Binding.Proxy(inConn, targetConn)
}

func (sf *HTTP) IsDeadLoop(inLocalAddr string, host string) bool {
	inIP, inPort, err := net.SplitHostPort(inLocalAddr)
	if err != nil {
		return false
	}
	outDomain, outPort, err := net.SplitHostPort(host)
	if err != nil {
		return false
	}
	if inPort == outPort {
		var outIPs []net.IP
		if sf.cfg.DNSAddress != "" {
			outIPs = []net.IP{net.ParseIP(sf.resolve(outDomain))}
		} else {
			outIPs, err = net.LookupIP(outDomain)
		}
		if err == nil {
			for _, ip := range outIPs {
				if ip.String() == inIP {
					return true
				}
			}
		}
		interfaceIPs, err := sword.SystemNetworkIPs()
		for _, ip := range sf.cfg.LocalIPS {
			interfaceIPs = append(interfaceIPs, net.ParseIP(ip).To4())
		}
		if err == nil {
			for _, localIP := range interfaceIPs {
				for _, outIP := range outIPs {
					if localIP.Equal(outIP) {
						return true
					}
				}
			}
		}
	}
	return false
}

func (sf *HTTP) resolve(address string) string {
	if sf.domainResolver != nil {
		return sf.domainResolver.MustResolve(address)
	}
	return address
}

// dialParent 获得父级连接
func (sf *HTTP) dialParent(address string) (outConn net.Conn, err error) {
	switch sf.cfg.ParentType {
	case "tcp", "tls", "stcp", "kcp":
		d := ccs.Dialer{
			Protocol: sf.cfg.ParentType,
			Timeout:  sf.cfg.Timeout,
			Config: ccs.Config{
				TCPTlsConfig: sf.cfg.tcpTlsConfig,
				StcpConfig:   sf.cfg.STCPConfig,
				KcpConfig:    sf.cfg.SKCPConfig.KcpConfig,
				ProxyURL:     sf.proxyURL,
			},
			AfterChains: cs.AdornConnsChain{cs.AdornCsnappy(sf.cfg.ParentCompress)},
		}
		outConn, err = d.Dial("tcp", address)
	case "ssh":
		t := time.NewTimer(sf.cfg.Timeout * 2)
		defer t.Stop()
		boff := backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Second*3), 1)
		boff = backoff.WithContext(boff, sf.ctx)

		err = backoff.Retry(func() (er error) {
			sshClient := sf.sshClient.Load().(*ssh.Client)
			wait := make(chan struct{}, 1)
			sf.goPool.Go(func() {
				outConn, er = sshClient.Dial("tcp", address)
				wait <- struct{}{}
			})

			t.Reset(sf.cfg.Timeout * 2)
			select {
			case <-sf.ctx.Done():
				return backoff.Permanent(io.ErrClosedPipe)
			case <-t.C:
				er = fmt.Errorf("ssh dial %s timeout", address)
			case <-wait:
			}
			if er != nil {
				sf.log.Errorf("connect ssh fail, %v, retrying...", er)
			}
			return er
		}, boff)
	}
	return
}

func (sf *HTTP) dialDirect(address string, localAddr string) (net.Conn, error) {
	if sf.cfg.BindListen {
		localIP, _, _ := net.SplitHostPort(localAddr)
		if !extnet.IsIntranet(localIP) {
			local, _ := net.ResolveTCPAddr("tcp", localIP+":0")
			d := net.Dialer{
				Timeout:   sf.cfg.Timeout,
				LocalAddr: local,
			}
			return d.Dial("tcp", address)
		}
	}
	return net.DialTimeout("tcp", address, sf.cfg.Timeout)
}

func (sf *HTTP) dialSSH(lAddr string) (*ssh.Client, error) {
	return ssh.Dial("tcp", sf.resolve(lAddr), &ssh.ClientConfig{
		User:    sf.cfg.SSHUser,
		Auth:    []ssh.AuthMethod{sf.cfg.sshAuthMethod},
		Timeout: sf.cfg.Timeout,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	})
}

func (sf *HTTP) isUseProxy(addr string) bool {
	if len(sf.cfg.Parent) > 0 {
		host, _, _ := net.SplitHostPort(addr)
		if extnet.IsDomain(host) && sf.cfg.Always {
			return true
		}

		if !extnet.IsIntranet(host) {
			useProxy, inMap, _, _ := sf.filters.IsProxy(addr)
			if !inMap {
				sf.filters.Add(addr, sf.resolve(addr))
			}
			return useProxy
		}
	}
	return false
}
