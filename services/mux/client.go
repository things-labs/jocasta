package mux

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/xtaci/smux"

	"github.com/thinkgos/jocasta/connection"
	"github.com/thinkgos/jocasta/core/through"
	"github.com/thinkgos/jocasta/cs"
	"github.com/thinkgos/jocasta/lib/cert"
	"github.com/thinkgos/jocasta/lib/extnet"
	"github.com/thinkgos/jocasta/lib/gpool"
	"github.com/thinkgos/jocasta/lib/logger"
	"github.com/thinkgos/jocasta/pkg/sword"
	"github.com/thinkgos/jocasta/services"
	"github.com/thinkgos/jocasta/services/ccs"
	"github.com/thinkgos/jocasta/services/skcp"
)

const MaxUDPIdleTime = 30 // 单位s

type ClientConfig struct {
	ParentType string `validate:"required,oneof=tcp tls stcp kcp"` // tcp|tls|stcp|kcp default tcp
	Parent     string `validate:"required"`                        // 格式: addr:port default empty
	Compress   bool   // default false
	SecretKey  string // default default
	// tls有效
	CertFile string // default proxy.crt
	KeyFile  string // default proxy.key
	// kcp有效
	SKCPConfig skcp.Config
	// stcp有效
	STCPMethod   string `validate:"required"` // default aes-192-cfb
	STCPPassword string // default thinkgos's_goproxy
	// 其它
	Timeout time.Duration `validate:"required"` // default 2s 单位ms
	// 跳板机
	Jumper string // default empty
	// private
	cert []byte
	key  []byte
}

type ClientUDPConnItem struct {
	conn           *smux.Stream
	lastActiveTime int64
	srcAddr        *net.UDPAddr
	localAddr      *net.UDPAddr
	localConn      *net.UDPConn
	sessId         string
}

type Client struct {
	cfg      ClientConfig
	sessions *smux.Session
	udpConns *connection.Manager
	jumper   *cs.Jumper
	gPool    gpool.Pool
	cancel   context.CancelFunc
	ctx      context.Context
	log      logger.Logger
}

var _ services.Service = (*Client)(nil)

