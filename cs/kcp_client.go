package cs

import (
	"context"
	"net"

	"github.com/xtaci/kcp-go/v5"
)

// KCPDialer KCP client dialer
type KCPDialer struct {
	Config      KcpConfig
	AfterChains AdornConnsChain
}

// Dial connects to the address on the named network.
func (sf *KCPDialer) Dial(network, addr string) (net.Conn, error) {
	return sf.DialContext(context.Background(), network, addr)
}

// DialContext connects to the address on the named network using the provided context.
func (sf *KCPDialer) DialContext(_ context.Context, _, addr string) (net.Conn, error) {
	conn, err := kcp.DialWithOptions(addr, sf.Config.Block, sf.Config.DataShard, sf.Config.ParityShard)
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
