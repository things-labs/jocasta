package tcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	"github.com/thinkgos/strext"
	"golang.org/x/sync/singleflight"

	"github.com/thinkgos/jocasta/connection"
	"github.com/thinkgos/jocasta/core/idns"
	"github.com/thinkgos/jocasta/core/through"
	"github.com/thinkgos/jocasta/cs"
	"github.com/thinkgos/jocasta/lib/cert"
	"github.com/thinkgos/jocasta/lib/encrypt"
	"github.com/thinkgos/jocasta/lib/gpool"
	"github.com/thinkgos/jocasta/lib/logger"
	"github.com/thinkgos/jocasta/pkg/sword"
	"github.com/thinkgos/jocasta/services"
	"github.com/thinkgos/jocasta/services/ccs"
	"github.com/thinkgos/jocasta/services/skcp"
)

const MaxUDPIdleTime = 10 // 单位s

type Config struct {
	// parent
	ParentType     string `validate:"required,oneof=tcp tls stcp kcp udp"` // 父级协议类型 tcp|tls|stcp|kcp|udp default empty
	Parent         string //`validate:"required,tcp_addr|udp_addr"`          // 父级地址,格式addr:port, default empty
	ParentCompress bool   // default false
	// local
	LocalType     string `validate:"required,oneof=tcp tls stcp kcp"` // 本地协议类型 tcp|tls|stcp|kcp
	Local         string //`validate:"required,tcp_addr|udp_addr"`      // 本地监听地址 default :28080
	LocalCompress bool   // default false
	// tls有效
	CertFile   string // cert文件 default proxy.crt
	KeyFile    string // key文件 default proxy.key
	CaCertFile string // ca文件 default empty
	// kcp有效
	SKCPConfig *skcp.Config
	// stcp有效
	STCPMethod   string `validate:"required"`
	STCPPassword string // default thinkgos's_goproxy
	// 其它
	Timeout time.Duration `validate:"required"` // dial超时时间, default 2s
	// 跳板机 仅支持tls,tcp下使用
	// https://username:password@host:port
	// https://host:port
	// socks5://username:password@host:port
	// socks5://host:port
	Jumper              string
	CheckParentInterval int // TODO: not used确认代理是否正常间隔,0表示不检查, default 3 单位s
	// private
	cert   []byte
	key    []byte
	caCert []byte
}

type connItem struct {
	conn           net.Conn
	srcAddr        *net.UDPAddr
	targetAddr     *net.UDPAddr
	targetConn     net.Conn
	lastActiveTime int64
}

type TCP struct {
	cfg     Config
	channel cs.Channel
	// parent type = "udp", udp -> udp绑定传输
	// src地址对udp连接映射
	// parent type != "udp", udp -> net.conn 其它的绑定传输
	// src地址对其它连接的绑定
	userConns   *connection.Manager
	single      *singleflight.Group
	jumper      *cs.Jumper
	dnsResolver *idns.Resolver
	gPool       gpool.Pool
	cancel      context.CancelFunc
	ctx         context.Context
	log         logger.Logger
}

var _ services.Service = (*TCP)(nil)

