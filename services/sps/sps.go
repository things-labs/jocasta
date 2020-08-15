package sps

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	cmap "github.com/orcaman/concurrent-map"
	"github.com/thinkgos/meter"
	"golang.org/x/net/proxy"
	"golang.org/x/time/rate"

	"github.com/thinkgos/jocasta/connection/cbuffered"
	"github.com/thinkgos/jocasta/connection/ccrypt"
	"github.com/thinkgos/jocasta/connection/ciol"
	"github.com/thinkgos/jocasta/connection/shadowsocks"
	"github.com/thinkgos/jocasta/connection/sni"
	"github.com/thinkgos/jocasta/core/basicAuth"
	"github.com/thinkgos/jocasta/core/idns"
	"github.com/thinkgos/jocasta/core/loadbalance"
	"github.com/thinkgos/jocasta/core/socks5"
	"github.com/thinkgos/jocasta/cs"
	"github.com/thinkgos/jocasta/lib/cert"
	"github.com/thinkgos/jocasta/lib/extnet"
	"github.com/thinkgos/jocasta/lib/logger"
	"github.com/thinkgos/jocasta/lib/outil"
	"github.com/thinkgos/jocasta/pkg/httpc"
	"github.com/thinkgos/jocasta/pkg/sword"
	"github.com/thinkgos/jocasta/services"
	"github.com/thinkgos/jocasta/services/ccs"
)

type Config struct {
	// parent
	ParentType      string   // 父级协议, tls|tcp|kcp,default empty
	Parent          []string // 父级地址,格式addr:port, default empty
	ParentCompress  bool
	ParentKey       string
	ParentAuth      string
	ParentTLSSingle bool
	// local
	LocalType     string // 本地协议, tls|tcp|kcp, default tcp
	Local         string // 本地监听地址, 格式addr:port,多个以','分隔 default :28080
	LocalCompress bool
	LocalKey      string
	// tls有效
	CertFile   string // cert文件名 default proxy.crt
	KeyFile    string // key文件名 default proxy.key
	CaCertFile string // ca文件名 default empty
	// kcp有效
	SKCPConfig ccs.SKCPConfig
	// stcp有效
	STCPMethod   string
	STCPPassword string
	// 其它
	Timeout time.Duration // tcp连接到父级或真实服务器超时时间,default 2000 单位ms
	// basic auth配置
	AuthFile       string
	Auth           []string
	AuthURL        string
	AuthURLOkCode  int
	AuthURLTimeout time.Duration
	AuthURLRetry   uint
	// dns域名解析
	DNSAddress string
	DNSTTL     int
	// 负载均衡
	LoadBalanceMethod     string
	LoadBalanceTimeout    time.Duration
	LoadBalanceRetryTime  time.Duration
	LoadBalanceHashTarget bool
	LoadBalanceOnlyHA     bool

	ParentServiceType string
	ParentSSMethod    string
	ParentSSKey       string
	SSMethod          string
	SSKey             string
	DisableHTTP       bool
	DisableSocks5     bool
	DisableSS         bool

	RateLimit string
	LocalIPS  []string
	Jumper    string
	Debug     bool

	cert      []byte
	key       []byte
	caCert    []byte
	rateLimit rate.Limit
}
type SPS struct {
	cfg                   Config
	domainResolver        *idns.Resolver
	basicAuthCenter       *basicAuth.Center
	serverChannels        []cs.Server
	userConns             cmap.ConcurrentMap
	localCipher           *shadowsocks.Cipher
	parentCipher          *shadowsocks.Cipher
	udpRelatedPacketConns cmap.ConcurrentMap
	lb                    *loadbalance.Group
	udpLocalKey           []byte
	udpParentKey          []byte
	jumper                *cs.Jumper
	parentAuthData        *sync.Map
	parentCipherData      *sync.Map
	log                   logger.Logger
}

var _ services.Service = (*SPS)(nil)

