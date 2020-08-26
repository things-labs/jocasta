package mux

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/xtaci/smux"

	"github.com/thinkgos/go-core-package/extcert"
	"github.com/thinkgos/go-core-package/extnet"
	"github.com/thinkgos/go-core-package/lib/logger"

	"github.com/thinkgos/jocasta/connection"
	"github.com/thinkgos/jocasta/core/captain"
	"github.com/thinkgos/jocasta/cs"
	"github.com/thinkgos/jocasta/pkg/ccs"
	"github.com/thinkgos/jocasta/pkg/outil"
	"github.com/thinkgos/jocasta/pkg/sword"
	"github.com/thinkgos/jocasta/pkg/through"
	"github.com/thinkgos/jocasta/pkg/through/ddt"
	"github.com/thinkgos/jocasta/services"
)

type ServerConfig struct {
	ParentType string `validate:"required,oneof=tcp tls stcp kcp"` // tcp|tls|tcps|kcp default tcp
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
	Timeout time.Duration `validate:"required"` // default 2s
	// 跳板机
	RawProxyURL string // default empty

	// 路由
	// protocol://localIP:localPort@[clientKey]clientLocalHost:ClientLocalPort
	// default empty
	Route string `validate:"required"`
	isUDP bool   // default false

	// private
	tcpTlsConfig cs.TLSConfig
	// 本地暴露的地址 格式:ip:port
	local string
	// 远端要穿透的地址 格式:ip:port
	remote_host string
	remote_port uint16
}

type UDPConnItem struct {
	conn           net.Conn
	lastActiveTime int64
	srcAddr        *net.UDPAddr
	localAddr      *net.UDPAddr
	sessId         string
}

type Server struct {
	id       string
	cfg      ServerConfig
	listener interface{} //net.Listener
	sessions *smux.Session
	udpConns *connection.Manager // 本地udp地址 -> 远端连接 映射
	mu       sync.Mutex
	proxyURL *url.URL
	cancel   context.CancelFunc
	ctx      context.Context
	log      logger.Logger
}

var _ services.Service = (*Server)(nil)

func NewServer(cfg ServerConfig, opts ...ServerOption) *Server {
	s := &Server{
		id:  outil.UniqueID(),
		cfg: cfg,
		log: logger.NewDiscard(),
	}

	s.udpConns = connection.New(time.Second, func(key string, value interface{}, now time.Time) bool {
		item := value.(*UDPConnItem)
		if now.Unix()-atomic.LoadInt64(&item.lastActiveTime) > MaxUDPIdleTime {
			item.conn.Close()
			s.log.Infof("gc udp conn %s", item.sessId)
			return true
		}
		return false
	})

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (sf *Server) inspectConfig() (err error) {
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
		if sf.proxyURL, err = cs.ParseProxyURL(sf.cfg.RawProxyURL); err != nil {
			return fmt.Errorf("invalid proxyURL parameter, %s", err)
		}
	}

	// parse routes
	// protocol://localIP:localPort@[clientKey]clientLocalHost:ClientLocalPort
	if strings.HasPrefix(sf.cfg.Route, "udp://") {
		sf.cfg.isUDP = true
	}
	info := strings.TrimPrefix(strings.TrimPrefix(sf.cfg.Route, "udp://"), "tcp://")
	_routeInfo := strings.Split(info, "@")
	if len(_routeInfo) != 2 || _routeInfo[0] == "" || _routeInfo[1] == "" {
		return errors.New("invalid route format,must be like protocol://localIP:localPort@[clientKey]clientLocalHost:ClientLocalPort")
	}
	sf.cfg.local = _routeInfo[0]
	sf.cfg.remote_host, sf.cfg.remote_port, err = extnet.SplitHostPort(_routeInfo[1])
	return
}

