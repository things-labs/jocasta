package socks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	cmap "github.com/orcaman/concurrent-map"
	"go.uber.org/atomic"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/proxy"
	"golang.org/x/time/rate"

	"github.com/thinkgos/go-socks5"
	"github.com/thinkgos/go-socks5/statute"
	"github.com/thinkgos/meter"
	"github.com/thinkgos/strext"

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
	"github.com/thinkgos/jocasta/pkg/ccs"
	"github.com/thinkgos/jocasta/pkg/sword"
	"github.com/thinkgos/jocasta/services"
)

type Config struct {
	// parent
	ParentType     string   // 父级协议类型 tcp|tls|stcp|kcp|ssh ,default tcp
	Parent         []string // 父级地址,格式addr:port, default nil
	ParentCompress bool     // default false
	ParentKey      string   // default empty
	ParentAuth     string   // 上级socks5授权用户密码,格式username:password, default empty
	// local
	LocalType     string // 本地协议类型 tcp|tls|stcp|kcp
	Local         string // 本地监听地址 default :28080
	LocalCompress bool   // default false
	LocalKey      string // default empty
	// tls有效
	CertFile   string // cert文件 default proxy.crt
	KeyFile    string // key文件 default proxy.key
	CaCertFile string // ca文件 default empty
	// kcp有效
	SKCPConfig ccs.SKCPConfig
	// stcp有效
	// stcp 加密方法 default: aes-192-cfb
	// stcp 加密密钥 default: thinkgos's_jocasta
	STCPConfig cs.StcpConfig
	// ssh有效
	SSHKeyFile     string // default empty
	SSHKeyFileSalt string // default empty
	SSHUser        string // default empty
	SSHPassword    string // default empty
	// 其它
	Timeout time.Duration // default 5000 单位ms
	Always  bool          // 强制所有域名走代理 default false
	// 代理过滤
	ProxyFile  string        // 代理域文件名 default blocked
	DirectFile string        // 直接域文件名 default direct
	Interval   time.Duration // 检查域名间隔,单位s, default 10s
	// basic auth 配置
	AuthFile       string        // default empty
	Auth           []string      // default empty
	AuthURL        string        // default empty
	AuthURLTimeout time.Duration // default 3000 单位ms
	AuthURLOkCode  int           // default 204
	AuthURLRetry   uint          // default 0
	// dns域名解析
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
	// 限速器
	RateLimit  string   //  限制速字节/s,可设置为2m, 100k等数值,default 0,不限速
	LocalIPS   []string // default empty
	BindListen bool     // default false
	Debug      bool

	// private
	tcpTlsConfig  cs.TLSConfig
	sshAuthMethod ssh.AuthMethod
	rateLimit     rate.Limit
	parentAuth    *proxy.Auth
}

type Socks struct {
	cfg                   Config
	channel               cs.Server
	socks5Srv             *socks5.Server
	filters               *filter.Filter
	basicAuthCenter       *basicAuth.Center
	lb                    *loadbalance.Balanced
	domainResolver        *idns.Resolver
	sshClient             atomic.Value
	userConns             cmap.ConcurrentMap
	udpRelatedPacketConns cmap.ConcurrentMap
	goPool                sword.GoPool
	cancel                context.CancelFunc
	ctx                   context.Context
	log                   logger.Logger
	udpLocalKey           []byte
	udpParentKey          []byte
}

var _ services.Service = (*Socks)(nil)

func New(log logger.Logger, cfg Config) *Socks {
	return &Socks{
		cfg:                   cfg,
		userConns:             cmap.New(),
		udpRelatedPacketConns: cmap.New(),
		goPool:                sword.GPool,
		log:                   log,
	}
}

