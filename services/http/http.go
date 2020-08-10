package http

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
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
	"github.com/thinkgos/jocasta/core/lb"
	"github.com/thinkgos/jocasta/cs"
	"github.com/thinkgos/jocasta/lib/cert"
	"github.com/thinkgos/jocasta/lib/extnet"
	"github.com/thinkgos/jocasta/lib/logger"
	"github.com/thinkgos/jocasta/pkg/httpc"
	"github.com/thinkgos/jocasta/pkg/sword"
	"github.com/thinkgos/jocasta/services"
	"github.com/thinkgos/jocasta/services/ccs"
	"github.com/thinkgos/jocasta/services/skcp"
)

type Config struct {
	// parent
	ParentType     string   // 父级协议, tcp|tls|stcp|kcp|ssh, default empty
	Parent         []string // 父级地址,格式addr:port, default empty
	ParentCompress bool     // 父级支持压缩传输, default false
	ParentKey      string   // 父级加密的key, default empty
	// local
	LocalType     string // 本地协议, tcp|tls|stcp|kcp, default tcp
	Local         string // 本地监听地址, 格式addr:port,多个以','分隔, default :28080
	LocalCompress bool   // 本地支持压缩传输, default false
	LocalKey      string // 本地加密的key default empty
	// tls 有效
	CertFile   string // cert文件名 default proxy.crt
	KeyFile    string // key文件名 default proxy.key
	CaCertFile string // ca文件名 default empty
	// kcp 有效
	SKCPConfig skcp.Config
	// stcp有效
	STCPMethod   string // stcp 加密方法 default aes-192-cfb
	STCPPassword string // stcp 加密密钥 default thinkgos's_goproxy
	// ssh有效
	SSHKeyFile     string // 私有key文件 default empty
	SSHKeyFileSalt string // 私有key加盐 default empty
	SSHUser        string // 用户
	SSHPassword    string // 密码
	// 其它
	Timeout time.Duration // tcp连接到父级或真实服务器超时时间,default 2000 单位ms
	Always  bool          // 是否一直使用父级代理,default false
	// 代理过滤
	ProxyFile   string        // 代理域文件名 default blocked
	DirectFile  string        // 直接域文件名 default direct
	HTTPTimeout time.Duration // http连接主机超时时间,单位ms,efault 3000
	Interval    time.Duration // 检查域名间隔,单位s,default 10
	// basic auth 配置
	AuthFile       string        // 授权文件,一行一对,格式user:passwrod, default empty
	Auth           []string      // 授权用户密码对, default empty
	AuthURL        string        // 授权url, default empty
	AuthURLTimeout time.Duration // 授权超时时间, 单位ms, default 3000
	AuthURLOkCode  int           // 授权成功的code, default 204
	AuthURLRetry   uint          // 授权重试次数, default 1
	// 自定义dns服务
	DNSAddress string // dns 解析服务器地址 default empty
	DNSTTL     int    // 解析结果缓存时间,单位秒 default 300s
	// 智能模式 代理过滤
	// direct 不在blocked都直连
	// parent 不在direct都走代理
	// intelligent blocked和direct都没有,智能判断
	// default intelligent
	Intelligent string
	// 负载均衡
	LoadBalanceMethod     string        // 负载均衡方法, roundrobin|leastconn|leasttime|hash|weight default roundrobin
	LoadBalanceTimeout    time.Duration // 负载均衡dial超时时间 default 500 单位ms
	LoadBalanceRetryTime  time.Duration // 负载均衡重试时间间隔 default 1000 单位ms
	LoadBalanceHashTarget bool          // hash方法时,选择hash的目标, 默认false
	LoadBalanceOnlyHA     bool          // 高可用模式, default false
	// 限速器
	RateLimit           string //  限制速字节/s,可设置为2m, 100k等数值,default 0,不限速
	LocalIPS            []string
	BindListen          bool
	Debug               bool
	CheckParentInterval int // not used
	// 跳板机 仅支持tls,tcp下使用
	// https://username:password@host:port
	// https://host:port
	// socks5://username:password@host:port
	// socks5://host:port
	Jumper string

	cert          []byte
	key           []byte
	caCert        []byte
	rateLimit     rate.Limit
	sshAuthMethod ssh.AuthMethod
}

