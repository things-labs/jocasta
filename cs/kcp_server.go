package cs

import (
	"net"
	"sync"

	"github.com/xtaci/kcp-go/v5"
	"go.uber.org/multierr"

	"github.com/thinkgos/jocasta/lib/gopool"
)

// KCPServer 传输,可选snappy压缩
// TODO: BUG 当对端关闭时,连接并未关闭,UDP无状态连接的原因
type KCPServer struct {
	Addr   string
	Config KcpConfig

	Status      chan error
	GoPool      gopool.Pool
	AfterChains AdornConnsChain
	Handler     Handler

	mu sync.Mutex
	ln net.Listener
}

// ListenAndServe listen and server
func (sf *KCPServer) ListenAndServe() error {
	ln, err := kcp.ListenWithOptions(sf.Addr, sf.Config.Block, sf.Config.DataShard, sf.Config.ParityShard)
	if err != nil {
		setStatus(sf.Status, err)
		return err
	}
	defer ln.Close() // nolint: errcheck
	err = multierr.Combine(
		ln.SetDSCP(sf.Config.DSCP),
		ln.SetReadBuffer(sf.Config.SockBuf),
		ln.SetWriteBuffer(sf.Config.SockBuf),
	)
	if err != nil {
		setStatus(sf.Status, err)
		return err
	}
	sf.mu.Lock()
	sf.ln = ln
	sf.mu.Unlock()
	setStatus(sf.Status, nil)
	for {
		conn, err := ln.AcceptKCP()
		if err != nil {
			return err
		}
		gopool.Go(sf.GoPool, func() {
			conn.SetStreamMode(true)
			conn.SetWriteDelay(true)
			conn.SetNoDelay(sf.Config.NoDelay, sf.Config.Interval, sf.Config.Resend, sf.Config.NoCongestion)
			conn.SetMtu(sf.Config.MTU)
			conn.SetWindowSize(sf.Config.SndWnd, sf.Config.RcvWnd)
			conn.SetACKNoDelay(sf.Config.AckNodelay)

			var c net.Conn = conn
			for _, chain := range sf.AfterChains {
				c = chain(c)
			}
			sf.Handler.ServerConn(c)
		})
	}
}

// LocalAddr return address
func (sf *KCPServer) LocalAddr() (addr string) {
	sf.mu.Lock()
	if sf.ln != nil {
		addr = sf.ln.Addr().String()
	}
	sf.mu.Unlock()
	return
}

// Close close kcp
func (sf *KCPServer) Close() (err error) {
	sf.mu.Lock()
	if sf.ln != nil {
		err = sf.ln.Close()
	}
	sf.mu.Unlock()
	return
}
