// 内网穿透,多路复用
package mux

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	cmap "github.com/orcaman/concurrent-map"
	"github.com/thinkgos/go-core-package/lib/encrypt"
	"github.com/thinkgos/strext"
	"github.com/xtaci/smux"

	"github.com/thinkgos/jocasta/connection"
	"github.com/thinkgos/jocasta/core/captain"
	"github.com/thinkgos/jocasta/cs"
	"github.com/thinkgos/jocasta/lib/cert"
	"github.com/thinkgos/jocasta/lib/logger"
	"github.com/thinkgos/jocasta/pkg/ccs"
	"github.com/thinkgos/jocasta/pkg/sword"
	"github.com/thinkgos/jocasta/pkg/through"
	"github.com/thinkgos/jocasta/services"
)

type BridgeConfig struct {
	LocalType string `validate:"required,oneof=tcp tls stcp kcp"` // tcp|tls|stcp|kcp, default: tcp
	Local     string `validate:"required"`                        // default: :28080
	Compress  bool   // 是否压缩传输, default: false
	// tls有效
	CaCertFile string // default: empty
	CertFile   string // default: proxy.crt
	KeyFile    string // default: proxy.key
	// kcp有效
	SKCPConfig ccs.SKCPConfig
	// stcp有效
	// stcp 加密方法 default: aes-192-cfb
	// stcp 加密密钥 default: thinkgos's_jocasta
	STCPConfig cs.StcpConfig
	// 其它
	Timeout time.Duration `validate:"required"` // 连接超时时间 default 2s
	// private
	tlsConfig cs.TLSConfig
}

type Bridge struct {
	cfg           BridgeConfig
	channel       cs.Server
	clientSession *connection.Manager // sk ---> session映射
	serverSession cmap.ConcurrentMap  // addr ---> session映射
	cancel        context.CancelFunc
	ctx           context.Context
	log           logger.Logger
}

var _ services.Service = (*Bridge)(nil)