func NewClient(cfg ClientConfig, opts ...ClientOption) *Client {
	c := &Client{cfg: cfg, log: logger.NewDiscard()}

	c.udpConns = connection.New(time.Second, func(key string, value interface{}, now time.Time) bool {
		item := value.(*ClientUDPConnItem)
		if now.Unix()-atomic.LoadInt64(&item.lastActiveTime) > MaxUDPIdleTime {
			item.conn.Close()
			item.localConn.Close()
			c.log.Infof("gc udp conn %s", item.sessId)
			return true
		}
		return false
	})

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (sf *Client) inspectConfig() (err error) {
	if err = sword.Validate.Struct(&sf.cfg); err != nil {
		return err
	}

	if sf.cfg.ParentType == "tls" {
		if sf.cfg.CertFile == "" || sf.cfg.KeyFile == "" {
			return fmt.Errorf("cert file and key file required")
		}
		sf.cfg.cert, sf.cfg.key, err = cert.Parse(sf.cfg.CertFile, sf.cfg.KeyFile)
		if err != nil {
			return err
		}
	}
	if sf.cfg.Jumper != "" {
		if sf.cfg.ParentType != "tls" && sf.cfg.ParentType != "tcp" {
			return fmt.Errorf("jumper only worked on tls or tcp")
		}
		sf.jumper, err = cs.NewJumper(sf.cfg.Jumper)
		if err != nil {
			return fmt.Errorf("invalid jumper parameter, %s", err)
		}
	}
	sf.log.Infof("[ Client ] use parent %s < %s >", sf.cfg.ParentType, sf.cfg.Parent)
	return
}

func (sf *Client) Start() (err error) {
	sf.ctx, sf.cancel = context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			sf.cancel()
		}
	}()

	if err = sf.inspectConfig(); err != nil {
		return
	}

	sf.submit(func() {
		sf.udpConns.RunWatch(sf.ctx)
	})

	sf.submit(func() {
		boff := backoff.WithContext(&backoff.ExponentialBackOff{
			InitialInterval:     time.Second,
			RandomizationFactor: 0.5,
			Multiplier:          1.5,
			MaxInterval:         time.Second * 30,
			MaxElapsedTime:      0,
			Clock:               backoff.SystemClock,
		}, sf.ctx)

		_ = backoff.Retry(func() error {
			pConn, err := sf.dialParent(sf.cfg.Parent)
			if err != nil {
				sf.log.Errorf("[ Client ] dial parent %s, retrying...", err)
				return err
			}
			defer pConn.Close()
			err = through.WriteConnType(pConn, sf.cfg.Timeout, typeClient, sf.cfg.SecretKey, "reserved")
			if err != nil {
				sf.log.Errorf("[ Client ] connection %s, retrying...", err)
				return err
			}

			session, err := smux.Server(pConn, nil)
			if err != nil {
				sf.log.Errorf("[ Client ] session %s, retrying...", err)
				return err
			}

			sf.sessions = session
			sf.log.Infof("[ Client ] node client sk< %s > created", sf.cfg.SecretKey)
			for {
				select {
				case <-sf.ctx.Done():
					return backoff.Permanent(errors.New("use of closed network connection"))
				default:
				}
				stream, err := session.AcceptStream()
				if err != nil {
					session.Close()
					boff.Reset()
					sf.log.Infof("[ Client ] accept stream %s, retrying...", err)
					return err
				}
				sf.submit(func() {
					var serverNodeId, serverSessId, clientLocalAddr string

					err = through.ReadStrings(stream, sf.cfg.Timeout, &serverNodeId, &serverSessId, &clientLocalAddr)
					if err != nil {
						sf.log.Errorf("[ Client ] read stream signal %s", err)
						return
					}
					sf.log.Debugf("[ Client ] sid< %s >@%s stream on %s", serverSessId, serverNodeId, clientLocalAddr)
					protocol, localAddr := clientLocalAddr[:3], clientLocalAddr[4:]
					if protocol == "udp" {
						sf.proxyUDP(stream, localAddr, serverSessId)
					} else {
						sf.proxyTCP(stream, localAddr, serverSessId)
					}
				})
			}
		}, boff)
	})

	sf.log.Infof("[ Client ] node client started")
	return
}
func (sf *Client) Stop() {
	if sf.cancel != nil {
		sf.cancel()
	}
	if sf.sessions != nil {
		sf.sessions.Close()
	}
	sf.log.Infof("node client sk< %s > stopped", sf.cfg.SecretKey)
}

func (sf *Client) proxyUDP(inConn *smux.Stream, localAddr, sessId string) {
	var item *ClientUDPConnItem
	var cacheSrcAddr string

	defer func() {
		inConn.Close()
		if item != nil {
			sf.udpConns.Remove(cacheSrcAddr)
			item.conn.Close()
			item.localConn.Close()
		}
	}()
	for {
		select {
		case <-sf.ctx.Done():
			return
		default:
		}
		// 读远端数据,写到本地udpConn
		srcAddr, body, err := through.ReadUdp(inConn)
		if err != nil {
			if !extnet.IsErrDeadline(err) && err != io.EOF {
				sf.log.Errorf("udp packet received from bridge, %s", err)
			}
			return
		}
		cacheSrcAddr = srcAddr
		if v, ok := sf.udpConns.Get(srcAddr); ok {
			item = v.(*ClientUDPConnItem)
		} else {
			_srcAddr, _ := net.ResolveUDPAddr("udp", srcAddr)
			zeroAddr, _ := net.ResolveUDPAddr("udp", ":")
			_localAddr, _ := net.ResolveUDPAddr("udp", localAddr)
			c, err := net.DialUDP("udp", zeroAddr, _localAddr)
			if err != nil {
				sf.log.Errorf("create local udp conn fail, %s", err)
				return
			}
			item = &ClientUDPConnItem{
				conn:      inConn,
				srcAddr:   _srcAddr,
				localAddr: _localAddr,
				localConn: c,
				sessId:    sessId,
			}
			sf.udpConns.Set(srcAddr, item)
			sf.submit(func() {
				sf.runUdpReceive(srcAddr, sessId)
			})
		}

		atomic.StoreInt64(&item.lastActiveTime, time.Now().Unix())
		sf.submit(func() {
			item.localConn.Write(body)
		})
	}
}

