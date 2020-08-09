// 内网穿透,多路复用
package mux

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime/debug"
	"time"

	"github.com/cenkalti/backoff/v4"
	cmap "github.com/orcaman/concurrent-map"
	"github.com/xtaci/smux"

	"github.com/thinkgos/jocasta/connection"
	"github.com/thinkgos/jocasta/cs"
	"github.com/thinkgos/jocasta/lib/cert"
	"github.com/thinkgos/jocasta/lib/gpool"
	"github.com/thinkgos/jocasta/lib/logger"
	"github.com/thinkgos/jocasta/pkg/sword"

	"github.com/thinkgos/jocasta/core/through"
	"github.com/thinkgos/jocasta/services"
	"github.com/thinkgos/jocasta/services/ccs"
	"github.com/thinkgos/jocasta/services/skcp"
)

// node type
const (
	typeClientControl = uint8(1)
	typeServer        = uint8(4)
	typeClient        = uint8(5)
)

type BridgeConfig struct {
	LocalType string `validate:"required,oneof=tcp tls stcp kcp"` // tcp|tls|stcp|kcp, default tcp
	Local     string `validate:"required"`                        // default :28080
	Compress  bool   // 是否压缩传输, default false
	// tls有效
	CertFile string // default proxy.crt
	KeyFile  string // default proxy.key
	// kcp有效
	SKCPConfig skcp.Config
	// stcp有效
	STCPMethod   string `validate:"required"` // stcp 加密方法 default aes-192-cfb
	STCPPassword string // stcp 加密密钥 default thinkgos's_goproxy
	// 其它
	Timeout time.Duration `validate:"required"` // 连接超时时间 default 2s
	// private
	cert []byte
	key  []byte
}

type Bridge struct {
	cfg         BridgeConfig
	channel     cs.Channel
	clientConns *connection.Manager // sk 对 session映射
	serverConns cmap.ConcurrentMap  // address 对 session映射
	gPool       gpool.Pool
	cancel      context.CancelFunc
	ctx         context.Context
	log         logger.Logger
}

var _ services.Service = (*Bridge)(nil)

func NewBridge(cfg BridgeConfig, opts ...BridgeOption) *Bridge {
	b := &Bridge{
		cfg:         cfg,
		serverConns: cmap.New(),
		log:         logger.NewDiscard(),
	}

	b.clientConns = connection.New(time.Second*5, func(key string, value interface{}, now time.Time) bool {
		sess := value.(*smux.Session)
		if sess.IsClosed() {
			sess.Close()
			b.log.Infof("[ Bridge ] node client released - sk< %s >", key)
			return true
		}
		return false
	})

	for _, opt := range opts {
		opt(b)
	}
	return b
}

func (sf *Bridge) inspectConfig() (err error) {
	if err = sword.Validate.Struct(&sf.cfg); err != nil {
		return err
	}

	if sf.cfg.LocalType == "tls" {
		if sf.cfg.CertFile == "" || sf.cfg.KeyFile == "" {
			return fmt.Errorf("cert file and key file required")
		}
		sf.cfg.cert, sf.cfg.key, err = cert.Parse(sf.cfg.CertFile, sf.cfg.KeyFile)
		if err != nil {
			return
		}
	}
	return
}

func (sf *Bridge) Start() (err error) {
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
			STCPMethod:   sf.cfg.STCPMethod,
			STCPPassword: sf.cfg.STCPPassword,
			KcpConfig:    sf.cfg.SKCPConfig.KcpConfig,
			Compress:     sf.cfg.Compress,
		})
	if err != nil {
		return
	}
	sf.Go(func() { sf.clientConns.RunWatch(sf.ctx) })
	sf.log.Infof("[ Bridge ] use bridge %s on %s", sf.cfg.LocalType, sf.channel.Addr())
	return
}

func (sf *Bridge) Stop() {
	if sf.cancel != nil {
		sf.cancel()
	}
	if sf.channel != nil {
		_ = sf.channel.Close()
	}
	for _, sess := range sf.clientConns.Items() {
		_ = sess.(*smux.Session).Close()
	}
	for _, sess := range sf.serverConns.Items() {
		_ = sess.(*smux.Session).Close()
	}
	sf.log.Infof("[ Bridge ] service bridge %s stopped", sf.cfg.LocalType)
}