func (sf *Server) Start() (err error) {
	sf.ctx, sf.cancel = context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			sf.cancel()
		}
	}()
	if err = sf.inspectConfig(); err != nil {
		return
	}

	var localhostAddr string

	if sf.cfg.isUDP {
		addr, err := net.ResolveUDPAddr("udp", sf.cfg.local)
		if err != nil {
			return err
		}
		var udpConn *net.UDPConn
		udpConn, err = net.ListenUDP("udp", addr)
		if err != nil {
			return err
		}
		sword.Go(
			func() {
				defer udpConn.Close()
				for {
					buf := make([]byte, 2048)
					n, srcAddr, err := udpConn.ReadFromUDP(buf)
					if err != nil {
						return
					}
					data := buf[0:n]
					sword.Go(func() {
						sf.handleUDP(udpConn, cs.Message{
							LocalAddr: addr,
							SrcAddr:   srcAddr,
							Data:      data,
						})
					})
				}
			},
		)
		sf.listener = udpConn
		sword.Go(func() {
			sf.udpConns.Watch(sf.ctx)
		})
		localhostAddr = udpConn.LocalAddr().String()
	} else {
		ln, err := extnet.Listen("tcp", sf.cfg.local, extnet.AdornSnappy(false))
		if err != nil {
			return err
		}
		sword.Go(func() {
			defer ln.Close()
			for {
				conn, err := ln.Accept()
				if err != nil {
					return
				}
				sword.Go(func() {
					sf.handleTCP(conn)
				})
			}
		})
		sf.listener = ln
		localhostAddr = ln.Addr().String()
	}

	sf.log.Infof("use %s parent %s", sf.cfg.ParentType, sf.cfg.Parent)
	sf.log.Infof("server on %s", localhostAddr)
	return
}

func (sf *Server) Stop() {
	if sf.cancel != nil {
		sf.cancel()
	}
	if sf.sessions != nil {
		sf.sessions.Close()
	}
	if sf.listener != nil {
		if c, ok := sf.listener.(io.Closer); ok {
			c.Close()
		}
	}
	sf.log.Infof("node server stopped")
}

func (sf *Server) dialThroughRemote() (outConn net.Conn, sessId string, err error) {
	outConn, err = sf.GetConn()
	if err != nil {
		return
	}
	sessId = outil.UniqueID()

	proto := ddt.Network_TCP
	if sf.cfg.isUDP {
		proto = ddt.Network_UDP
	}
	request := through.HandshakeRequest{
		Version: through.Version,
		Hand: ddt.HandshakeRequest{
			NodeId:    sf.id,
			SessionId: sessId,
			Protocol:  proto,
			Host:      sf.cfg.remote_host,
			Port:      uint32(sf.cfg.remote_port),
		},
	}
	var b []byte
	b, err = request.Bytes()
	if err != nil {
		outConn.Close()
		return
	}
	_, err = outConn.Write(b)
	if err != nil {
		outConn.Close()
	}
	return
}

func (sf *Server) GetConn() (conn net.Conn, err error) {
	sf.mu.Lock()
	defer sf.mu.Unlock()

	if sf.sessions == nil {
		var pConn net.Conn

		pConn, err = sf.dialParent()
		if err != nil {
			return
		}

		// through message
		var data []byte
		msg := through.NegotiateRequest{
			Types:   through.TypesServer,
			Version: through.Version,
			Nego: ddt.NegotiateRequest{
				SecretKey: sf.cfg.SecretKey,
				Id:        sf.id,
			},
		}
		data, err = msg.Bytes()
		if err != nil {
			return
		}
		_, err = pConn.Write(data)
		if err != nil {
			_ = pConn.Close()
			return
		}
		var tr captain.Reply
		tr, err = captain.ParseReply(pConn)
		if err != nil {
			_ = pConn.Close()
			return
		}
		if tr.Status != through.RepSuccess {
			err = errors.New("bridge response error")
			return
		}

		sf.sessions, err = smux.Client(pConn, nil)
		if err != nil {
			return
		}

		sf.log.Infof("session[%s] created", sf.cfg.SecretKey)
		sword.Go(func() {
			t := time.NewTicker(time.Second * 5)
			defer t.Stop()
			for {
				select {
				case <-sf.ctx.Done():
					return
				case <-t.C:
				}
				sf.mu.Lock()
				if sf.sessions != nil && sf.sessions.IsClosed() {
					sf.sessions = nil
					sf.mu.Unlock()
					return
				}
				sf.mu.Unlock()
			}
		})
	}
	conn, err = sf.sessions.OpenStream()
	if err != nil {
		sf.sessions.Close()
		sf.sessions = nil
	}
	return
}