func New(log logger.Logger, cfg Config) *SPS {
	return &SPS{
		cfg:                   cfg,
		serverChannels:        make([]cs.Server, 0),
		userConns:             cmap.New(),
		udpRelatedPacketConns: cmap.New(),
		parentAuthData:        &sync.Map{},
		parentCipherData:      &sync.Map{},
		log:                   log,
	}
}
func (sf *SPS) InspectConfig() (err error) {
	if len(sf.cfg.Parent) == 1 && (sf.cfg.Parent)[0] == "" {
		(sf.cfg.Parent) = []string{}
	}

	if len(sf.cfg.Parent) == 0 {
		return fmt.Errorf("parent required for %s %s", sf.cfg.LocalType, sf.cfg.Local)
	}
	if sf.cfg.ParentType == "" {
		return fmt.Errorf("parent type unkown,use -T <tls|tcp|kcp>")
	}
	if sf.cfg.ParentType == "ss" && (sf.cfg.ParentSSKey == "" || sf.cfg.ParentSSMethod == "") {
		return fmt.Errorf("ss parent need a ss key, set it by : -J <sskey>")
	}
	if sf.cfg.ParentType == "tls" || sf.cfg.LocalType == "tls" {
		if !sf.cfg.ParentTLSSingle {
			sf.cfg.cert, sf.cfg.key, err = cert.Parse(sf.cfg.CertFile, sf.cfg.KeyFile)
			if err != nil {
				return
			}
		}
		if sf.cfg.CaCertFile != "" {
			sf.cfg.caCert, err = ioutil.ReadFile(sf.cfg.CaCertFile)
			if err != nil {
				return fmt.Errorf("read ca file error,ERR:%s", err)
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
	sf.udpLocalKey = sf.LocalUDPKey()
	sf.udpParentKey = sf.ParentUDPKey()
	if sf.cfg.Jumper != "" {
		if sf.cfg.ParentType != "tls" && sf.cfg.ParentType != "tcp" {
			return fmt.Errorf("jumper only worked of -T is tls or tcp")
		}
		sf.jumper, err = cs.NewJumper(sf.cfg.Jumper)
		if err != nil {
			return fmt.Errorf("new jumper, %s", err)
		}
	}
	return
}
func (sf *SPS) InitService() (err error) {
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
			sf.log.Debugf("auth from url [%s]", sf.cfg.AuthURL)
		}
		sf.basicAuthCenter = basicAuth.New(opts...)

		n := sf.basicAuthCenter.Add(sf.cfg.Auth...)
		sf.log.Debugf("auth data added %d, total:%d", n, sf.basicAuthCenter.Total())

		if sf.cfg.AuthFile != "" {
			n, err := sf.basicAuthCenter.LoadFromFile(sf.cfg.AuthFile)
			if err != nil {
				sf.log.Warnf("load auth-file %v", err)
			}
			sf.log.Infof("auth data added from file %d , total:%d", n, sf.basicAuthCenter.Total())
		}
	}
	// init lb
	if len(sf.cfg.Parent) > 0 {
		configs := []loadbalance.Config{}

		for _, addr := range sf.cfg.Parent {
			var _addrInfo []string
			if strings.Contains(addr, "#") {
				_s := addr[:strings.Index(addr, "#")]
				_auth, err := outil.Base64DecodeString(_s)
				if err != nil {
					sf.log.Errorf("decoding parent auth data [ %s ] fail , error : %s", _s, err)
					return err
				}
				_addrInfo = strings.Split(addr[strings.Index(addr, "#")+1:], "@")
				if sf.cfg.ParentServiceType == "ss" {
					_s := strings.Split(_auth, ":")
					m := _s[0]
					k := _s[1]
					if m == "" {
						m = sf.cfg.ParentSSMethod
					}
					if k == "" {
						k = sf.cfg.ParentSSKey
					}
					cipher, err := shadowsocks.NewCipher(m, k)
					if err != nil {
						sf.log.Errorf("error generating cipher, ssMethod: %s, ssKey: %s, error : %s", m, k, err)
						return err
					}
					sf.parentCipherData.Store(_addrInfo[0], cipher)
				} else {
					sf.parentAuthData.Store(_addrInfo[0], _auth)
				}

			} else {
				_addrInfo = strings.Split(addr, "@")
			}
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
				Period:           sf.cfg.LoadBalanceRetryTime,
				Timeout:          sf.cfg.LoadBalanceTimeout,
			})
		}
		sf.lb = loadbalance.NewGroup(sf.cfg.LoadBalanceMethod, configs,
			loadbalance.WithDNSServer(sf.domainResolver),
			loadbalance.WithLogger(sf.log),
			loadbalance.WithEnableDebug(sf.cfg.Debug),
			loadbalance.WithGPool(sword.GPool),
		)
	}

	if sf.cfg.SSMethod != "" && sf.cfg.SSKey != "" {
		sf.localCipher, err = shadowsocks.NewCipher(sf.cfg.SSMethod, sf.cfg.SSKey)
		if err != nil {
			sf.log.Errorf("error generating cipher : %s", err)
			return
		}
	}
	if sf.cfg.ParentServiceType == "ss" {
		sf.parentCipher, err = shadowsocks.NewCipher(sf.cfg.ParentSSMethod, sf.cfg.ParentSSKey)
		if err != nil {
			sf.log.Errorf("error generating cipher : %s", err)
			return
		}
	}
	return
}