func (sf *Socks) inspectConfig() (err error) {
	if len(sf.cfg.Parent) == 1 && (sf.cfg.Parent)[0] == "" {
		sf.cfg.Parent = []string{}
	}

	if sf.cfg.LocalType == "tls" || (sf.cfg.ParentType == "tls" && len(sf.cfg.Parent) > 0) {
		sf.cfg.tcpTlsConfig.Cert, sf.cfg.tcpTlsConfig.Key, err = cert.Parse(sf.cfg.CertFile, sf.cfg.KeyFile)
		if err != nil {
			return err
		}
		if sf.cfg.CaCertFile != "" {
			sf.cfg.tcpTlsConfig.CaCert, err = ioutil.ReadFile(sf.cfg.CaCertFile)
			if err != nil {
				return fmt.Errorf("read ca file, %s", err)
			}
		}
	}

	if len(sf.cfg.Parent) > 0 {
		if sf.cfg.ParentType == "" {
			return fmt.Errorf("parent type required for %s", sf.cfg.Parent)
		}
		if !strext.Contains([]string{"tcp", "tls", "stcp", "kcp", "ssh"}, sf.cfg.ParentType) {
			return fmt.Errorf("parent type suport <tcp|tls|stcp|kcp|ssh>")
		}
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
			return fmt.Errorf("parse rate limit size, %s", err)
		}
		sf.cfg.rateLimit = rate.Limit(size)
	}
	if sf.cfg.ParentAuth != "" {
		au := strings.Split(sf.cfg.ParentAuth, ":")
		if len(au) != 2 {
			return errors.New("parent auth data format invalid")
		}
		sf.cfg.parentAuth = &proxy.Auth{User: au[0], Password: au[1]}
	}

	sf.udpLocalKey = sf.localUDPKey()
	sf.udpParentKey = sf.parentUDPKey()
	return
}

