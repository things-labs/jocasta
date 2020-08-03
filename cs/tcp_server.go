package cs

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"runtime/debug"
	"strconv"

	"github.com/thinkgos/jocasta/connection/cencrypt"
	"github.com/thinkgos/jocasta/connection/csnappy"
	"github.com/thinkgos/jocasta/lib/encrypt"
	"github.com/thinkgos/jocasta/lib/gpool"
)

const (
	tcp    = iota // 无加密,可选择压缩
	tcptls        // tcp tls加密
	stcp          // 采用自定义加密方式,可选snappy压缩
)

type TCP struct {
	common
	ln            net.Listener
	caCert        []byte
	certOrMethod  []byte
	keyOrPassword []byte
	special       bool
	who           int
	handler       func(conn net.Conn)
	gPool         gpool.Pool
}

func NewTcp(addr string, compress bool, handler func(conn net.Conn), opts ...TcpOption) (*TCP, error) {
	c, err := newCommon(addr)
	if err != nil {
		return nil, err
	}

	p := &TCP{
		common:  c,
		special: compress,
		who:     tcp,
		handler: handler,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p, nil
}

func NewStcp(addr, method, password string, compress bool, handler func(conn net.Conn), opts ...TcpOption) (*TCP, error) {
	if method == "" || password == "" || !encrypt.HasCipherMethod(method) {
		return nil, errors.New("invalid method or password")
	}
	c, err := newCommon(addr)
	if err != nil {
		return nil, err
	}
	p := &TCP{
		common:        c,
		certOrMethod:  []byte(method),
		keyOrPassword: []byte(password),
		special:       compress,
		who:           stcp,
		handler:       handler,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p, nil
}

func NewTcpTls(addr string, certBytes, keyBytes, caCertBytes []byte, single bool, handler func(conn net.Conn), opts ...TcpOption) (*TCP, error) {
	com, err := newCommon(addr)
	if err != nil {
		return nil, err
	}
	p := &TCP{
		common:        com,
		certOrMethod:  certBytes,
		keyOrPassword: keyBytes,
		caCert:        caCertBytes,
		special:       single,
		who:           tcptls,
		handler:       handler,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p, nil
}

func (sf *TCP) ListenAndServe() error {
	switch sf.who {
	case tcp:
		return sf.listenAndServeTcp()
	case stcp:
		return sf.listenAndServeStcp()
	case tcptls:
		return sf.listenAndServeTcpTls()
	default:
		err := fmt.Errorf("unknown listen and serve who(%d)", sf.who)
		sf.status <- err
		return err
	}
}

func (sf *TCP) Close() (err error) {
	if sf.ln != nil {
		err = sf.ln.Close()
	}
	return
}

func (sf *TCP) Addr() (addr string) {
	if sf.ln != nil {
		addr = sf.ln.Addr().String()
	}
	return
}

func (sf *TCP) listenAndServeTcp() (err error) {
	preFn := sf.handler
	sf.handler = func(c net.Conn) {
		// 压缩
		if sf.special {
			c = csnappy.New(c)
		}
		preFn(c)
	}
	return sf.listenRawTCP()
}

func (sf *TCP) listenAndServeStcp() error {
	_, err := encrypt.NewCipher(string(sf.certOrMethod), string(sf.keyOrPassword))
	if err != nil {
		sf.status <- err
		return err
	}
	preFn := sf.handler
	sf.handler = func(c net.Conn) {
		// 压缩
		if sf.special {
			c = csnappy.New(c)
		}

		// 这里应永远不出错,加密
		cip, _ := encrypt.NewCipher(string(sf.certOrMethod), string(sf.keyOrPassword))
		c = cencrypt.New(c, cip)
		preFn(c)
	}
	return sf.listenRawTCP()
}

func (sf *TCP) listenAndServeTcpTls() (err error) {
	sf.ln, err = sf.listenTcpTls()
	if err != nil {
		sf.status <- err
		return err
	}
	defer sf.ln.Close()
	sf.status <- nil
	for {
		conn, err := sf.ln.Accept()
		if err != nil {
			return err
		}
		sf.submit(func() {
			defer func() {
				if e := recover(); e != nil {
					sf.log.Errorf("tls connection handler crashed, %s , \ntrace:%s", e, string(debug.Stack()))
				}
			}()
			sf.handler(conn)
		})
	}
}

func (sf *TCP) listenRawTCP() (err error) {
	sf.ln, err = net.Listen("tcp", net.JoinHostPort(sf.ip, strconv.Itoa(sf.port)))
	if err != nil {
		sf.status <- err
		return err
	}
	defer sf.ln.Close()

	sf.status <- nil
	for {
		conn, err := sf.ln.Accept()
		if err != nil {
			return err
		}
		sf.submit(func() {
			defer func() {
				if e := recover(); e != nil {
					sf.log.Errorf("tcp connection handler crashed, %s , \ntrace:%s", e, string(debug.Stack()))
				}
			}()
			sf.handler(conn)
		})
	}
}

func (sf *TCP) listenTcpTls() (ln net.Listener, err error) {
	var cert tls.Certificate

	cert, err = tls.X509KeyPair(sf.certOrMethod, sf.keyOrPassword)
	if err != nil {
		return
	}
	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	if !sf.special {
		clientCertPool := x509.NewCertPool()
		caBytes := sf.certOrMethod
		if sf.caCert != nil {
			caBytes = sf.caCert
		}
		ok := clientCertPool.AppendCertsFromPEM(caBytes)
		if !ok {
			err = errors.New("parse root certificate")
			return
		}
		config.ClientCAs = clientCertPool
		config.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return tls.Listen("tcp", net.JoinHostPort(sf.ip, strconv.Itoa(sf.port)), config)
}

func (sf *TCP) submit(f func()) {
	if sf.gPool == nil || sf.gPool.Submit(f) != nil {
		go f()
	}
}

type TcpOption func(*TCP)

func WithTcpGPool(pool gpool.Pool) TcpOption {
	return func(p *TCP) {
		if pool != nil {
			p.gPool = pool
		}
	}
}