func (sf *Server) dialParent() (net.Conn, error) {
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
	return d.Dial("tcp", sf.cfg.Parent)
}

func (sf *Server) runUDPReceive(key, id string) {
	v, ok := sf.udpConns.Get(key)
	if !ok {
		sf.log.Warnf("udp conn not exists for %s, connectId : %s", key, id)
		return
	}
	udpConnItem := v.(*UDPConnItem)

	sf.log.Infof("udp conn %s connected", id)
	defer func() {
		sf.udpConns.Remove(key)
		udpConnItem.conn.Close()
		sf.log.Infof("udp conn %s released", id)
	}()

	for {
		// 从远端接收数据,发送到本地
		da, err := captain.ParseStreamDatagram(udpConnItem.conn)
		if err != nil {
			if strings.Contains(err.Error(), "n != int(") {
				continue
			}
			if err != io.EOF {
				sf.log.Errorf("udp conn read udp packet fail, %s ", err)
			}
			return
		}
		atomic.StoreInt64(&udpConnItem.lastActiveTime, time.Now().Unix())
		sword.Go(func() {
			sf.listener.(*net.UDPConn).WriteToUDP(da.Data, udpConnItem.srcAddr)
		})
	}
}

func (sf *Server) handleUDP(_ *net.UDPConn, msg cs.Message) {
	var udpConnItem *UDPConnItem

	srcAddr := msg.SrcAddr.String()
	if v, ok := sf.udpConns.Get(srcAddr); ok {
		udpConnItem = v.(*UDPConnItem)
	} else {
		// 不存在,建立一条与远端链接隧道
		outConn, id, err := sf.dialThroughRemote()
		if err != nil {
			sf.log.Errorf("connect to %s fail, %s", sf.cfg.Parent, err)
			return
		}
		udpConnItem = &UDPConnItem{
			conn:      outConn,
			srcAddr:   msg.SrcAddr,
			localAddr: msg.LocalAddr,
			sessId:    id,
		}
		sf.udpConns.Set(srcAddr, udpConnItem)
		// 从远端接收数据,发送到本地
		sword.Go(func() {
			sf.runUDPReceive(srcAddr, id)
		})
	}
	// 读取本地数据, 发送数据到远端
	atomic.StoreInt64(&udpConnItem.lastActiveTime, time.Now().Unix())

	as, err := captain.ParseAddrSpec(srcAddr)
	if err != nil {
		return
	}

	sData := captain.StreamDatagram{
		Addr: as,
		Data: msg.Data,
	}
	header, err := sData.Header()
	if err != nil {
		return
	}
	buf := sword.Binding.Get()
	defer sword.Binding.Put(buf)

	tmpBuf := append(buf, header...)
	tmpBuf = append(tmpBuf, sData.Data...)
	_, err = udpConnItem.conn.Write(tmpBuf)
	if err != nil {
		sf.log.Errorf("write udp packet to %s fail, %s ", sf.cfg.Parent, err)
	}
}

func (sf *Server) handleTCP(inConn net.Conn) {
	var sessId string
	var targetConn net.Conn

	defer inConn.Close()

	boff := backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Second*3), 10)
	boff = backoff.WithContext(boff, sf.ctx)
	err := backoff.Retry(func() (e error) {
		targetConn, sessId, e = sf.dialThroughRemote()
		if e != nil {
			sf.log.Infof("[ Server ] connect to %s, %s, retrying...", sf.cfg.Parent, e)
			return e
		}
		return nil
	}, boff)

	if err != nil {
		return
	}

	sf.log.Infof("[ Server ] sk< %s > ---> sid< %s > stream binding created", sf.cfg.SecretKey, sessId)
	defer func() {
		sf.log.Infof("[ Server ] sk< %s > ---> sid< %s > stream binding released", sf.cfg.SecretKey, sessId)
		targetConn.Close()
	}()
	err = sword.Binding.Proxy(targetConn, inConn)
	if err != nil && err != io.EOF {
		sf.log.Errorf("[ Server ] proxying, %s", err)
	}
}

type ServerOption func(b *Server)

func WithServerLogger(l logger.Logger) ServerOption {
	return func(b *Server) {
		if l != nil {
			b.log = l
		}
	}
}
