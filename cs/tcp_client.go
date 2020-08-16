package cs

import (
	"net"
	"time"

	"github.com/thinkgos/jocasta/connection/csnappy"
)

// TCPDialer tcp dialer
type TCPDialer struct {
	Compress bool
	Forward  Dialer
}

// DialTimeout dial the remote server
func (sf *TCPDialer) DialTimeout(address string, timeout time.Duration) (net.Conn, error) {
	var dial Dialer = TCPDirect{}

	if sf.Forward != nil {
		dial = sf.Forward
	}
	conn, err := dial.DialTimeout(address, timeout)

	if err != nil {
		return nil, err
	}
	if sf.Compress {
		conn = csnappy.New(conn)
	}
	return conn, nil
}