func (sf *Bridge) handler(inConn net.Conn) {
	var nodeType uint8
	var nodeSecretKey string
	var nodeId string

	err := through.ReadConnType(inConn, sf.cfg.Timeout, &nodeType, &nodeSecretKey, &nodeId) // 连接类型和sk,节点id
	if err != nil {
		inConn.Close()
		sf.log.Errorf("[ Bridge ] read ddt packet, %s", err)
		return
	}
	sf.log.Debugf("[ Bridge ] node connected: type< %d >,sk< %s >,id< %s >", nodeType, nodeSecretKey, nodeId)
	switch nodeType {
	case typeServer:
		defer inConn.Close()

		session, err := smux.Server(inConn, nil)
		if err != nil {
			sf.log.Errorf("[ Bridge ] node server session, %+v", err)
			return
		}

		inAddr := inConn.RemoteAddr().String()
		sf.serverConns.Upsert(inAddr, session, func(exist bool, valueInMap, newValue interface{}) interface{} {
			if exist {
				_ = valueInMap.(*smux.Session).Close()
			}
			return newValue
		})

		sf.log.Infof("[ Bridge ] server %s connected -- sk< %s >", nodeId, nodeSecretKey)
		defer func() {
			sf.log.Infof("[ Bridge ] server %s released -- sk< %s >", nodeId, nodeSecretKey)
			sf.serverConns.Remove(inAddr)
			_ = session.Close()
		}()

		for {
			stream, err := session.AcceptStream()
			if err != nil {
				return
			}
			sf.Go(func() {
				sf.proxyStream(stream, nodeSecretKey, nodeId)
			})
		}

	case typeClient:
		session, err := smux.Client(inConn, nil)
		if err != nil {
			_ = inConn.Close()
			sf.log.Errorf("[ Bridge ] node client session, %+v", err)
			return
		}

		sf.clientConns.Upsert(nodeSecretKey, session, func(exist bool, valueInMap, newValue interface{}) interface{} {
			if exist {
				_ = valueInMap.(*smux.Session).Close()
			}
			return newValue
		})
		sf.log.Infof("[ Bridge ] client connected -- sk< %s >", nodeSecretKey)
	default:
		sf.log.Errorf("[ Bridge ] node type unknown < %d >", nodeType)
	}
}

func (sf *Bridge) proxyStream(inStream *smux.Stream, sk, serverNodeId string) {
	var targetStream *smux.Stream

	defer inStream.Close()

	// try to go a binding client
	boff := backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Second*3), 10)
	boff = backoff.WithContext(boff, sf.ctx)
	err := backoff.Retry(func() (err error) {
		select {
		case <-inStream.GetDieCh():
			return backoff.Permanent(io.ErrClosedPipe)
		default:
		}
		conn, ok := sf.clientConns.Get(sk)
		if !ok {
			sf.log.Infof("[ Bridge ] client sk< %s > not exists for server %d@%s, retrying...", sk, inStream.ID(), serverNodeId)
			return errors.New("client not exists")
		}

		if conn.(*smux.Session).IsClosed() {
			return backoff.Permanent(io.ErrClosedPipe)
		}

		targetStream, err = conn.(*smux.Session).OpenStream()
		if err != nil {
			if errors.Is(err, io.ErrClosedPipe) {
				return backoff.Permanent(io.ErrClosedPipe)
			}
			sf.log.Infof("[ Bridge ] client sk< %s > open stream for server %d@%s failed, %v, retrying...", sk, inStream.ID(), serverNodeId, err)
			return err
		}
		return nil
	}, boff)
	if err != nil {
		sf.log.Errorf("[ Bridge ] client sk< %s > ---> server %d@%s failed, %v", sk, inStream.ID(), serverNodeId, err)
		return
	}

	sf.log.Infof("[ Bridge ] client %d@sk< %s > ---> server %d@%s created", targetStream.ID(), sk, inStream.ID(), serverNodeId)
	defer func() {
		targetStream.Close()
		sf.log.Infof("[ Bridge ] client %d@sk< %s > ---> server %d@%s released", targetStream.ID(), sk, inStream.ID(), serverNodeId)
	}()

	err = sword.Binding.Proxy(targetStream, inStream)
	if err != nil && err != io.EOF {
		sf.log.Errorf("[ Bridge ] proxying, %s", err)
	}
}

// 提交任务到协程池处理,如果协程池未定义或提交失败,将采用goroutine
func (sf *Bridge) Go(f func()) {
	if sf.gPool == nil || sf.gPool.Submit(f) != nil {
		go func() {
			defer func() {
				if err := recover(); err != nil {
					sf.log.DPanicf("[ Bridge ] crashed %s\nstack:\n%s", err, string(debug.Stack()))
				}
			}()
			f()
		}()
	}
}