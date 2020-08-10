package udp

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/thinkgos/strext"
	"golang.org/x/sync/singleflight"

	"github.com/thinkgos/jocasta/connection"
	"github.com/thinkgos/jocasta/core/idns"
	"github.com/thinkgos/jocasta/cs"
	"github.com/thinkgos/jocasta/lib/cert"
	"github.com/thinkgos/jocasta/lib/encrypt"
	"github.com/thinkgos/jocasta/lib/extnet"
	"github.com/thinkgos/jocasta/lib/logger"
	"github.com/thinkgos/jocasta/pkg/captain"
	"github.com/thinkgos/jocasta/pkg/sword"
	"github.com/thinkgos/jocasta/services"
	"github.com/thinkgos/jocasta/services/ccs"
	"github.com/thinkgos/jocasta/services/skcp"
)

const MaxUDPIdleTime = 10 // 单位s

type Config struct {
	// parent
	ParentType     string `validate:"required,oneof=tcp tls stcp kcp udp"` // 父级协议,tcp|tls|stcp|kcp|udp default empty
	Parent         string // 父级地址,格式addr:port default empty
	ParentCompress bool   // default false
	// local
	Local string // 本地监听地址 default :28080
	// tls有效
	CertFile   string // cert文件名
	KeyFile    string // key文件名
	CaCertFile string // ca文件 default empty
	// kcp有效
	SKCPConfig *skcp.Config
	// stcp有效
	STCPMethod   string `validate:"required"` // default aes-192-cfb
	STCPPassword string // default thinkgos's_goproxy
	// 其它
	Timeout             time.Duration `validate:"required"` // tcp连接父级代理超时时间 单位ms
	CheckParentInterval int           // TODO: not used 确认代理是否正常间隔,0表示不检查, default 3 单位s
	// private
	cert   []byte
	key    []byte
	caCert []byte
}

type connItem struct {
	targetConn     net.Conn
	srcAddr        *net.UDPAddr
	targetAddr     *net.UDPAddr
	lastActiveTime int64
}

type UDP struct {
	cfg     Config
	channel *cs.UDP
	// parent type = "udp", udp -> udp绑定传输
	// src地址对udp连接映射
	// parent type != "udp", udp -> 其它的绑定传输
	// src地址对其它连接的绑定
	conns       *connection.Manager
	single      singleflight.Group
	gPool       sword.GoPool
	dnsResolver *idns.Resolver
	cancel      context.CancelFunc
	ctx         context.Context
	log         logger.Logger
}

var _ services.Service = (*UDP)(nil)

func New(cfg Config, opts ...Option) *UDP {
	u := &UDP{
		cfg: cfg,
		conns: connection.New(time.Second,
			func(key string, value interface{}, now time.Time) bool {
				nowSeconds := now.Unix()
				item := value.(*connItem)
				if nowSeconds-atomic.LoadInt64(&item.lastActiveTime) > MaxUDPIdleTime {
					item.targetConn.Close()
					return true
				}
				return false
			}),
		log: logger.NewDiscard(),
	}

	for _, opt := range opts {
		opt(u)
	}
	return u
}

func (sf *UDP) inspectConfig() (err error) {
	if err = sword.Validate.Struct(&sf.cfg); err != nil {
		return
	}

	if sf.cfg.ParentType == "tls" {
		sf.cfg.cert, sf.cfg.key, err = cert.Parse(sf.cfg.CertFile, sf.cfg.KeyFile)
		if err != nil {
			return
		}
		if sf.cfg.CaCertFile != "" {
			if sf.cfg.caCert, err = ioutil.ReadFile(sf.cfg.CaCertFile); err != nil {
				return fmt.Errorf("read ca file %+v", err)
			}
		}
	}

	// stcp 方法检查
	if sf.cfg.ParentType == "stcp" && !strext.Contains(encrypt.CipherMethods(), sf.cfg.STCPMethod) {
		return fmt.Errorf("stcp cipher method support one of %s", strings.Join(encrypt.CipherMethods(), ","))
	}
	return
}

func (sf *UDP) Start() (err error) {
	sf.ctx, sf.cancel = context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			sf.cancel()
		}
	}()
	if err = sf.inspectConfig(); err != nil {
		return
	}

	sf.channel, err = ccs.ListenAndServeUDP(sf.cfg.Local, sf.handle)
	if err != nil {
		return
	}

	sf.gPool.Go(func() { sf.conns.RunWatch(sf.ctx) })
	sf.log.Infof("[ UDP ] use parent %s< %s >", sf.cfg.Parent, sf.cfg.ParentType)
	sf.log.Infof("[ UDP ] use proxy udp on %s", sf.channel.Addr())
	return
}

func (sf *UDP) Stop() {
	if sf.cancel != nil {
		sf.cancel()
	}
	if sf.channel != nil {
		sf.channel.Close()
	}
	for _, c := range sf.conns.Items() {
		c.(*connItem).targetConn.Close()
	}
	sf.log.Infof("[ UDP ] service stopped")
}

func (sf *UDP) handle(ln *net.UDPConn, msg cs.Message) {
	switch sf.cfg.ParentType {
	case "tcp", "tls", "stcp", "kcp":
		sf.proxyUdp2Any(ln, msg)
	case "udp":
		sf.proxyUdp2Udp(ln, msg)
	default:
		sf.log.Errorf("[ UDP ] unknown parent type %s", sf.cfg.ParentType)
	}
}