func NewBridge(cfg BridgeConfig, opts ...BridgeOption) *Bridge {
	b := &Bridge{
		cfg:           cfg,
		serverSession: cmap.New(),
		log:           logger.NewDiscard(),
	}

	b.clientSession = connection.New(time.Second*5, func(key string, value interface{}, now time.Time) bool {
		if sess := value.(*smux.Session); sess.IsClosed() {
			sess.Close()
			b.log.Infof("[ Bridge ] Node client released - sk< %s >", key)
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

	// tls证书检查
	if sf.cfg.LocalType == "tls" {
		if sf.cfg.CertFile == "" || sf.cfg.KeyFile == "" {
			return fmt.Errorf("cert file and key file required")
		}
		sf.cfg.tlsConfig.Cert, sf.cfg.tlsConfig.Key, err = cert.LoadPair(sf.cfg.CertFile, sf.cfg.KeyFile)
		if err != nil {
			return
		}
		if sf.cfg.CaCertFile != "" {
			if sf.cfg.tlsConfig.CaCert, err = cert.LoadCrt(sf.cfg.CaCertFile); err != nil {
				return fmt.Errorf("read ca file %+v", err)
			}
		}
	}

	// stcp 方法检查
	if strext.Contains([]string{sf.cfg.Local, sf.cfg.LocalType}, "stcp") &&
		!strext.Contains(encrypt.CipherMethods(), sf.cfg.STCPConfig.Method) {
		return fmt.Errorf("stcp cipher method support one of %s", strings.Join(encrypt.CipherMethods(), ","))
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

	srv := ccs.Server{
		Protocol: sf.cfg.LocalType,
		Addr:     sf.cfg.Local,
		Config: ccs.Config{
			TLSConfig:  sf.cfg.tlsConfig,
			StcpConfig: sf.cfg.STCPConfig,
			KcpConfig:  sf.cfg.SKCPConfig.KcpConfig,
		},
		GoPool:      sword.GoPool,
		AfterChains: cs.AdornConnsChain{cs.AdornCsnappy(sf.cfg.Compress)},
		Handler:     cs.HandlerFunc(sf.handler),
	}
	var errChan <-chan error
	sf.channel, errChan = srv.RunListenAndServe()
	if err = <-errChan; err != nil {
		return
	}
	sword.Go(func() { sf.clientSession.Watch(sf.ctx) })
	sf.log.Infof("[ Bridge ] use bridge %s on %s", sf.cfg.LocalType, sf.channel.LocalAddr())
	return
}

func (sf *Bridge) Stop() {
	if sf.cancel != nil {
		sf.cancel()
	}
	if sf.channel != nil {
		_ = sf.channel.Close()
	}
	for _, sess := range sf.clientSession.Items() {
		sess.(*smux.Session).Close() // nolint: errcheck
	}
	for _, sess := range sf.serverSession.Items() {
		sess.(*smux.Session).Close() // nolint: errcheck
	}
	sf.log.Infof("[ Bridge ] bridge %s stopped", sf.cfg.LocalType)
}

func (sf *Bridge) handler(inConn net.Conn) {
	negos, err := through.ParseNegotiateRequest(inConn)
	if err != nil {
		inConn.Close()
		sf.log.Errorf("[ Bridge ] parse negotiate request, %s", err)
		return
	}
	sf.log.Debugf("[ Bridge ] Node connected: type< %d >,sk< %s >,id< %s >", negos.Types, negos.Nego.SecretKey, negos.Nego.Id)

	switch negos.Types {
	case through.TypesServer:
		session, err := smux.Server(inConn, nil)
		if err != nil {
			inConn.Write([]byte{through.RepServerFailure, through.Version}) // nolint: errcheck
			inConn.Close()                                                  // nolint: errcheck
			sf.log.Errorf("[ Bridge ] Node smux server session, %+v", err)
			return
		}

		inAddr := inConn.RemoteAddr().String()
		sf.serverSession.Upsert(inAddr, session, func(exist bool, valueInMap, newValue interface{}) interface{} {
			if exist {
				_ = valueInMap.(*smux.Session).Close()
			}
			return newValue
		})
		captain.SendReply(inConn, through.RepSuccess, through.Version) // nolint: errcheck

		sf.log.Infof("[ Bridge ] Node server %s connected -- sk< %s >", negos.Nego.Id, negos.Nego.SecretKey)
		defer func() {
			sf.log.Infof("[ Bridge ] Node server %s released -- sk< %s >", negos.Nego.Id, negos.Nego.SecretKey)
			sf.serverSession.Remove(inAddr)
			session.Close() // nolint: errcheck
			inConn.Close()  // nolint: errcheck
		}()

		for {
			stream, err := session.AcceptStream()
			if err != nil {
				return
			}
			sword.Go(func() {
				sf.proxyStream(stream, negos.Nego.SecretKey, negos.Nego.Id)
			})
		}

	case through.TypesClient:
		session, err := smux.Client(inConn, nil)
		if err != nil {
			captain.SendReply(inConn, through.RepServerFailure, through.Version) // nolint: errcheck
			inConn.Close()                                                       // nolint: errcheck
			sf.log.Errorf("[ Bridge ] Node client session, %+v", err)
			return
		}
		captain.SendReply(inConn, through.RepSuccess, through.Version) // nolint: errcheck

		sf.clientSession.Upsert(negos.Nego.SecretKey, session, func(exist bool, valueInMap, newValue interface{}) interface{} {
			if exist {
				_ = valueInMap.(*smux.Session).Close() // nolint: errcheck
			}
			return newValue
		})

		sf.log.Infof("[ Bridge ] Node client connected -- sk< %s >", negos.Nego.SecretKey)
	default:
		captain.SendReply(inConn, through.RepTypesNotSupport, through.Version) // nolint: errcheck
		sf.log.Errorf("[ Bridge ] Node type unknown < %d >", negos.Types)
	}
}

func (sf *Bridge) proxyStream(inStream *smux.Stream, sk, serverNodeId string) {
	var targetStream *smux.Stream

	defer inStream.Close()

	// try to binding a client
	boff := backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Second*3), 10)
	boff = backoff.WithContext(boff, sf.ctx)
	err := backoff.Retry(func() (err error) {
		select {
		case <-inStream.GetDieCh():
			return backoff.Permanent(io.ErrClosedPipe)
		default:
		}
		conn, ok := sf.clientSession.Get(sk)
		if !ok {
			sf.log.Infof("[ Bridge ] Node client sk< %s > not exists for server %d@%s, retrying...", sk, inStream.ID(), serverNodeId)
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
			sf.log.Infof("[ Bridge ] Node client sk< %s > open stream for server %d@%s failed, %v, retrying...", sk, inStream.ID(), serverNodeId, err)
			return err
		}
		return nil
	}, boff)
	if err != nil {
		sf.log.Errorf("[ Bridge ] Node client sk< %s > ---> server %d@%s failed, %v", sk, inStream.ID(), serverNodeId, err)
		return
	}

	sf.log.Infof("[ Bridge ] Node client %d@sk< %s > ---> server %d@%s created", targetStream.ID(), sk, inStream.ID(), serverNodeId)
	defer func() {
		targetStream.Close()
		sf.log.Infof("[ Bridge ] Node client %d@sk< %s > ---> server %d@%s released", targetStream.ID(), sk, inStream.ID(), serverNodeId)
	}()

	err = sword.Binding.Proxy(targetStream, inStream)
	if err != nil && err != io.EOF {
		sf.log.Errorf("[ Bridge ] proxying, %s", err)
	}
}

type BridgeOption func(b *Bridge)

func WithBridgeLogger(l logger.Logger) BridgeOption {
	return func(b *Bridge) {
		if l != nil {
			b.log = l
		}
	}
}
