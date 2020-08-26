package cs

import (
	"net"

	"github.com/xtaci/kcp-go/v5"
	"go.uber.org/multierr"

	"github.com/thinkgos/go-core-package/extnet"
)

// KCPListen 传输,可选snappy压缩
// TODO: BUG 当对端关闭时,连接并未关闭,UDP无状态连接的原因
type kcpListen struct {
	net.Listener
	Config      KcpConfig
	AfterChains extnet.AdornConnsChain
}

// ListenAndServe listen and server
func KCPListen(_, addr string, config KcpConfig, AfterChains ...extnet.AdornConn) (net.Listener, error) {
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
	conn.SetNoDelay(sf.Config.NoDelay, sf.Config.Interval, sf.Config.Resend, sf.Config.NoCongestion)
	conn.SetMtu(sf.Config.MTU)
	conn.SetWindowSize(sf.Config.SndWnd, sf.Config.RcvWnd)
	conn.SetACKNoDelay(sf.Config.AckNodelay)

	var c net.Conn = conn
	for _, chain := range sf.AfterChains {
		c = chain(c)
	}
	return c, nil
}
