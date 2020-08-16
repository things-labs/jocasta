package cs

import (
	"context"
	"net"
	"time"

	"golang.org/x/net/proxy"

	"github.com/thinkgos/jocasta/connection/cencrypt"
	"github.com/thinkgos/jocasta/connection/csnappy"
	"github.com/thinkgos/jocasta/lib/encrypt"
)

// StcpDialer stcp dialer
type StcpDialer struct {
	Method   string
	Password string
	Compress bool
	Timeout  time.Duration
	Forward  proxy.Dialer
}

// Dial dial the remote server
func (sf *StcpDialer) Dial(network, addr string) (net.Conn, error) {
	return sf.DialContext(context.Background(), network, addr)
}

// DialContext connects to the address on the named network using the provided context.
func (sf *StcpDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	cip, err := encrypt.NewCipher(sf.Method, sf.Password)
	if err != nil {
		return nil, err
	}

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
	return cencrypt.New(conn, cip), nil
}