func (sf *UDP) proxyUdp2Any(_ *net.UDPConn, msg cs.Message) {
	srcAddr := msg.SrcAddr.String()

	itm, err, _ := sf.single.Do(srcAddr, func() (interface{}, error) {
		if v, ok := sf.conns.Get(srcAddr); ok {
			return v, nil
		}
		targetConn, err := sf.dialParent(sf.resolve(sf.cfg.Parent))
		if err != nil {
			sf.log.Errorf("[ UDP ] connect to target %s fail, %s", sf.cfg.Parent, err)
			return nil, err
		}
		item := &connItem{
			targetConn: targetConn,
			srcAddr:    msg.SrcAddr,
			targetAddr: msg.LocalAddr,
		}
		sf.conns.Set(srcAddr, item)
		sword.Go(func() {
			sf.log.Infof("[ UDP ] udp conn %s ---> %s connected", srcAddr, targetConn.RemoteAddr().String())
			defer func() {
				sf.conns.Remove(srcAddr)
				item.targetConn.Close()
				sf.log.Infof("[ UDP ] udp conn %s ---> %s released", srcAddr, targetConn.RemoteAddr().String())
			}()

			for {
				da, err := captain.ParseStreamDatagram(item.targetConn)
				if err != nil {
					sf.log.Errorf("[ UDP ] udp conn read from target conn fail, %s ", err)
					if strings.Contains(err.Error(), "n != int(") {
						continue
					}
					return
				}
				atomic.StoreInt64(&item.lastActiveTime, time.Now().Unix())
				_, err = sf.channel.WriteToUDP(da.Data, item.srcAddr)
				if err != nil {
					sf.log.Errorf("[ UDP ] udp conn write to local conn fail, %s ", err)
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

	err = extnet.WrapWriteTimeout(item.targetConn, sf.cfg.Timeout, func(c net.Conn) error {
		as, err := captain.ParseAddrSpec(srcAddr)
		if err != nil {
			return err
		}
		sData := captain.StreamDatagram{
			Addr: as,
			Data: msg.Data,
		}
		header, err := sData.Header()
		if err != nil {
			return err
		}
		c.Write(header)     // nolint: errcheck
		c.Write(sData.Data) // nolint: errcheck
		return nil
	})
	if err != nil {
		sf.log.Errorf("[ UDP ] udp conn write to target conn fail, %s ", err)
	}
}

func (sf *UDP) proxyUdp2Udp(_ *net.UDPConn, msg cs.Message) {
	localAddr := msg.LocalAddr.String()
	srcAddr := msg.SrcAddr.String()
	sf.log.Debugf("[ UDP ] udp conn %s ---> %s request", srcAddr, localAddr)

	itm, err, _ := sf.single.Do(srcAddr, func() (interface{}, error) {
		if v, ok := sf.conns.Get(srcAddr); ok {
			return v, nil
		}
		targetAddr, err := net.ResolveUDPAddr("udp", sf.cfg.Parent)
		if err != nil {
			sf.log.Errorf("[ UDP ] resolve udp addr %s fail, %+v", sf.cfg.Parent, err)
			return nil, err
		}
		targetConn, err := net.DialUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0}, targetAddr)
		if err != nil {
			sf.log.Errorf("[ UDP ] connect to udp target conn %s fail, %+v", targetAddr.String(), err)
			return nil, err
		}

		item := &connItem{
			targetConn: targetConn,
			srcAddr:    msg.SrcAddr,
			targetAddr: targetAddr,
		}
		sf.conns.Set(srcAddr, item)
		sf.gPool.Go(func() {
			sf.log.Infof("[ UDP ] udp conn %s ---> %s connected", srcAddr, localAddr)
			buf := sword.Binding.Get()
			defer func() {
				sword.Binding.Put(buf)
				sf.conns.Remove(srcAddr)
				item.targetConn.Close()
				sf.log.Infof("[ UDP ] udp conn %s ---> %s released", srcAddr, localAddr)
			}()
			for {
				n, err := item.targetConn.Read(buf[:cap(buf)])
				if err != nil {
					sf.log.Warnf("[ UDP ] udp conn read from target conn fail, %s ", err)
					return
				}
				atomic.StoreInt64(&item.lastActiveTime, time.Now().Unix())
				_, err = sf.channel.WriteToUDP(buf[:n], item.srcAddr)
				if err != nil {
					sf.log.Warnf("[ UDP ] udp conn write to local conn fail, %s ", err)
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
	_, err = item.targetConn.(*net.UDPConn).WriteToUDP(msg.Data, item.targetAddr)
	if err != nil {
		sf.log.Warnf("[ UDP ] udp conn write to target conn fail, %s ", err)
	}
}

func (sf *UDP) dialParent(address string) (conn net.Conn, err error) {
	return ccs.DialTimeout(sf.cfg.ParentType, address, sf.cfg.Timeout,
		ccs.Config{
			Cert:         sf.cfg.cert,
			Key:          sf.cfg.key,
			CaCert:       sf.cfg.caCert,
			STCPMethod:   sf.cfg.STCPMethod,
			STCPPassword: sf.cfg.STCPPassword,
			KcpConfig:    sf.cfg.SKCPConfig.KcpConfig,
			Compress:     sf.cfg.ParentCompress,
		})
}

// 解析domain
func (sf *UDP) resolve(address string) string {
	if sf.dnsResolver != nil {
		return sf.dnsResolver.MustResolve(address)
	}
	return address
}