func New(cfg Config, opts ...Option) *TCP {
	t := &TCP{
		cfg: cfg,
		userConns: connection.New(time.Second,
			func(key string, value interface{}, now time.Time) bool {
				nowSeconds := now.Unix()
				item := value.(*connItem)
				if nowSeconds-atomic.LoadInt64(&item.lastActiveTime) > MaxUDPIdleTime {
					item.conn.Close()
					item.targetConn.Close()
					return true
				}
				return false
			}),
		log: logger.NewDiscard(),
	}
	for _, opt := range opts {
		opt(t)
	}
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
		sf.cfg.cert, sf.cfg.key, err = cert.Parse(sf.cfg.CertFile, sf.cfg.KeyFile)
		if err != nil {
			return err
		}
		if sf.cfg.CaCertFile != "" {
			if sf.cfg.caCert, err = ioutil.ReadFile(sf.cfg.CaCertFile); err != nil {
				return fmt.Errorf("read ca file %+v", err)
			}
		}
	}

	// stcp 方法检查
	if strext.Contains([]string{sf.cfg.ParentType, sf.cfg.LocalType}, "stcp") &&
		!strext.Contains(encrypt.CipherMethods(), sf.cfg.STCPMethod) {
		return fmt.Errorf("stcp cipher method support one of %s", strings.Join(encrypt.CipherMethods(), ","))
	}

	if sf.cfg.Jumper != "" {
		if !strext.Contains([]string{"tcp", "tls"}, sf.cfg.ParentType) {
			return fmt.Errorf("jumper only support one of parent type <tcp|tls> but give %s", sf.cfg.ParentType)
		}
		if sf.jumper, err = cs.NewJumper(sf.cfg.Jumper); err != nil {
			return fmt.Errorf("new jumper, %+v", err)
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

	sf.channel, err = ccs.ListenAndServeAny(sf.cfg.LocalType, sf.cfg.Local, sf.handler,
		ccs.Config{
			Cert:         sf.cfg.cert,
			Key:          sf.cfg.key,
			CaCert:       sf.cfg.caCert,
			STCPMethod:   sf.cfg.STCPMethod,
			STCPPassword: sf.cfg.STCPPassword,
			KcpConfig:    sf.cfg.SKCPConfig.KcpConfig,
			Compress:     sf.cfg.LocalCompress,
		})
	if err != nil {
		return err
	}

	if sf.cfg.ParentType == "udp" {
		sf.Go(func() { sf.userConns.RunWatch(sf.ctx) })
	}

	sf.log.Infof("[ TCP ] use parent %s< %s >", sf.cfg.Parent, sf.cfg.ParentType)
	sf.log.Infof("[ TCP ] use proxy %s on %s", sf.cfg.LocalType, sf.channel.Addr())
	return nil
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
	defer inConn.Close()
	switch sf.cfg.ParentType {
	case "tcp", "tls", "stcp", "kcp":
		sf.proxyAnyToAny(inConn)
	case "udp":
		sf.proxyAny2UDP(inConn)
	default:
		sf.log.Errorf("unknown parent type %s", sf.cfg.ParentType)
	}
}

func (sf *TCP) proxyAnyToAny(inConn net.Conn) {
	targetConn, err := sf.dialParent(sf.resolve(sf.cfg.Parent))
	if err != nil {
		sf.log.Errorf("[ TCP ] dial parent %s, %s", sf.cfg.Parent, err)
		return
	}

	srcAddr := inConn.RemoteAddr().String()
	targetAddr := targetConn.RemoteAddr().String()

	sf.userConns.Upsert(srcAddr, inConn, func(exist bool, valueInMap, newValue interface{}) interface{} {
		if exist {
			log.Println("exissfafdadlfkal;dfaldfk")
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
	if err != nil && err != io.EOF {
		sf.log.Errorf("[ TCP ] proxying, %s", err)
	}
}

func (sf *TCP) proxyAny2UDP(inConn net.Conn) {
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
		srcAddr, body, err := through.ReadUdp(inConn)
		if err != nil {
			sf.log.Warnf("[ TCP ] udp read from local conn fail, %v", err)
			if strings.Contains(err.Error(), "n != int(") {
				continue
			}
			return
		}

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
			sf.Go(func() {
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
						sf.log.Warnf("[ TCP ] udp read from target conn fail, %v", err)
						return
					}
					atomic.StoreInt64(&item.lastActiveTime, time.Now().Unix())
					err = through.WriteUdp(item.conn, sf.cfg.Timeout, item.srcAddr.String(), buf[:n])
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
		_, err = item.targetConn.Write(body)
		if err != nil {
			sf.log.Errorf("[ TCP ] udp write to target conn fail, %s", err)
			return
		}
	}
}

func (sf *TCP) dialParent(address string) (net.Conn, error) {
	return ccs.DialTimeout(sf.cfg.ParentType, address, sf.cfg.Timeout,
		ccs.Config{
			Cert:         sf.cfg.cert,
			Key:          sf.cfg.key,
			STCPMethod:   sf.cfg.STCPMethod,
			STCPPassword: sf.cfg.STCPPassword,
			KcpConfig:    sf.cfg.SKCPConfig.KcpConfig,
			Compress:     sf.cfg.ParentCompress,
			Jumper:       sf.jumper,
		})
}

// 解析domain
func (sf *TCP) resolve(address string) string {
	if sf.dnsResolver != nil {
		return sf.dnsResolver.MustResolve(address)
	}
	return address
}

// 提交任务到协程池处理,如果协程池未定义或提交失败,将采用goroutine
func (sf *TCP) Go(f func()) {
	if sf.gPool == nil || sf.gPool.Submit(f) != nil {
		go func() {
			defer func() {
				if err := recover(); err != nil {
					sf.log.DPanicf("[ TCP ] crashed %s\nstack:\n%s", err, string(debug.Stack()))
				}
			}()
			f()
		}()
	}
}
