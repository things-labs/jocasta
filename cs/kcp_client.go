package cs

import (
	"net"

	"github.com/xtaci/kcp-go/v5"

	"github.com/thinkgos/ppcore/connection/csnappy"
)

func DialKcp(address string, cfg KcpConfig) (net.Conn, error) {
	conn, err := kcp.DialWithOptions(address, cfg.Block, cfg.DataShard, cfg.ParityShard)
	if err != nil {
		return nil, err
	}
	conn.SetStreamMode(true)
	conn.SetWriteDelay(true)
	conn.SetNoDelay(cfg.NoDelay, cfg.Interval, cfg.Resend, cfg.NoCongestion)
	conn.SetMtu(cfg.MTU)
	conn.SetWindowSize(cfg.SndWnd, cfg.RcvWnd)
	conn.SetACKNoDelay(cfg.AckNodelay)
	if cfg.NoComp {
		return conn, err
	}
	return csnappy.New(conn), err
}
