package cs

import (
	"net"
	"time"

	"github.com/xtaci/kcp-go/v5"

	"github.com/thinkgos/jocasta/connection/csnappy"
)

// KCPDialer KCP client dialer
type KCPDialer struct {
	Config KcpConfig
}

// DialTimeout dial KCP server
func (sf *KCPDialer) DialTimeout(address string, _ time.Duration) (net.Conn, error) {
	conn, err := kcp.DialWithOptions(address, sf.Config.Block, sf.Config.DataShard, sf.Config.ParityShard)
	if err != nil {
		return nil, err
	}
	conn.SetStreamMode(true)
	conn.SetWriteDelay(true)
	conn.SetNoDelay(sf.Config.NoDelay, sf.Config.Interval, sf.Config.Resend, sf.Config.NoCongestion)
	conn.SetMtu(sf.Config.MTU)
	conn.SetWindowSize(sf.Config.SndWnd, sf.Config.RcvWnd)
	conn.SetACKNoDelay(sf.Config.AckNodelay)
	if sf.Config.NoComp {
		return conn, err
	}
	return csnappy.New(conn), err
}
