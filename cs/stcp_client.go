package cs

import (
	"context"
	"net"
	"time"

	"github.com/thinkgos/jocasta/lib/encrypt"
)

// StcpDialer stcp dialer
type StcpDialer struct {
	Method       string
	Password     string
	Compress     bool
	Timeout      time.Duration
	Forward      Dialer
	BeforeChains AdornConnsChain
	AfterChains  AdornConnsChain
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

	d := NetDialer{
		sf.Timeout,
		sf.Forward,
		AdornConnsChain{
			AdornCencryptCip(cip), AdornCsnappy(sf.Compress),
		},
		sf.BeforeChains,
		sf.AfterChains,
	}
	return d.DialContext(ctx, network, addr)
}
