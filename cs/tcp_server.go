package cs

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"sync"

	"github.com/thinkgos/jocasta/connection/cencrypt"
	"github.com/thinkgos/jocasta/connection/csnappy"
	"github.com/thinkgos/jocasta/lib/encrypt"
	"github.com/thinkgos/jocasta/lib/gpool"
)

// TCPServer tcp server
type TCPServer struct {
	Addr     string
	Compress bool
	Status   chan error
	GoPool   gpool.Pool
	Handler  Handler

	mu sync.Mutex
	ln net.Listener
}

func (sf *TCPServer) ListenAndServe() error {
	ln, err := net.Listen("tcp", sf.Addr)
	if err != nil {
		setStatus(sf.Status, err)
		return err
	}
	defer ln.Close()

	sf.mu.Lock()
	sf.ln = ln
	sf.mu.Unlock()
	setStatus(sf.Status, nil)
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		gpool.Go(sf.GoPool, func() {
			if sf.Compress {
				conn = csnappy.New(conn)
			}
			sf.Handler.ServerConn(conn)
		})
	}
}

// LocalAddr local listen address
func (sf *TCPServer) LocalAddr() (addr string) {
	sf.mu.Lock()
	if sf.ln != nil {
		addr = sf.ln.Addr().String()
	}
	sf.mu.Unlock()
	return
}

// Close close server
func (sf *TCPServer) Close() (err error) {
	sf.mu.Lock()
	if sf.ln != nil {
		err = sf.ln.Close()
	}
	sf.mu.Unlock()
	return
}

// StcpServer stcp server
type StcpServer struct {
	Addr     string
	Method   string
	Password string
	Compress bool
	Status   chan error
	GoPool   gpool.Pool
	Handler  Handler

	mu sync.Mutex
	ln net.Listener
}

// ListenAndServe listen and serve
func (sf *StcpServer) ListenAndServe() error {
	if sf.Method == "" || sf.Password == "" || !encrypt.HasCipherMethod(sf.Method) {
		err := errors.New("invalid method or password")
		setStatus(sf.Status, err)
		return err
	}
	_, err := encrypt.NewCipher(sf.Method, sf.Password)
	if err != nil {
		setStatus(sf.Status, err)
		return err
	}

	ln, err := net.Listen("tcp", sf.Addr)
	if err != nil {
		setStatus(sf.Status, err)
		return err
	}
	defer ln.Close()

	sf.mu.Lock()
	sf.ln = ln
	sf.mu.Unlock()

	setStatus(sf.Status, nil)
	for {
		conn, err := sf.ln.Accept()
		if err != nil {
			return err
		}
		gpool.Go(sf.GoPool, func() {
			if sf.Compress {
				conn = csnappy.New(conn)
			}
			// 这里应永远不出错,加密
			cip, _ := encrypt.NewCipher(sf.Method, sf.Password)
			sf.Handler.ServerConn(cencrypt.New(conn, cip))
		})
	}
}

// LocalAddr local listen address
func (sf *StcpServer) LocalAddr() (addr string) {
	sf.mu.Lock()
	if sf.ln != nil {
		addr = sf.ln.Addr().String()
	}
	sf.mu.Unlock()
	return
}

// Close close the server
func (sf *StcpServer) Close() (err error) {
	if sf.ln != nil {
		err = sf.ln.Close()
	}
	return
}

// TCPTlsServer tcp tls server
type TCPTlsServer struct {
	Addr    string
	CaCert  []byte
	Cert    []byte
	Key     []byte
	Single  bool
	Status  chan error
	GoPool  gpool.Pool
	Handler Handler

	mu sync.Mutex
	ln net.Listener
}

// ListenAndServe listen and serve
func (sf *TCPTlsServer) ListenAndServe() error {
	ln, err := sf.listenTCPTls()
	if err != nil {
		setStatus(sf.Status, err)
		return err
	}
	defer ln.Close()
	sf.mu.Lock()
	sf.ln = ln
	sf.mu.Unlock()
	setStatus(sf.Status, nil)
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		gpool.Go(sf.GoPool, func() { sf.Handler.ServerConn(conn) })
	}
}

// LocalAddr local listen address
func (sf *TCPTlsServer) LocalAddr() (addr string) {
	sf.mu.Lock()
	if sf.ln != nil {
		addr = sf.ln.Addr().String()
	}
	sf.mu.Unlock()
	return
}

// Close close the server
func (sf *TCPTlsServer) Close() (err error) {
	sf.mu.Lock()
	if sf.ln != nil {
		err = sf.ln.Close()
	}
	sf.mu.Unlock()
	return
}

func (sf *TCPTlsServer) listenTCPTls() (ln net.Listener, err error) {
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