func (sf *SPS) Start() (err error) {
	if err = sf.InspectConfig(); err != nil {
		return
	}
	if err = sf.InitService(); err != nil {
		return
	}

	sf.log.Infof("use %s %s parent %v [ %s ]", sf.cfg.ParentType, sf.cfg.ParentServiceType, sf.cfg.Parent, strings.ToUpper(sf.cfg.LoadBalanceMethod))
	for _, addr := range strings.Split(sf.cfg.Local, ",") {
		if addr != "" {
			srv := ccs.Server{
				Protocol: sf.cfg.LocalType,
				Addr:     addr,
				Config: ccs.Config{
					Cert:         sf.cfg.cert,
					Key:          sf.cfg.key,
					CaCert:       sf.cfg.caCert,
					SingleTLS:    false,
					STCPMethod:   sf.cfg.STCPMethod,
					STCPPassword: sf.cfg.STCPPassword,
					KcpConfig:    sf.cfg.SKCPConfig.KcpConfig,
					Compress:     sf.cfg.LocalCompress,
				},
				GoPool:  sword.GPool,
				Handler: cs.HandlerFunc(sf.handle),
			}

			sc, errChan := srv.RunListenAndServe()
			if err = <-errChan; err != nil {
				return
			}
			sf.serverChannels = append(sf.serverChannels, sc)
			if sf.cfg.ParentServiceType == "socks" {
				err = sf.RunSSUDP(addr)
			} else {
				sf.log.Warnf("warn : udp only for socks parent ")
			}
			if err != nil {
				return
			}
			sf.log.Infof("%s http(s)+socks+ss proxy on %s", sf.cfg.LocalType, sc.LocalAddr())
		}
	}
	return
}

