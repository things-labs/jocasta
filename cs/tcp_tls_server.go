package cs

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"sync"

	"github.com/thinkgos/jocasta/lib/gpool"
)

// TCPTlsServer tcp tls server
type TCPTlsServer struct {
	Addr   string
	Config TCPTlsConfig

	Status      chan error
	GoPool      gpool.Pool
	AfterChains AdornConnsChain
	Handler     Handler

	mu sync.Mutex
	ln net.Listener
}

// ListenAndServe listen and serve
func (sf *TCPTlsServer) ListenAndServe() error {
	ln, err := sf.listen()
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
			for _, chain := range sf.AfterChains {
				conn = chain(conn)
			}
			sf.Handler.ServerConn(conn)
		})
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

func (sf *TCPTlsServer) listen() (net.Listener, error) {
	cert, err := tls.X509KeyPair(sf.Config.Cert, sf.Config.Key)
	if err != nil {
		return nil, err
	}
	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	if !sf.Config.Single {
		clientCertPool := x509.NewCertPool()
		caBytes := sf.Config.Cert
		if sf.Config.CaCert != nil {
			caBytes = sf.Config.CaCert
		}
		ok := clientCertPool.AppendCertsFromPEM(caBytes)
		if !ok {
			return nil, errors.New("parse root certificate")
		}
		config.ClientCAs = clientCertPool
		config.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return tls.Listen("tcp", sf.Addr, config)
}
