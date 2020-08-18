package cs

import (
	"crypto/tls"
	"net"
	"sync"

	"github.com/thinkgos/jocasta/lib/gpool"
)

// TCPServer tcp server
type TCPServer struct {
	Addr   string
	Config *tls.Config // if not nil it will use tls

	Status      chan error
	GoPool      gpool.Pool
	AfterChains AdornConnsChain
	Handler     Handler

	mu sync.Mutex
	ln net.Listener
}

// ListenAndServe listen and serve
func (sf *TCPServer) ListenAndServe() error {
	ln, err := net.Listen("tcp", sf.Addr)
	if sf.Config != nil {
		ln = tls.NewListener(ln, sf.Config)
	}

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
