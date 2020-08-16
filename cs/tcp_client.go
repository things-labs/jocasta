package cs

import (
	"context"
	"net"
	"time"

	"golang.org/x/net/proxy"
)

// TCPDialer tcp dialer
type TCPDialer struct {
	Compress    bool
	Timeout     time.Duration
	Forward     proxy.Dialer
	PreChains   AdornConnsChain
	AfterChains AdornConnsChain
}

// Dial connects to the address on the named network.
func (sf *TCPDialer) Dial(network, addr string) (net.Conn, error) {
	return sf.DialContext(context.Background(), network, addr)
}

// DialContext connects to the address on the named network using the provided context.
func (sf *TCPDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	d := Dialer{
		sf.Timeout,
		sf.Forward,
		AdornConnsChain{
			ChainCsnappy(sf.Compress),
		},
		sf.PreChains,
		sf.AfterChains,
	}
	return d.DialContext(ctx, network, addr)
}
