package cs

import (
	"context"
	"net"
	"time"
)

// TCPDialer tcp dialer
type TCPDialer struct {
	Timeout          time.Duration   // timeout for dial
	BaseAdorn        AdornConn       // base adorn conn
	AfterAdornChains AdornConnsChain // chains after base
	Forward          Dialer          // if set it will use forward.
}

// Dial connects to the address on the named network.
func (sf *TCPDialer) Dial(network, addr string) (net.Conn, error) {
	return sf.DialContext(context.Background(), network, addr)
}

// DialContext connects to the address on the named network using the provided context.
func (sf *TCPDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	var d Dialer = &net.Dialer{Timeout: sf.Timeout}

	if sf.Forward != nil {
		d = sf.Forward
	}

	contextDial := func(ctx context.Context, network, addr string) (net.Conn, error) {
		return DialContext(ctx, d, network, addr)
	}
	if f, ok := d.(ContextDialer); ok {
		contextDial = f.DialContext
	}

	conn, err := contextDial(ctx, network, addr)
	if err != nil {
		return nil, err
	}
	if sf.BaseAdorn != nil {
		conn = sf.BaseAdorn(conn)
	}
	for _, chain := range sf.AfterAdornChains {
		conn = chain(conn)
	}
	return conn, nil
}

// DialContext dial context with dialer
// WARNING: this can leak a goroutine for as long as the underlying Dialer implementation takes to timeout
// A Conn returned from a successful Dial after the context has been cancelled will be immediately closed.
func DialContext(ctx context.Context, d Dialer, network, address string) (net.Conn, error) {
	var conn net.Conn
	var err error

	done := make(chan struct{}, 1)
	go func() {
		conn, err = d.Dial(network, address)
		close(done)
		if conn != nil && ctx.Err() != nil {
			conn.Close()
		}
	}()
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case <-done:
	}
	return conn, err
}