func (sf *Client) proxyTCP(inConn net.Conn, localAddr, sessId string) {
	var targetConn net.Conn

	defer inConn.Close()

	boff := backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Second*3), 3)
	boff = backoff.WithContext(boff, sf.ctx)
	err := backoff.Retry(func() (e error) {
		targetConn, e = net.DialTimeout("tcp", localAddr, sf.cfg.Timeout)
		if e != nil {
			sf.log.Infof("[ Client ] connect to local[ %s ] failed, %s, retrying...", localAddr, e)
			return e
		}
		return nil
	}, boff)
	if err != nil {
		sf.log.Warnf("[ Client ] connect to local[ %s ] failed, %s", localAddr, err)
		return
	}

	sf.log.Infof("[ Client ] sk< %s > ---> sid< %s > stream binding created", sf.cfg.SecretKey, sessId)
	defer func() {
		sf.log.Infof("[ Client ] sk< %s > ---> sid< %s > stream binding released", sf.cfg.SecretKey, sessId)
		targetConn.Close()
	}()

	err = sword.Binding.Proxy(inConn, targetConn)
	if err != nil && err != io.EOF {
		sf.log.Errorf("[ Client ] proxying, %s", err)
	}
}

func (sf *Client) runUdpReceive(key, id string) {
	v, ok := sf.udpConns.Get(key)
	if !ok {
		sf.log.Warnf("udp conn not exists for %s, connectId : %s", key, id)
		return
	}
	connItem := v.(*ClientUDPConnItem)

	sf.log.Infof("udp conn %s connected", id)
	defer func() {
		sf.udpConns.Remove(key)
		connItem.conn.Close()
		connItem.localConn.Close()
		sf.log.Infof("udp conn %s released", id)
	}()
	for {
		// 读本地udpConn,写到远端
		buf := sword.Binding.Get()
		n, err := connItem.localConn.Read(buf[:cap(buf)])
		if err != nil {
			sword.Binding.Put(buf)
			if !extnet.IsErrClosed(err) {
				sf.log.Errorf("udp conn read udp packet fail , err: %s ", err)
			}
			return
		}
		atomic.StoreInt64(&connItem.lastActiveTime, time.Now().Unix())
		sf.submit(func() {
			defer sword.Binding.Put(buf)

			err := through.WriteUdp(connItem.conn, sf.cfg.Timeout,
				connItem.srcAddr.String(), buf[:n])
			if err != nil {
				connItem.localConn.Close()
				return
			}
		})
	}
}

func (sf *Client) dialParent(address string) (net.Conn, error) {
	return ccs.DialTimeout(sf.cfg.ParentType, address, sf.cfg.Timeout,
		ccs.Config{
			Cert:         sf.cfg.cert,
			Key:          sf.cfg.key,
			STCPMethod:   sf.cfg.STCPMethod,
			STCPPassword: sf.cfg.STCPPassword,
			KcpConfig:    sf.cfg.SKCPConfig.KcpConfig,
			Compress:     sf.cfg.Compress,
			Jumper:       sf.jumper,
		})
}

// 提交任务到协程池处理,如果协程池未定义或提交失败,将采用goroutine
func (sf *Client) submit(f func()) {
	if sf.gPool == nil || sf.gPool.Submit(f) != nil {
		go func() {
			defer func() {
				if err := recover(); err != nil {
					sf.log.DPanicf("[ Client ] crashed %s\nstack:\n%s", err, string(debug.Stack()))
				}
			}()
			f()
		}()
	}
}