func (sf *SPS) Stop() {
	for _, sc := range sf.serverChannels {
		if sc != nil {
			sc.Close()
		}
	}
	for _, c := range sf.userConns.Items() {
		if _, ok := c.(net.Conn); ok {
			c.(net.Conn).Close()
		}
	}
	if sf.lb != nil {
		sf.lb.Close()
	}
	for _, c := range sf.udpRelatedPacketConns.Items() {
		c.(*net.UDPConn).Close()
	}
	sf.log.Infof("service sps stopped")
}
func (sf *SPS) handle(inConn net.Conn) {
	defer inConn.Close()

	if sf.cfg.LocalKey != "" {
		inConn = ccrypt.New(inConn, ccrypt.Config{Password: sf.cfg.LocalKey})
	}
	var err error

	switch sf.cfg.ParentType {
	case "tcp", "stcp", "tls", "kcp":
		err = sf.proxyTCP(inConn)
	default:
		err = fmt.Errorf("unkown parent type %s", sf.cfg.ParentType)
	}
	if err != nil {
		sf.log.Errorf("connect to %s fail, %s from %s", sf.cfg.ParentType, err, inConn.RemoteAddr())
	}
}
func (sf *SPS) proxyTCP(inConn net.Conn) (err error) {
	enableUDP := sf.cfg.ParentServiceType == "socks"
	udpIP, _, _ := net.SplitHostPort(inConn.LocalAddr().String())
	if len(sf.cfg.LocalIPS) > 0 {
		udpIP = (sf.cfg.LocalIPS)[0]
	}
	bInConn := cbuffered.New(inConn)
	//important
	//action read will regist read event to system,
	//when data arrived , system call process
	//so that we can get buffered bytes count
	//otherwise Buffered() always return 0
	bInConn.ReadByte()
	bInConn.UnreadByte()

	n := 2048
	if n > bInConn.Buffered() {
		n = bInConn.Buffered()
	}
	h, err := bInConn.Peek(n)
	if err != nil {
		sf.log.Errorf("peek error %s ", err)
		return
	}
	isSNI, _ := sni.ServerNameFromBytes(h)
	inConn = bInConn
	address := ""
	var auth = proxy.Auth{}
	var forwardBytes []byte

	if extnet.IsSocks5(h) {
		if sf.cfg.DisableSocks5 {
			return
		}
		//socks5 server
		serverConn := socks5.NewServer(inConn, sf.cfg.Timeout, sf.basicAuthCenter, enableUDP, udpIP, nil)

		if err = serverConn.Handshake(); err != nil {
			return
		}
		address = serverConn.Target()
		auth = serverConn.AuthData()
		if serverConn.IsUDP() {
			sf.proxyUDP(inConn, serverConn)
			return
		}
	} else if extnet.IsHTTP(h) || isSNI != "" {
		if sf.cfg.DisableHTTP {
			return
		}
		//http
		var request httpc.Request
		err = extnet.WrapTimeout(inConn, sf.cfg.Timeout, func(c net.Conn) (err1 error) {
			request, err1 = httpc.New(inConn, 1024,
				httpc.WithBasicAuth(sf.basicAuthCenter),
				httpc.WithLogger(sf.log),
			)
			return
		})
		if err != nil {
			sf.log.Errorf("new http request fail,ERR: %s", err)
			return
		}
		if len(h) >= 7 && strings.ToLower(string(h[:7])) == "connect" {
			//https
			request.HTTPSReply()
			//s.log.Printf("https reply: %s", request.Host)
		} else {
			forwardBytes = request.RawHeader
		}
		address = request.Host
		var userpass string
		if sf.basicAuthCenter != nil {
			userpass, err = request.GetProxyAuthUserPassPair()
			if err != nil {
				return
			}
			userpassA := strings.Split(userpass, ":")
			if len(userpassA) == 2 {
				auth = proxy.Auth{User: userpassA[0], Password: userpassA[1]}
			}
		}
	} else {
		//ss
		if sf.cfg.DisableSS {
			return
		}
		var ssConn *shadowsocks.Conn
		err = extnet.WrapTimeout(inConn, time.Second*5, func(c net.Conn) error {
			var err error
			ssConn = shadowsocks.New(inConn, sf.localCipher.Clone())
			address, err = shadowsocks.ParseRequest(ssConn)
			return err
		})
		if err != nil {
			return
		}
		// ensure the host does not contain some illegal characters, NUL may panic on Win32
		if strings.ContainsRune(address, 0x00) {
			err = errors.New("invalid domain name")
			return
		}
		inConn = ssConn
	}
	if err != nil || address == "" {
		sf.log.Errorf("unknown request from: %s,%s", inConn.RemoteAddr(), string(h))
		err = errors.New("unknown request")
		return
	}
	//connect to parent
	var outConn net.Conn
	selectAddr := inConn.RemoteAddr().String()
	if sf.cfg.LoadBalanceMethod == "hash" && sf.cfg.LoadBalanceHashTarget {
		selectAddr = address
	}
	lbAddr := sf.lb.Select(selectAddr, sf.cfg.LoadBalanceOnlyHA)
	outConn, err = sf.dialParent(lbAddr)
	if err != nil {
		sf.log.Errorf("connect to %s , err:%s", lbAddr, err)
		return
	}
	ParentAuth := sf.getParentAuth(lbAddr)
	if ParentAuth != "" || sf.cfg.ParentSSKey != "" || sf.basicAuthCenter != nil {
		forwardBytes = extnet.RemoveProxyHeaders(forwardBytes)
	}

	//ask parent for connect to target address
	switch sf.cfg.ParentServiceType {
	case "http":
		//http parent
		isHTTPS := false

		pb := new(bytes.Buffer)
		if len(forwardBytes) == 0 {
			isHTTPS = true
			pb.Write([]byte(fmt.Sprintf("CONNECT %s HTTP/1.1\r\n", address)))
		}
		pb.WriteString(fmt.Sprintf("Host: %s\r\n", address))
		pb.WriteString(fmt.Sprintf("Proxy-Host: %s\r\n", address))
		pb.WriteString("Proxy-Connection: Keep-Alive\r\n")
		pb.WriteString("Connection: Keep-Alive\r\n")

		u := ""
		if ParentAuth != "" {
			a := strings.Split(ParentAuth, ":")
			if len(a) != 2 {
				err = fmt.Errorf("parent auth data format error")
				return
			}
			u = fmt.Sprintf("%s:%s", a[0], a[1])
		} else {
			if sf.basicAuthCenter == nil && auth.Password != "" && auth.User != "" {
				u = fmt.Sprintf("%s:%s", auth.User, auth.Password)
			}
		}
		if u != "" {
			pb.Write([]byte(fmt.Sprintf("Proxy-Authorization: Basic %s\r\n", base64.StdEncoding.EncodeToString([]byte(u)))))
		}

		if isHTTPS {
			pb.Write([]byte("\r\n"))
		} else {
			forwardBytes = extnet.InsertProxyHeaders(forwardBytes, pb.String())
			pb.Reset()
			pb.Write(forwardBytes)
			forwardBytes = nil
		}

		err = extnet.WrapWriteTimeout(outConn, sf.cfg.Timeout, func(c net.Conn) error {
			_, err := c.Write(pb.Bytes())
			return err
		})
		if err != nil {
			outConn.Close()
			sf.log.Errorf("write CONNECT to %s , err:%s", lbAddr, err)
			return
		}

		if isHTTPS {
			reply := make([]byte, 1024)
			err = extnet.WrapReadTimeout(outConn, sf.cfg.Timeout, func(c net.Conn) error {
				_, err := c.Read(reply)
				return err
			})
			if err != nil {
				outConn.Close()
				sf.log.Errorf("read reply from %s , err:%s", lbAddr, err)
				return
			}
			//s.log.Printf("reply: %s", string(reply[:n]))
		}
	case "socks":
		sf.log.Infof("connect %s", address)

		//socks client
		_, err = sf.HandshakeSocksParent(ParentAuth, outConn, "tcp", address, auth, false)
		if err != nil {
			sf.log.Errorf("handshake fail, %s", err)
			return
		}
	case "ss":
		ra, e := shadowsocks.ParseAddrSpec(address)
		if e != nil {
			err = fmt.Errorf("build ss raw addr fail, err: %s", e)
			return
		}

		outConn, err = shadowsocks.NewConnWithRawAddr(outConn, ra, sf.getParentCipher(lbAddr))
		if err != nil {
			err = fmt.Errorf("dial ss parent fail, err : %s", err)
			return
		}
	default:
		return errors.New("not support")
	}
	//forward client data to target,if necessary.
	if len(forwardBytes) > 0 {
		outConn.Write(forwardBytes)
	}

	if sf.cfg.rateLimit > 0 {
		outConn = ciol.New(outConn, ciol.WithReadLimiter(sf.cfg.rateLimit))
	}

	//bind
	inAddr := inConn.RemoteAddr().String()
	outAddr := outConn.RemoteAddr().String()

	sf.userConns.Upsert(inAddr, inConn, func(exist bool, valueInMap interface{}, newValue interface{}) interface{} {
		if exist {
			valueInMap.(net.Conn).Close()
		}
		return newValue
	})
	sf.lb.ConnsIncrease(lbAddr)
	sf.log.Infof("conn %s - %s connected [%s]", inAddr, outAddr, address)

	defer func() {
		sf.log.Infof("conn %s - %s released [%s]", inAddr, outAddr, address)
		sf.userConns.Remove(inAddr)
		sf.lb.ConnsDecrease(lbAddr)
	}()
	return sword.Binding.Proxy(inConn, outConn)
}