func (sf *Socks) initService() (err error) {
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
			sf.log.Infof("auth from url[ %s ]", sf.cfg.AuthURL)
		}
		sf.basicAuthCenter = basicAuth.New(opts...)

		n := sf.basicAuthCenter.Add(sf.cfg.Auth...)
		sf.log.Infof("auth data added %d, total: %d", n, sf.basicAuthCenter.Total())

		if sf.cfg.AuthFile != "" {
			n, err := sf.basicAuthCenter.LoadFromFile(sf.cfg.AuthFile)
			if err != nil {
				sf.log.Warnf("load auth-file %v", err)
			}
			sf.log.Infof("auth data added from file %d , total:%d", n, sf.basicAuthCenter.Total())
		}
	}

	if len(sf.cfg.Parent) > 0 {
		// init filters
		sf.filters = filter.New(sf.cfg.Intelligent,
			filter.WithTimeout(sf.cfg.Timeout),
			filter.WithLivenessPeriod(sf.cfg.Interval),
			filter.WithGPool(sword.GPool),
			filter.WithLogger(sf.log))
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
			_addr, weight := _addrInfo[0], 1

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
				Timeout:          sf.cfg.LoadBalanceTimeout,
				Period:           sf.cfg.LoadBalanceRetryTime,
			})
		}
		sf.lb = loadbalance.New(sf.cfg.LoadBalanceMethod, configs,
			loadbalance.WithDNSServer(sf.domainResolver),
			loadbalance.WithLogger(sf.log),
			loadbalance.WithEnableDebug(sf.cfg.Debug),
			loadbalance.WithGPool(sf.goPool),
		)
	}
	// init ssh connect
	if sf.cfg.ParentType == "ssh" {
		sshClient, err := sf.dialSSH(sf.resolve(sf.lb.Select("")))
		if err != nil {
			return fmt.Errorf("dial ssh fail, %s", err)
		}
		sf.sshClient.Store(sshClient)
		sf.goPool.Go(func() {
			sf.log.Debugf("[ Socks ] ssh keepalive started")
			t := time.NewTicker(time.Second * 10)
			defer func() {
				t.Stop()
				sf.log.Debugf("[ Socks ] ssh keepalive stopped")
			}()

			//循环检查ssh网络连通性
			for {
				address := sf.resolve(sf.lb.Select(""))
				conn, err := net.DialTimeout("tcp", address, sf.cfg.Timeout*2)
				if err != nil {
					sf.sshClient.Load().(*ssh.Client).Close()
					sf.log.Infof("[ Socks ] ssh disconnect, retrying...")
					sshClient, e := sf.dialSSH(address)
					if e != nil {
						sf.log.Infof("[ Socks ] ssh reconnect failed")
					} else {
						sf.log.Infof("[ Socks ] ssh reconnect success")
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
		sf.log.Warnf("[ Socks ] socks udp not supported for ssh")
	}
	return
}

func (sf *Socks) Start() (err error) {
	sf.ctx, sf.cancel = context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			sf.Stop()
		}
	}()
	if err = sf.inspectConfig(); err != nil {
		return err
	}
	if err = sf.initService(); err != nil {
		return err
	}

	var opts []socks5.Option

	if sf.basicAuthCenter != nil {
		opts = append(opts, socks5.WithCredential(&Credential{sf.basicAuthCenter}))
	}
	opts = append(opts,
		socks5.WithConnectHandle(sf.proxyTCP),
		socks5.WithGPool(sword.AntsPool))
	sf.socks5Srv = socks5.NewServer(opts...)

	srv := ccs.Server{
		Protocol: sf.cfg.LocalType,
		Addr:     sf.cfg.Local,
		Config: ccs.Config{
			TCPTlsConfig: sf.cfg.tcpTlsConfig,
			StcpConfig:   sf.cfg.STCPConfig,
			KcpConfig:    sf.cfg.SKCPConfig.KcpConfig,
		},
		GoPool:      sf.goPool,
		AfterChains: cs.AdornConnsChain{cs.AdornCsnappy(sf.cfg.LocalCompress)},
		Handler:     cs.HandlerFunc(sf.handle),
	}

	var errChan <-chan error
	sf.channel, errChan = srv.RunListenAndServe()
	if err = <-errChan; err != nil {
		return
	}

	if len(sf.cfg.Parent) > 0 {
		sf.log.Infof("[ Socks ] use parent %s < %v [ %s ] >", sf.cfg.ParentType, sf.cfg.Parent, strings.ToUpper(sf.cfg.LoadBalanceMethod))
	}
	sf.log.Infof("[ Socks ] use proxy %s on %s", sf.cfg.LocalType, sf.channel.LocalAddr())
	return
}

func (sf *Socks) Stop() {
	if sf.cancel != nil {
		sf.cancel()
	}
	if sf.channel != nil {
		sf.channel.Close()
	}
	if sf.lb != nil {
		sf.lb.Close()
	}
	if sf.filters != nil {
		sf.filters.Close()
	}
	if sf.cfg.ParentType == "ssh" {
		sf.sshClient.Load().(*ssh.Client).Close()
	}
	for _, c := range sf.userConns.Items() {
		c.(io.Closer).Close()
	}
	for _, c := range sf.udpRelatedPacketConns.Items() {
		c.(io.Closer).Close()
	}
	sf.log.Infof("[ Socks ] service stopped")
}

func (sf *Socks) handle(inConn net.Conn) {
	if sf.cfg.LocalKey != "" {
		inConn = ccrypt.New(inConn, ccrypt.Config{Password: sf.cfg.LocalKey})
	}

	if err := sf.socks5Srv.ServeConn(inConn); err != nil {
		sf.log.Errorf("[ Socks ] server conn failed, %v", err)
	}
}

func (sf *Socks) proxyTCP(ctx context.Context, writer io.Writer, request *socks5.Request) error {
	// Attempt to connect
	targetConn, lbAddr, err := sf.dialForTcp(ctx, request)
	if err != nil {
		msg := err.Error()
		resp := statute.RepHostUnreachable
		if strings.Contains(msg, "refused") {
			resp = statute.RepConnectionRefused
		} else if strings.Contains(msg, "network is unreachable") {
			resp = statute.RepNetworkUnreachable
		}
		if err := socks5.SendReply(writer, resp, nil); err != nil {
			return fmt.Errorf("failed to send reply, %v", err)
		}
		return fmt.Errorf("connect to %v failed, %v", request.RawDestAddr, err)
	}
	defer targetConn.Close()

	// Send success
	if err = socks5.SendReply(writer, statute.RepSuccess, targetConn.LocalAddr()); err != nil {
		return fmt.Errorf("failed to send reply, %v", err)
	}

	if sf.cfg.rateLimit > 0 {
		targetConn = ciol.New(targetConn, ciol.WithReadLimiter(sf.cfg.rateLimit))
	}

	srcAddr := request.RemoteAddr.String()
	targetAddr := request.DestAddr.String()

	sf.userConns.Upsert(srcAddr, writer, func(exist bool, valueInMap, newValue interface{}) interface{} {
		if exist {
			valueInMap.(io.Closer).Close()
		}
		return newValue
	})
	if len(sf.cfg.Parent) > 0 {
		sf.lb.ConnsIncrease(lbAddr)
	}
	sf.log.Infof("[ Socks ] tcp %s --> %s connected", srcAddr, targetAddr)

	defer func() {
		sf.log.Infof("[ Socks ] tcp %s --> %s released", srcAddr, targetAddr)
		sf.userConns.Remove(srcAddr)
		if len(sf.cfg.Parent) > 0 {
			sf.lb.ConnsDecrease(lbAddr)
		}
	}()

	// start proxying
	eCh1 := make(chan error, 1)
	eCh2 := make(chan error, 1)
	sf.goPool.Go(func() { eCh1 <- sf.socks5Srv.Proxy(targetConn, request.Reader) })
	sf.goPool.Go(func() { eCh2 <- sf.socks5Srv.Proxy(writer, targetConn) })
	// Wait
	select {
	case err = <-eCh1:
	case err = <-eCh2:
	}
	return err
}

func (sf *Socks) dialForTcp(ctx context.Context, request *socks5.Request) (conn net.Conn, lbAddr string, err error) {
	srcAddr := request.RemoteAddr.String()
	localAddr := request.LocalAddr.String()
	targetAddr := request.DestAddr.String()

	if sf.IsDeadLoop(localAddr, targetAddr) {
		sf.log.Errorf("[ Socks ] dead loop detected , %s", targetAddr)
		return nil, "", errors.New("dead loop")
	}

	useProxy := sf.isUseProxy(targetAddr)
	if useProxy {
		boff := backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Second), 5)
		boff = backoff.WithContext(boff, sf.ctx)
		err = backoff.Retry(func() error {
			realAuth := sf.proxyAuth(proxy.Auth{
				User:     request.AuthContext.Payload["username"],
				Password: request.AuthContext.Payload["password"],
			}, false)

			socksAddr := targetAddr
			if sf.cfg.ParentType != "ssh" {
				selectAddr := srcAddr
				if sf.cfg.LoadBalanceMethod == "hash" && sf.cfg.LoadBalanceHashTarget {
					selectAddr = targetAddr
				}
				lbAddr = sf.lb.Select(selectAddr)
				socksAddr = lbAddr
			}

			dial := cs.Socks5{
				ProxyHost: socksAddr,
				Auth:      realAuth,
				Timeout:   sf.cfg.Timeout,
				Forward:   direct{sf},
			}
			conn, err = dial.Dial("tcp", targetAddr)
			sf.log.Errorf("[ Socks ] dial conn fail, %v, retrying...", err)
			return err
		}, boff)
	} else {
		conn, err = sf.dialDirect(sf.resolve(targetAddr), localAddr)
	}
	if err != nil {
		sf.log.Warnf("[ Socks ] dial conn fail, %v", err)
		return nil, "", err
	}
	if useProxy && sf.cfg.ParentKey != "" {
		conn = ccrypt.New(conn, ccrypt.Config{Password: sf.cfg.ParentKey})
	}
	method := "DIRECT"
	if useProxy {
		method = "PROXY"
	}
	sf.log.Infof("[ Socks ] %s use %s", targetAddr, method)
	return conn, lbAddr, nil
}

