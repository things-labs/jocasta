package cs

import (
	"net"
	"time"

	"github.com/thinkgos/jocasta/connection/cencrypt"
	"github.com/thinkgos/jocasta/connection/csnappy"
	"github.com/thinkgos/jocasta/lib/encrypt"
)

// StcpDialer stcp dialer
type StcpDialer struct {
	Method   string
	Password string
	Compress bool
}

// DialTimeout dial the remote server
func (sf *StcpDialer) DialTimeout(address string, timeout time.Duration) (net.Conn, error) {
	cip, err := encrypt.NewCipher(sf.Method, sf.Password)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return nil, err
	}
	if sf.Compress {
		conn = csnappy.New(conn)
	}
	return cencrypt.New(conn, cip), nil
}
