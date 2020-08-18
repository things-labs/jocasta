package cs

import (
	"context"
	"crypto/tls"
	"net"
	"time"
)

// TCPDialer tcp dialer
type TCPDialer struct {
	Config      *tls.Config
	Timeout     time.Duration
	Forward     Dialer
	AfterChains AdornConnsChain
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
		nil,
		sf.AfterChains,
	}
	if sf.Config != nil {
		d.Chains = AdornConnsChain{adornTLS(sf.Config)}
	}
	return d.DialContext(ctx, network, addr)
}

// adornTLS tls chain
func adornTLS(conf *tls.Config) func(conn net.Conn) net.Conn {
	return func(conn net.Conn) net.Conn {
		return tls.Client(conn, conf)
	}
}