func (sf *SPS) getParentAuth(lbAddr string) string {
	if v, ok := sf.parentAuthData.Load(lbAddr); ok {
		return v.(string)
	}
	return sf.cfg.ParentAuth
}

func (sf *SPS) getParentCipher(lbAddr string) *shadowsocks.Cipher {
	if v, ok := sf.parentCipherData.Load(lbAddr); ok {
		return v.(*shadowsocks.Cipher).Clone()
	}
	return sf.parentCipher.Clone()
}

func (sf *SPS) buildRequest(address string) (buf []byte, err error) {
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		err = errors.New("proxy: failed to parse port number: " + portStr)
		return
	}
	if port < 1 || port > 0xffff {
		err = errors.New("proxy: port number out of range: " + portStr)
		return
	}
	buf = buf[:0]
	buf = append(buf, 0x05, 0x01, 0 /* reserved */)

	if ip := net.ParseIP(host); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			buf = append(buf, 0x01)
			ip = ip4
		} else {
			buf = append(buf, 0x04)
		}
		buf = append(buf, ip...)
	} else {
		if len(host) > 255 {
			err = errors.New("proxy: destination host name too long: " + host)
			return
		}
		buf = append(buf, 0x03)
		buf = append(buf, byte(len(host)))
		buf = append(buf, host...)
	}
	buf = append(buf, byte(port>>8), byte(port))
	return
}

