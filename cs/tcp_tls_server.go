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

func (sf *TCPTlsServer) listen() (ln net.Listener, err error) {
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
