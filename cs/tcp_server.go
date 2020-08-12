package cs

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"

	"github.com/thinkgos/jocasta/connection/cencrypt"
	"github.com/thinkgos/jocasta/connection/csnappy"
	"github.com/thinkgos/jocasta/lib/encrypt"
	"github.com/thinkgos/jocasta/lib/gpool"
)

type TCPServer struct {
	Addr     string
	ln       net.Listener
	Compress bool
	Handler  Handler
	GoPool   gpool.Pool
}

func (sf *TCPServer) ListenAndServe() error {
	preHandler := sf.Handler
	sf.Handler = HandlerFunc(func(c net.Conn) {
		// 压缩
		if sf.Compress {
			c = csnappy.New(c)
		}
		preHandler.ServerConn(c)
	})
	return sf.listenRawTCP()
}

func (sf *TCPServer) listenRawTCP() (err error) {
	sf.ln, err = net.Listen("tcp", sf.Addr)
	if err != nil {
		return err
	}
	defer sf.ln.Close()
	for {
		conn, err := sf.ln.Accept()
		if err != nil {
			return err
		}
		goFunc(sf.GoPool, func() { sf.Handler.ServerConn(conn) })
	}
}

func (sf *TCPServer) LocalAddr() (addr string) {
	if sf.ln != nil {
		addr = sf.ln.Addr().String()
	}
	return
}

func (sf *TCPServer) Close() (err error) {
	if sf.ln != nil {
		err = sf.ln.Close()
	}
	return
}

type StcpServer struct {
	Addr     string
	ln       net.Listener
	Method   string
	Password string
	Compress bool
	Handler  Handler
	GoPool   gpool.Pool
}

func (sf *StcpServer) ListenAndServe() error {
	if sf.Method == "" || sf.Password == "" || !encrypt.HasCipherMethod(sf.Method) {
		return errors.New("invalid method or password")
	}
	_, err := encrypt.NewCipher(sf.Method, sf.Password)
	if err != nil {
		return err
	}
	preFn := sf.Handler
	sf.Handler = HandlerFunc(func(c net.Conn) {
		// 压缩
		if sf.Compress {
			c = csnappy.New(c)
		}

		// 这里应永远不出错,加密
		cip, _ := encrypt.NewCipher(sf.Method, sf.Password)
		c = cencrypt.New(c, cip)
		preFn.ServerConn(c)
	})
	return sf.listenRawTCP()
}

func (sf *StcpServer) listenRawTCP() (err error) {
	sf.ln, err = net.Listen("tcp", sf.Addr)
	if err != nil {
		return err
	}
	defer sf.ln.Close()

	for {
		conn, err := sf.ln.Accept()
		if err != nil {
			return err
		}
		goFunc(sf.GoPool, func() { sf.Handler.ServerConn(conn) })
	}
}
func (sf *StcpServer) LocalAddr() (addr string) {
	if sf.ln != nil {
		addr = sf.ln.Addr().String()
	}
	return
}

func (sf *StcpServer) Close() (err error) {
	if sf.ln != nil {
		err = sf.ln.Close()
	}
	return
}

type TCPTlsServer struct {
	Addr    string
	ln      net.Listener
	CaCert  []byte
	Cert    []byte
	Key     []byte
	Single  bool
	Handler Handler
	GoPool  gpool.Pool
}

func (sf *TCPTlsServer) ListenAndServe() error {
	var err error

	sf.ln, err = sf.listenTCPTLS()
	if err != nil {
		return err
	}
	defer sf.ln.Close()
	for {
		conn, err := sf.ln.Accept()
		if err != nil {
			return err
		}
		goFunc(sf.GoPool, func() { sf.Handler.ServerConn(conn) })
	}
}

func (sf *TCPTlsServer) LocalAddr() (addr string) {
	if sf.ln != nil {
		addr = sf.ln.Addr().String()
	}
	return
}

func (sf *TCPTlsServer) Close() (err error) {
	if sf.ln != nil {
		err = sf.ln.Close()
	}
	return
}

func (sf *TCPTlsServer) listenTCPTLS() (ln net.Listener, err error) {
	var cert tls.Certificate

	cert, err = tls.X509KeyPair(sf.Cert, sf.Key)
	if err != nil {
		return
	}
	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	if !sf.Single {
		clientCertPool := x509.NewCertPool()
		caBytes := sf.Cert
		if sf.CaCert != nil {
			caBytes = sf.CaCert
		}
		ok := clientCertPool.AppendCertsFromPEM(caBytes)
		if !ok {
			err = errors.New("parse root certificate")
			return
		}
		config.ClientCAs = clientCertPool
		config.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return tls.Listen("tcp", sf.Addr, config)
}

func goFunc(goPool gpool.Pool, f func()) {
	if goPool != nil {
		goPool.Go(f)
	} else {
		go f()
	}
}
