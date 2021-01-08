package mux

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/thinkgos/x/extcert"
	"github.com/thinkgos/x/extnet"
	"github.com/thinkgos/x/lib/logger"
	"github.com/xtaci/smux"

	"github.com/thinkgos/jocasta/connection"
	"github.com/thinkgos/jocasta/core/captain"
	"github.com/thinkgos/jocasta/cs"
	"github.com/thinkgos/jocasta/pkg/ccs"
	"github.com/thinkgos/jocasta/pkg/sword"
	"github.com/thinkgos/jocasta/pkg/through"
	"github.com/thinkgos/jocasta/pkg/through/ddt"
	"github.com/thinkgos/jocasta/services"
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
	SKCPConfig ccs.SKCPConfig
	// stcp有效
	// stcp 加密方法 default: aes-192-cfb
	// stcp 加密密钥 default: thinkgos's_jocasta
	STCPConfig cs.StcpConfig
	// 其它
	Timeout time.Duration `validate:"required"` // default 2s 单位ms
	// 跳板机
	RawProxyURL string // default empty
	// private
	tcpTlsConfig cs.TLSConfig
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
	proxyURL *url.URL
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
		sf.cfg.tcpTlsConfig.Cert, sf.cfg.tcpTlsConfig.Key, err = extcert.LoadPair(sf.cfg.CertFile, sf.cfg.KeyFile)
		if err != nil {
			return err
		}
	}
	if sf.cfg.RawProxyURL != "" {
		if sf.cfg.ParentType != "tls" && sf.cfg.ParentType != "tcp" {
			return fmt.Errorf("proxyURL only worked on tls or tcp")
		}
		sf.proxyURL, err = cs.ParseProxyURL(sf.cfg.RawProxyURL)
		if err != nil {
			return fmt.Errorf("invalid proxyURL parameter, %s", err)
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

	sword.Go(func() {
		sf.udpConns.Watch(sf.ctx)
	})

	sword.Go(func() {
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

			// through message
			msg := through.NegotiateRequest{
				Types:   through.TypesClient,
				Version: through.Version,
				Nego: ddt.NegotiateRequest{
					SecretKey: sf.cfg.SecretKey,
					Id:        "reserved",
				},
			}
			data, err := msg.Bytes()
			if err != nil {
				return err
			}
			_, err = pConn.Write(data)
			if err != nil {
				sf.log.Errorf("[ Client ] connection %s, retrying...", err)
				return err
			}

			tr, err := captain.ParseReply(pConn)
			if err != nil {
				return err
			}
			if tr.Status != through.RepSuccess {
				err = errors.New("bridge response error")
				sf.log.Errorf("[ Client ] bridge response %d, retrying...", tr.Status)
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
				sword.Go(func() {
					hand, err := through.ParseHandshakeRequest(stream)
					if err != nil {
						sf.log.Errorf("[ Client ] read stream signal %s", err)
						return
					}
					localAddr := net.JoinHostPort(hand.Hand.Host, strconv.FormatUint(uint64(hand.Hand.Port), 10))
					sf.log.Debugf("[ Client ] sid< %s >@%s stream on %s@%s", hand.Hand.SessionId, hand.Hand.NodeId, hand.Hand.Protocol, localAddr)
					if hand.Hand.Protocol == ddt.Network_UDP {
						sf.proxyUDP(stream, localAddr, hand.Hand.SessionId)
					} else {
						sf.proxyTCP(stream, localAddr, hand.Hand.SessionId)
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
		da, err := captain.ParseStreamDatagram(inConn)
		if err != nil {
			if !extnet.IsErrDeadline(err) && err != io.EOF {
				sf.log.Errorf("udp packet received from bridge, %s", err)
			}
			return
		}
		cacheSrcAddr = da.Addr.String()
		if v, ok := sf.udpConns.Get(cacheSrcAddr); ok {
			item = v.(*ClientUDPConnItem)
		} else {
			_srcAddr, _ := net.ResolveUDPAddr("udp", cacheSrcAddr)
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
			sf.udpConns.Set(cacheSrcAddr, item)
			sword.Go(func() {
				sf.runUdpReceive(cacheSrcAddr, sessId)
			})
		}

		atomic.StoreInt64(&item.lastActiveTime, time.Now().Unix())
		sword.Go(func() {
			item.localConn.Write(da.Data)
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
		sword.Go(func() {
			defer sword.Binding.Put(buf)
			as, err := captain.ParseAddrSpec(connItem.srcAddr.String())
			if err != nil {
				connItem.localConn.Close()
				return
			}

			sData := captain.StreamDatagram{
				Addr: as,
				Data: buf[:n],
			}
			header, err := sData.Header()
			if err != nil {
				connItem.localConn.Close()
				return
			}
			buf := sword.Binding.Get()
			defer sword.Binding.Put(buf)

			tmpBuf := append(buf, header...)
			tmpBuf = append(tmpBuf, sData.Data...)
			_, err = connItem.conn.Write(tmpBuf)
			if err != nil {
				connItem.localConn.Close()
				return
			}
		})
	}
}

func (sf *Client) dialParent(address string) (net.Conn, error) {
	d := ccs.Dialer{
		Protocol: sf.cfg.ParentType,
		Timeout:  sf.cfg.Timeout,
		Config: ccs.Config{
			TLSConfig:  sf.cfg.tcpTlsConfig,
			StcpConfig: sf.cfg.STCPConfig,
			KcpConfig:  sf.cfg.SKCPConfig.KcpConfig,
			ProxyURL:   sf.proxyURL,
		},
		AfterChains: extnet.AdornConnsChain{extnet.AdornSnappy(sf.cfg.Compress)},
	}
	return d.Dial("tcp", address)
}

type ClientOption func(b *Client)

func WithClientLogger(l logger.Logger) ClientOption {
	return func(b *Client) {
		if l != nil {
			b.log = l
		}
	}
}
