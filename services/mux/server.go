package mux

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/xtaci/smux"

	"github.com/thinkgos/jocasta/connection"
	"github.com/thinkgos/jocasta/core/through"
	"github.com/thinkgos/jocasta/cs"
	"github.com/thinkgos/jocasta/lib/cert"
	"github.com/thinkgos/jocasta/lib/logger"
	"github.com/thinkgos/jocasta/lib/outil"
	"github.com/thinkgos/jocasta/pkg/captain"
	"github.com/thinkgos/jocasta/pkg/captain/ddt"
	"github.com/thinkgos/jocasta/pkg/sword"
	"github.com/thinkgos/jocasta/services"
	"github.com/thinkgos/jocasta/services/ccs"
	"github.com/thinkgos/jocasta/services/skcp"
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
	SKCPConfig skcp.Config
	// stcp有效
	STCPMethod   string `validate:"required"` // default aes-192-cfb
	STCPPassword string // default thinkgos's_goproxy
	// 其它
	Timeout time.Duration `validate:"required"` // default 2s
	// 跳板机
	Jumper string // default empty

	// 路由
	// protocol://localIP:localPort@[clientKey]clientLocalHost:ClientLocalPort
	// default empty
	Route string `validate:"required"`
	IsUDP bool   // default false

	// private
	cert []byte
	key  []byte
	// 本地暴露的地址 格式:ip:port
	local string
	// 远端要穿透的地址 格式:ip:port
	remote string
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
	channel  cs.Channel
	sessions *smux.Session
	udpConns *connection.Manager // 本地udp地址 -> 远端连接 映射
	mu       sync.Mutex
	jumper   *cs.Jumper
	gPool    sword.GoPool
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
		sf.cfg.cert, sf.cfg.key, err = cert.Parse(sf.cfg.CertFile, sf.cfg.KeyFile)
		if err != nil {
			return err
		}
	}

	if sf.cfg.Jumper != "" {
		if sf.cfg.ParentType != "tls" && sf.cfg.ParentType != "tcp" {
			return fmt.Errorf("jumper only worked on tls or tcp")
		}
		if sf.jumper, err = cs.NewJumper(sf.cfg.Jumper); err != nil {
			return fmt.Errorf("invalid jumper parameter, %s", err)
		}
	}

	// parse routes
	// protocol://localIP:localPort@[clientKey]clientLocalHost:ClientLocalPort
	if strings.HasPrefix(sf.cfg.Route, "udp://") {
		sf.cfg.IsUDP = true
	}
	info := strings.TrimPrefix(strings.TrimPrefix(sf.cfg.Route, "udp://"), "tcp://")
	_routeInfo := strings.Split(info, "@")
	if len(_routeInfo) != 2 || _routeInfo[0] == "" || _routeInfo[1] == "" {
		return errors.New("invalid route format,must be like protocol://localIP:localPort@[clientKey]clientLocalHost:ClientLocalPort")
	}
	sf.cfg.local, sf.cfg.remote = _routeInfo[0], _routeInfo[1]

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

	if sf.cfg.IsUDP {
		sf.cfg.remote = "udp:" + sf.cfg.remote
		sf.channel, err = cs.NewUDP(sf.cfg.local, sf.handleUDP, cs.WithUDPGPool(sword.GPool))
	} else {
		sf.cfg.remote = "tcp:" + sf.cfg.remote
		sf.channel, err = cs.NewTCP(sf.cfg.local, false, sf.handleTCP, cs.WithTCPGPool(sword.GPool))
	}
	if err != nil {
		return
	}
	sf.gPool.Go(func() { _ = sf.channel.ListenAndServe() })

	if err = <-sf.channel.Status(); err != nil {
		return
	}

	if sf.cfg.IsUDP {
		sf.gPool.Go(func() {
			sf.udpConns.RunWatch(sf.ctx)
		})
	}
	sf.log.Infof("use %s parent %s", sf.cfg.ParentType, sf.cfg.Parent)
	sf.log.Infof("server on %s", sf.channel.Addr())
	return
}

func (sf *Server) Stop() {
	if sf.cancel != nil {
		sf.cancel()
	}
	if sf.sessions != nil {
		sf.sessions.Close()
	}
	if sf.channel != nil {
		sf.channel.Close()
	}
	sf.log.Infof("node server stopped")
}

func (sf *Server) dialThroughRemote() (outConn net.Conn, sessId string, err error) {
	outConn, err = sf.GetConn()
	if err != nil {
		return
	}
	sessId = outil.UniqueID()

	err = through.WriteStrings(outConn, sf.cfg.Timeout, sf.id, sessId, sf.cfg.remote)
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
		msg := captain.ThroughNegotiateRequest{
			Types:   captain.TTypesServer,
			Version: 1,
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

		sf.sessions, err = smux.Client(pConn, nil)
		if err != nil {
			return
		}

		sf.log.Infof("session[%s] created", sf.cfg.SecretKey)
		sf.gPool.Go(func() {
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
	return ccs.DialTimeout(sf.cfg.ParentType, sf.cfg.Parent, sf.cfg.Timeout,
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
		sf.gPool.Go(func() {
			sf.channel.(*cs.UDP).WriteToUDP(da.Data, udpConnItem.srcAddr)
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
		sf.gPool.Go(func() {
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