type HTTP struct {
	cfg             Config
	channels        []cs.Channel
	filters         *filter.Filter
	basicAuthCenter *basicAuth.Center
	lb              *lb.Group
	domainResolver  *idns.Resolver
	sshClient       atomic.Value
	userConns       cmap.ConcurrentMap
	cancel          context.CancelFunc
	ctx             context.Context
	log             logger.Logger
	jumper          *cs.Jumper
	gPool           sword.GoPool
}

var _ services.Service = (*HTTP)(nil)

func New(log logger.Logger, cfg Config) *HTTP {
	return &HTTP{
		cfg:       cfg,
		channels:  make([]cs.Channel, 0),
		userConns: cmap.New(),
		log:       log,
		gPool:     sword.GPool,
	}
}

func (sf *HTTP) inspectConfig() (err error) {
	if len(sf.cfg.Parent) == 1 && (sf.cfg.Parent)[0] == "" {
		sf.cfg.Parent = []string{}
	}

	// tls 证书
	if sf.cfg.LocalType == "tls" || (sf.cfg.ParentType == "tls" && len(sf.cfg.Parent) > 0) {
		sf.cfg.cert, sf.cfg.key, err = cert.Parse(sf.cfg.CertFile, sf.cfg.KeyFile)
		if err != nil {
			return err
		}
		if sf.cfg.CaCertFile != "" {
			sf.cfg.caCert, err = ioutil.ReadFile(sf.cfg.CaCertFile)
			if err != nil {
				return fmt.Errorf("read ca file %+v", err)
			}
		}
	}

	if len(sf.cfg.Parent) > 0 {
		if sf.cfg.ParentType == "" {
			return fmt.Errorf("parent type required for %s", sf.cfg.Parent)
		}

		if !strext.Contains([]string{"tcp", "tls", "stcp", "kcp", "ssh"}, sf.cfg.ParentType) {
			return fmt.Errorf("parent type suport <tcp|tls|kcp|ssh>")
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

	if sf.cfg.RateLimit != "0" && sf.cfg.RateLimit != "" {
		size, err := meter.ParseBytes(sf.cfg.RateLimit)
		if err != nil {
			return fmt.Errorf("parse rate limit size, %+v", err)
		}
		sf.cfg.rateLimit = rate.Limit(size)
	}
	if sf.cfg.Jumper != "" {
		if sf.cfg.ParentType != "tls" && sf.cfg.ParentType != "tcp" {
			return fmt.Errorf("jumper only support one of <tls|tcp> but %s", sf.cfg.ParentType)
		}
		sf.jumper, err = cs.NewJumper(sf.cfg.Jumper)
		if err != nil {
			return fmt.Errorf("new jumper, %+v", err)
		}
	}
	return nil
}

func (sf *HTTP) InitService() (err error) {
	var opts []basicAuth.Option

	// init domain resolver
	if sf.cfg.DNSAddress != "" {
		sf.domainResolver = idns.New(sf.cfg.DNSAddress, sf.cfg.DNSTTL)
	}
	// init basic auth
	if sf.cfg.AuthFile != "" || len(sf.cfg.Auth) > 0 || sf.cfg.AuthURL != "" {
		if sf.domainResolver != nil {
			opts = append(opts, basicAuth.WithDNSServer(sf.domainResolver))
		}
		if sf.cfg.AuthURL != "" {
			opts = append(opts, basicAuth.WithAuthURL(sf.cfg.AuthURL, sf.cfg.AuthURLTimeout, sf.cfg.AuthURLOkCode, sf.cfg.AuthURLRetry))
			sf.log.Debugf("auth from url [ %s ]", sf.cfg.AuthURL)
		}
		sf.basicAuthCenter = basicAuth.New(opts...)

		n := sf.basicAuthCenter.Add(sf.cfg.Auth...)
		sf.log.Debugf("auth data added %d, total:%d", n, sf.basicAuthCenter.Total())

		if sf.cfg.AuthFile != "" {
			n, err := sf.basicAuthCenter.LoadFromFile(sf.cfg.AuthFile)
			if err != nil {
				return fmt.Errorf("load auth-file %v", err)
			}
			sf.log.Debugf("auth data added from file %d , total:%d", n, sf.basicAuthCenter.Total())
		}
	}
	//init lb
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
		configs := []lb.Config{}

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
			configs = append(configs, lb.Config{
				Address:     _addr,
				Weight:      weight,
				MinActive:   1,
				MaxInactive: 2,
				Timeout:     sf.cfg.LoadBalanceTimeout,
				RetryTime:   sf.cfg.LoadBalanceRetryTime,
			})
		}
		sf.lb = lb.NewGroup(lb.Method(sf.cfg.LoadBalanceMethod), configs, sf.domainResolver, sf.log, sf.cfg.Debug)
	}

	if sf.cfg.ParentType == "ssh" {
		sshClient, err := sf.dialSSH(sf.resolve(sf.lb.Select("", sf.cfg.LoadBalanceOnlyHA)))
		if err != nil {
			return fmt.Errorf("dial ssh fail, %s", err)
		}
		sf.sshClient.Store(sshClient)
		sf.gPool.Go(func() {
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
				address := sf.resolve(sf.lb.Select("", sf.cfg.LoadBalanceOnlyHA))
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
		sc, err := ccs.ListenAndServeAny(sf.cfg.LocalType, addr, sf.handle,
			ccs.Config{
				Cert:         sf.cfg.cert,
				Key:          sf.cfg.key,
				CaCert:       sf.cfg.caCert,
				KcpConfig:    sf.cfg.SKCPConfig.KcpConfig,
				STCPMethod:   sf.cfg.STCPMethod,
				STCPPassword: sf.cfg.STCPPassword,
				Compress:     sf.cfg.LocalCompress,
			})
		if err != nil {
			return err
		}
		sf.channels = append(sf.channels, sc)
		sf.log.Infof("use proxy %s on %s", sf.cfg.LocalType, sc.Addr())
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
		sf.lb.Stop()
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
				if lb.Method(sf.cfg.LoadBalanceMethod) == lb.SELECT_HASH && sf.cfg.LoadBalanceHashTarget {
					selectAddr = targetDomainAddr
				}
				lbAddr = sf.lb.Select(selectAddr, sf.cfg.LoadBalanceOnlyHA)
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
	method := "DIRECT"
	if useProxy {
		method = "PROXY"
	}
	sf.log.Infof("< %s > use %s", targetDomainAddr, method)

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
		sf.lb.IncreaseConns(lbAddr)
	}

	sf.log.Debugf("conn %s - %s connected [%s]", srcAddr, targetAddr, req.Host)
	defer func() {
		sf.userConns.Remove(srcAddr)
		if len(sf.cfg.Parent) > 0 {
			sf.lb.DecreaseConns(lbAddr)
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
		outConn, err = ccs.DialTimeout(sf.cfg.ParentType, address, sf.cfg.Timeout,
			ccs.Config{
				Cert:         sf.cfg.cert,
				Key:          sf.cfg.key,
				CaCert:       sf.cfg.caCert,
				KcpConfig:    sf.cfg.SKCPConfig.KcpConfig,
				STCPMethod:   sf.cfg.STCPMethod,
				STCPPassword: sf.cfg.STCPPassword,
				Compress:     sf.cfg.ParentCompress,
				Jumper:       sf.jumper,
			})
	case "ssh":
		t := time.NewTimer(sf.cfg.Timeout * 2)
		defer t.Stop()
		boff := backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Second*3), 1)
		boff = backoff.WithContext(boff, sf.ctx)

		err = backoff.Retry(func() (er error) {
			sshClient := sf.sshClient.Load().(*ssh.Client)
			wait := make(chan struct{}, 1)
			sf.gPool.Go(func() {
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
		if !extnet.IsInternalIP(localIP) {
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

func (sf *HTTP) isUseProxy(address string) bool {
	if len(sf.cfg.Parent) > 0 {
		host, _, _ := net.SplitHostPort(address)
		if extnet.IsDomain(host) && sf.cfg.Always || !extnet.IsInternalIP(host) {
			if sf.cfg.Always {
				return true
			}
			useProxy, isInMap, _, _ := sf.filters.IsProxy(address)
			if !isInMap {
				sf.filters.Add(address, sf.resolve(address))
			}
			return useProxy
		}
	}
	return false
}
