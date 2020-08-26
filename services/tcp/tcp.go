package tcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	"github.com/thinkgos/go-core-package/extcert"
	"github.com/thinkgos/go-core-package/extnet"
	"github.com/thinkgos/go-core-package/lib/encrypt"
	"github.com/thinkgos/go-core-package/lib/logger"
	"github.com/thinkgos/strext"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"github.com/thinkgos/jocasta/connection"
	"github.com/thinkgos/jocasta/core/captain"
	"github.com/thinkgos/jocasta/core/idns"
	"github.com/thinkgos/jocasta/cs"
	"github.com/thinkgos/jocasta/pkg/ccs"
	"github.com/thinkgos/jocasta/pkg/enet"
	"github.com/thinkgos/jocasta/pkg/sword"
	"github.com/thinkgos/jocasta/services"
)

const defaultUDPIdleTime = 10 // 单位s

// Config config
type Config struct {
	// parent
	ParentType     string `validate:"required,oneof=tcp tls stcp kcp udp"` // 父级协议类型 tcp|tls|stcp|kcp|udp default: empty
	Parent         string // 父级地址,格式addr:port, default empty
	ParentCompress bool   // 父级支持压缩传输, default: false
	// local
	LocalType     string `validate:"required,oneof=tcp tls stcp kcp"` // 本地协议类型 tcp|tls|stcp|kcp
	Local         string // 本地监听地址 default :22800
	LocalCompress bool   // 本地支持压缩传输, default: false
	// tls有效
	CertFile   string // cert文件 default: proxy.crt
	KeyFile    string // key文件 default: proxy.key
	CaCertFile string // ca文件 default: empty
	// kcp有效
	SKCPConfig ccs.SKCPConfig
	// stcp有效
	// stcp 加密方法 default: aes-192-cfb
	// stcp 加密密钥 default: thinkgos's_jocasta
	STCPConfig cs.StcpConfig
	// 其它
	Timeout time.Duration `validate:"required"` // 连接父级或真实服务器超时时间, default: 2s
	// 通过代理, 支持tcp,tls,stcp下使用
	//      https://username:password@host:port
	//      https://host:port
	//      socks5://username:password@host:port
	//      socks5://host:port
	RawProxyURL string
	// private
	tlsConfig cs.TLSConfig
}

type connItem struct {
	conn           net.Conn
	srcAddr        *net.UDPAddr
	targetAddr     *net.UDPAddr
	targetConn     net.Conn
	lastActiveTime int64 // unix time
}

type TCP struct {
	cfg     Config
	channel net.Listener
	// parent type = "udp", udp -> udp绑定传输
	// src地址对udp连接映射
	// parent type != "udp", udp -> net.conn 其它的绑定传输
	// src地址对其它连接的绑定
	userConns   *connection.Manager
	single      singleflight.Group
	proxyURL    *url.URL
	dnsResolver *idns.Resolver
	cancel      context.CancelFunc
	ctx         context.Context
	log         logger.Logger
	udpIdleTime int64
}

var _ services.Service = (*TCP)(nil)

func New(cfg Config, opts ...Option) *TCP {
	t := &TCP{
		cfg:         cfg,
		log:         logger.NewDiscard(),
		udpIdleTime: defaultUDPIdleTime,
	}
	for _, opt := range opts {
		opt(t)
	}

	t.userConns = connection.New(time.Second,
		func(key string, value interface{}, now time.Time) bool {
			nowSeconds := now.Unix()
			item := value.(*connItem)
			if nowSeconds-atomic.LoadInt64(&item.lastActiveTime) > t.udpIdleTime {
				item.conn.Close()
				item.targetConn.Close()
				return true
			}
			return false
		})

	return t
}

