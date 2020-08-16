package cs

import (
	"context"
	"net"
	"time"

	"golang.org/x/net/proxy"

	"github.com/thinkgos/jocasta/connection/csnappy"
)

// TCPDialer tcp dialer
type TCPDialer struct {
	Compress bool
	Timeout  time.Duration
	Forward  proxy.Dialer
}

// Dial connects to the address on the named network.
func (sf *TCPDialer) Dial(network, addr string) (net.Conn, error) {
	return sf.DialContext(context.Background(), network, addr)
}

// DialContext connects to the address on the named network using the provided context.
func (sf *TCPDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	var d proxy.Dialer = &net.Dialer{Timeout: sf.Timeout}
	if sf.Forward != nil {
		d = sf.Forward
	}

	contextDial := func(ctx context.Context, network, addr string) (net.Conn, error) {
		return DialContext(ctx, d, network, addr)
	}
	if f, ok := d.(proxy.ContextDialer); ok {
		contextDial = f.DialContext
	}

	conn, err := contextDial(ctx, network, addr)
	if err != nil {
		return nil, err
	}
	if sf.Compress {
		conn = csnappy.New(conn)
	}
	return conn, nil
}