func (sf *Socks) IsDeadLoop(inLocalAddr string, outAddr string) bool {
	inIP, inPort, err := net.SplitHostPort(inLocalAddr)
	if err != nil {
		return false
	}
	outDomain, outPort, err := net.SplitHostPort(outAddr)
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

// 解析domain
func (sf *Socks) resolve(address string) string {
	if sf.domainResolver != nil {
		return sf.domainResolver.MustResolve(address)
	}
	return address
}

func (sf *Socks) dialParent(targetAddr string) (outConn net.Conn, err error) {
	switch sf.cfg.ParentType {
	case "tcp", "tls", "stcp", "kcp":
		d := ccs.Dialer{
			Protocol: sf.cfg.ParentType,
			Timeout:  sf.cfg.Timeout,
			Config: ccs.Config{
				TCPTlsConfig: sf.cfg.tcpTlsConfig,
				StcpConfig:   sf.cfg.STCPConfig,
				KcpConfig:    sf.cfg.SKCPConfig.KcpConfig,
			},
			AfterChains: cs.AdornConnsChain{cs.AdornCsnappy(sf.cfg.ParentCompress)},
		}
		outConn, err = d.Dial("tcp", targetAddr)
	case "ssh":
		t := time.NewTimer(sf.cfg.Timeout * 2)
		defer t.Stop()

		boff := backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Second), 1)
		boff = backoff.WithContext(boff, sf.ctx)
		err = backoff.Retry(func() (er error) {
			sshClient := sf.sshClient.Load().(*ssh.Client)
			wait := make(chan struct{}, 1)
			sword.Go(func() {
				outConn, er = sshClient.Dial("tcp", targetAddr)
				wait <- struct{}{}
			})

			t.Reset(sf.cfg.Timeout * 2)
			select {
			case <-sf.ctx.Done():
				return backoff.Permanent(io.ErrClosedPipe)
			case <-t.C:
				er = fmt.Errorf("ssh dial conn %s timeout", targetAddr)
			case <-wait:
			}
			if er != nil {
				sf.log.Errorf("[ Socks ] ssh dial conn, %v", er)
			}
			return er
		}, boff)
	default:
		return nil, fmt.Errorf("not support parent type: %s", sf.cfg.ParentType)
	}
	return
}

