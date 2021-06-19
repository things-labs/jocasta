package cs

import (
	"net"

	"github.com/xtaci/kcp-go/v5"
	"go.uber.org/multierr"

	"github.com/thinkgos/jocasta/connection"
)

// ListenKCP 传输,可选snappy压缩
// TODO: BUG 当对端关闭时,连接并未关闭,UDP无状态连接的原因
type kcpListen struct {
	net.Listener
	config      KcpConfig
	afterChains connection.AdornConnsChain
}

// ListenKCP listen
func ListenKCP(_, addr string, config KcpConfig, AfterChains ...connection.AdornConn) (net.Listener, error) {
	ln, err := kcp.ListenWithOptions(addr, config.Block, config.DataShard, config.ParityShard)
	if err != nil {
		return nil, err
	}
	err = multierr.Combine(
		ln.SetDSCP(config.DSCP),
		ln.SetReadBuffer(config.SockBuf),
		ln.SetWriteBuffer(config.SockBuf),
	)
	if err != nil {
		return nil, err
	}
	return &kcpListen{
		ln,
		config,
		AfterChains,
	}, nil
}

func (sf *kcpListen) Accept() (net.Conn, error) {
	conn, err := sf.Listener.(*kcp.Listener).AcceptKCP()
	if err != nil {
		return nil, err
	}
	conn.SetStreamMode(true)
	conn.SetWriteDelay(true)
	conn.SetNoDelay(sf.config.NoDelay, sf.config.Interval, sf.config.Resend, sf.config.NoCongestion)
	conn.SetMtu(sf.config.MTU)
	conn.SetWindowSize(sf.config.SndWnd, sf.config.RcvWnd)
	conn.SetACKNoDelay(sf.config.AckNodelay)

	var c net.Conn = conn
	for _, chain := range sf.afterChains {
		c = chain(c)
	}
	return c, nil
}
