package cs

import (
	"net"
	"sync"

	"github.com/thinkgos/jocasta/connection/csnappy"
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

// ListenAndServe listen and serve
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
