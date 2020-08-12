package cs

import (
	"net"

	"github.com/xtaci/kcp-go/v5"

	"github.com/thinkgos/jocasta/connection/csnappy"
	"github.com/thinkgos/jocasta/lib/gpool"
)

// KCPServer 传输,可选snappy压缩
type KCPServer struct {
	Addr    string
	l       net.Listener
	Config  KcpConfig
	Handler Handler
	GoPool  gpool.Pool
}

func (sf *KCPServer) ListenAndServe() error {
	lis, err := kcp.ListenWithOptions(sf.Addr, sf.Config.Block, sf.Config.DataShard, sf.Config.ParityShard)
	if err != nil {
		return err
	}
	defer lis.Close()

	if err = lis.SetDSCP(sf.Config.DSCP); err != nil {
		return err
	}
	if err = lis.SetReadBuffer(sf.Config.SockBuf); err != nil {
		return err
	}
	if err = lis.SetWriteBuffer(sf.Config.SockBuf); err != nil {
		return err
	}

	sf.l = lis
	for {
		conn, err := lis.AcceptKCP()
		if err != nil {
			return err
		}
		sf.goFunc(func() {
			var c net.Conn

			conn.SetStreamMode(true)
			conn.SetWriteDelay(true)
			conn.SetNoDelay(sf.Config.NoDelay, sf.Config.Interval, sf.Config.Resend, sf.Config.NoCongestion)
			conn.SetMtu(sf.Config.MTU)
			conn.SetWindowSize(sf.Config.SndWnd, sf.Config.RcvWnd)
			conn.SetACKNoDelay(sf.Config.AckNodelay)

			if sf.Config.NoComp {
				c = conn
			} else {
				c = csnappy.New(conn)
			}
			sf.Handler.ServerConn(c)
		})
	}
}

// Addr return address
func (sf *KCPServer) LocalAddr() (addr string) {
	if sf.l != nil {
		addr = sf.l.Addr().String()
	}
	return
}

// Close close kcp
func (sf *KCPServer) Close() (err error) {
	if sf.l != nil {
		err = sf.l.Close()
	}
	return
}

// 提交任务到协程
func (sf *KCPServer) goFunc(f func()) {
	if sf.GoPool != nil {
		sf.GoPool.Go(f)
	} else {
		go f()
	}
}
