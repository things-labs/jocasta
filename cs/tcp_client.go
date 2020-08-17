package cs

import (
	"context"
	"net"
	"time"
)

// TCPDialer tcp dialer
type TCPDialer struct {
	Compress     bool
	Timeout      time.Duration
	Forward      Dialer
	BeforeChains AdornConnsChain
	AfterChains  AdornConnsChain
}

// Dial connects to the address on the named network.
func (sf *TCPDialer) Dial(network, addr string) (net.Conn, error) {
	return sf.DialContext(context.Background(), network, addr)
}

// DialContext connects to the address on the named network using the provided context.
func (sf *TCPDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	d := NetDialer{
		sf.Timeout,
		sf.Forward,
		AdornConnsChain{
			AdornCsnappy(sf.Compress),
		},
		sf.BeforeChains,
		sf.AfterChains,
	}
	return d.DialContext(ctx, network, addr)
}