func (sf *TCP) inspectConfig() (err error) {
	if err = sword.Validate.Struct(&sf.cfg); err != nil {
		return
	}

	// tls 证书检查
	if strext.Contains([]string{sf.cfg.ParentType, sf.cfg.LocalType}, "tls") {
		if sf.cfg.CertFile == "" || sf.cfg.KeyFile == "" {
			return errors.New("cert file and key file required")
		}
		if sf.cfg.tlsConfig.Cert, sf.cfg.tlsConfig.Key, err = extcert.LoadPair(sf.cfg.CertFile, sf.cfg.KeyFile); err != nil {
			return err
		}
		if sf.cfg.CaCertFile != "" {
			if sf.cfg.tlsConfig.CaCert, err = extcert.LoadCrt(sf.cfg.CaCertFile); err != nil {
				return fmt.Errorf("read ca file %+v", err)
			}
		}
	}

	// stcp 方法检查
	if strext.Contains([]string{sf.cfg.ParentType, sf.cfg.LocalType}, "stcp") &&
		!strext.Contains(encrypt.CipherMethods(), sf.cfg.STCPConfig.Method) {
		return fmt.Errorf("stcp cipher method support one of %s", strings.Join(encrypt.CipherMethods(), ","))
	}

	if sf.cfg.RawProxyURL != "" {
		if sf.proxyURL, err = cs.ParseProxyURL(sf.cfg.RawProxyURL); err != nil {
			return fmt.Errorf("new proxyURL, %+v", err)
		}
	}
	return
}

func (sf *TCP) Start() (err error) {
	sf.ctx, sf.cancel = context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			sf.cancel()
		}
	}()

	if err = sf.inspectConfig(); err != nil {
		return err
	}

	srv := ccs.Server{
		Protocol: sf.cfg.LocalType,
		Addr:     sf.cfg.Local,
		Config: ccs.Config{
			TLSConfig:  sf.cfg.tlsConfig,
			StcpConfig: sf.cfg.STCPConfig,
			KcpConfig:  sf.cfg.SKCPConfig.KcpConfig,
		},
		GoPool:      sword.GoPool,
		AfterChains: extnet.AdornConnsChain{extnet.AdornSnappy(sf.cfg.LocalCompress)},
		Handler:     cs.HandlerFunc(sf.handler),
	}
	ln, err := srv.Listen()
	if err != nil {
		return err
	}

	sword.Go(func() { srv.Server(ln) })
	sf.channel = ln

	if sf.cfg.ParentType == "udp" {
		sword.Go(func() { sf.userConns.Watch(sf.ctx) })
	}

	sf.log.Infof("[ TCP ] use parent %s< %s >", sf.cfg.Parent, sf.cfg.ParentType)
	sf.log.Infof("[ TCP ] use proxy %s on %s", sf.cfg.LocalType, sf.channel.Addr().String())
	return
}

func (sf *TCP) Stop() {
	if sf.cancel != nil {
		sf.cancel()
	}
	if sf.channel != nil {
		sf.channel.Close()
	}
	for _, c := range sf.userConns.Items() {
		if sf.cfg.ParentType == "udp" {
			item := c.(*connItem)
			item.conn.Close()
			item.targetConn.Close()
		} else {
			c.(net.Conn).Close()
		}
	}
	sf.log.Infof("[ TCP ] service stopped")
}

func (sf *TCP) handler(inConn net.Conn) {
	defer func() {
		if err := recover(); err != nil {
			sf.log.DPanicf("[ TCP ] handler", zap.Any("crashed", err), zap.ByteString("stack", debug.Stack()))
		}
	}()
	defer inConn.Close()
	switch sf.cfg.ParentType {
	case "tcp", "tls", "stcp", "kcp":
		sf.proxyStream2Stream(inConn)
	case "udp":
		sf.proxyStream2UDP(inConn)
	default:
		sf.log.Errorf("unknown parent type %s", sf.cfg.ParentType)
	}
}

func (sf *TCP) proxyStream2Stream(inConn net.Conn) {
	targetConn, err := sf.dialParent(sf.resolve(sf.cfg.Parent))
	if err != nil {
		sf.log.Errorf("[ TCP ] dial parent %s, %s", sf.cfg.Parent, err)
		return
	}

	srcAddr := inConn.RemoteAddr().String()
	targetAddr := targetConn.RemoteAddr().String()

	sf.userConns.Upsert(srcAddr, inConn, func(exist bool, valueInMap, newValue interface{}) interface{} {
		if exist {
			valueInMap.(net.Conn).Close()
		}
		return newValue
	})
	sf.log.Infof("[ TCP ] tcp %s ---> %s connected", srcAddr, targetAddr)
	defer func() {
		sf.log.Infof("[ TCP ] tcp %s ---> %s released", srcAddr, targetAddr)
		targetConn.Close()
		sf.userConns.Remove(srcAddr)
	}()

	err = sword.Binding.Proxy(inConn, targetConn)
	if err != nil && !errors.Is(err, io.EOF) && !extnet.IsErrClosed(err) {
		sf.log.Errorf("[ TCP ] proxying, %s", err)
	}
}