// 直连
func (sf *Socks) dialDirect(address string, localAddr string) (conn net.Conn, err error) {
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

func (sf *Socks) dialSSH(lAddr string) (*ssh.Client, error) {
	return ssh.Dial("tcp", lAddr, &ssh.ClientConfig{
		User:    sf.cfg.SSHUser,
		Auth:    []ssh.AuthMethod{sf.cfg.sshAuthMethod},
		Timeout: sf.cfg.Timeout,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	})
}

// 选择使用的代理授权账号密码
func (sf *Socks) proxyAuth(auth proxy.Auth, fromSS bool) *proxy.Auth {
	if sf.cfg.parentAuth != nil {
		return sf.cfg.parentAuth
	}
	if !fromSS && sf.basicAuthCenter != nil && auth.User != "" && auth.Password != "" {
		return &auth
	}
	return nil
}

func (sf *Socks) isUseProxy(addr string) bool {
	if len(sf.cfg.Parent) > 0 {
		host, _, _ := net.SplitHostPort(addr)
		if extnet.IsDomain(host) && sf.cfg.Always || !extnet.IsIntranet(host) {
			if sf.cfg.Always {
				return true
			}
			useProxy, isInMap, _, _ := sf.filters.IsProxy(addr)
			if !isInMap {
				sf.filters.Add(addr, sf.resolve(addr))
			}
			return useProxy
		}
	}
	return false
}

type Credential struct {
	basicAuthCenter *basicAuth.Center
}

func (sf *Credential) Valid(user, password, userAddr string) bool {
	return sf.basicAuthCenter.VerifyFromLocal(user, password)
}

type direct struct {
	socks *Socks
}

func (sf direct) Dial(network string, addr string) (net.Conn, error) {
	return sf.socks.dialParent(addr)
}