func (sf *SPS) resolve(address string) string {
	if sf.domainResolver != nil {
		return sf.domainResolver.MustResolve(address)
	}
	return address
}

func (sf *SPS) dialParent(address string) (net.Conn, error) {
	d := ccs.Dialer{
		Protocol: sf.cfg.ParentType,
		Config: ccs.Config{
			Cert:         sf.cfg.cert,
			Key:          sf.cfg.key,
			CaCert:       sf.cfg.caCert,
			KcpConfig:    sf.cfg.SKCPConfig.KcpConfig,
			STCPMethod:   sf.cfg.STCPMethod,
			STCPPassword: sf.cfg.STCPPassword,
			Compress:     sf.cfg.ParentCompress,
			Jumper:       sf.jumper,
		},
	}
	conn, err := d.DialTimeout(address, sf.cfg.Timeout)
	if err != nil {
		return nil, err
	}

	if sf.cfg.ParentKey != "" {
		conn = ccrypt.New(conn, ccrypt.Config{Password: sf.cfg.ParentKey})
	}
	return conn, nil
}
func (sf *SPS) HandshakeSocksParent(parentAuth string, outconn net.Conn, network, dstAddr string, auth proxy.Auth, fromSS bool) (client *socks5.Client, err error) {
	var realAuth *proxy.Auth

	if parentAuth != "" {
		a := strings.Split(parentAuth, ":")
		if len(a) != 2 {
			err = fmt.Errorf("parent auth data format error")
			return
		}
		realAuth = &proxy.Auth{User: a[0], Password: a[1]}
	} else {
		if !fromSS && sf.basicAuthCenter == nil && auth.Password != "" && auth.User != "" {
			realAuth = &auth
		}
	}
	client = socks5.NewClient(outconn, network, dstAddr, sf.cfg.Timeout, realAuth, nil)
	err = client.Handshake()
	return
}