func (sf *TCP) proxyStream2UDP(inConn net.Conn) {
	localAddr := inConn.LocalAddr().String()

	targetAddr, err := net.ResolveUDPAddr("udp", sf.cfg.Parent)
	if err != nil {
		sf.log.Errorf("[ TCP ] resolve udp addr %s fail, %+v", sf.cfg.Parent, err)
		return
	}
	for {
		select {
		case <-sf.ctx.Done():
			return
		default:
		}
		// read client ---> write remote
		da, err := captain.ParseStreamDatagram(inConn)
		if err != nil {
			if strings.Contains(err.Error(), "n != int(") {
				continue
			}
			if !extnet.IsErrClosed(err) {
				sf.log.Warnf("[ TCP ] udp read from local conn fail, %v", err)
			}
			return
		}
		srcAddr := da.Addr.String()
		itm, err, _ := sf.single.Do(srcAddr, func() (interface{}, error) {
			if v, ok := sf.userConns.Get(srcAddr); ok {
				return v, nil
			}
			targetConn, err := net.DialUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0}, targetAddr)
			if err != nil {
				sf.log.Errorf("[ TCP ] connect target udp conn fail, %s", err)
				return nil, err
			}

			_srcAddr, _ := net.ResolveUDPAddr("udp", srcAddr)
			item := &connItem{
				conn:       inConn,
				srcAddr:    _srcAddr,
				targetAddr: targetAddr,
				targetConn: targetConn,
			}
			sf.userConns.Set(srcAddr, item)
			sword.Go(func() {
				sf.log.Infof("[ TCP ] udp conn %s ---> %s connected", srcAddr, localAddr)
				buf := sword.Binding.Get()
				defer func() {
					sword.Binding.Put(buf)
					sf.userConns.Remove(srcAddr)
					item.conn.Close()
					item.targetConn.Close()
					sf.log.Infof("[ TCP ] udp conn %s ---> %s released", srcAddr, localAddr)
				}()

				for {
					// read remote ---> write client
					n, err := item.targetConn.Read(buf[:cap(buf)])
					if err != nil {
						if !extnet.IsErrClosed(err) {
							sf.log.Warnf("[ TCP ] udp read from target conn fail, %v", err)
						}

						return
					}
					atomic.StoreInt64(&item.lastActiveTime, time.Now().Unix())
					err = enet.WrapWriteTimeout(item.conn, sf.cfg.Timeout, func(c net.Conn) error {
						as, err := captain.ParseAddrSpec(item.srcAddr.String())
						if err != nil {
							return err
						}
						sData := captain.StreamDatagram{
							Addr: as,
							Data: buf[:n],
						}
						header, err := sData.Header()
						if err != nil {
							return err
						}
						buf := sword.Binding.Get()
						sword.Binding.Put(buf)
						tmpBuf := append(buf, header...)
						tmpBuf = append(tmpBuf, sData.Data...)
						c.Write(tmpBuf) // nolint: errcheck
						return nil
					})
					if err != nil {
						sf.log.Warnf("[ TCP ] udp write to local conn fail, %v", err)
						return
					}
				}
			})
			return item, nil
		})
		if err != nil {
			return
		}

		item := itm.(*connItem)
		atomic.StoreInt64(&item.lastActiveTime, time.Now().Unix())
		_, err = item.targetConn.Write(da.Data)
		if err != nil {
			sf.log.Errorf("[ TCP ] udp write to target conn fail, %s", err)
			return
		}
	}
}

func (sf *TCP) dialParent(address string) (net.Conn, error) {
	d := ccs.Dialer{
		Protocol: sf.cfg.ParentType,
		Timeout:  sf.cfg.Timeout,
		Config: ccs.Config{
			TLSConfig:  sf.cfg.tlsConfig,
			StcpConfig: sf.cfg.STCPConfig,
			KcpConfig:  sf.cfg.SKCPConfig.KcpConfig,
			ProxyURL:   sf.proxyURL,
		},
		AfterChains: extnet.AdornConnsChain{extnet.AdornSnappy(sf.cfg.ParentCompress)},
	}
	return d.Dial("tcp", address)
}

// 解析domain
func (sf *TCP) resolve(address string) string {
	if sf.dnsResolver != nil {
		return sf.dnsResolver.MustResolve(address)
	}
	return address
}
