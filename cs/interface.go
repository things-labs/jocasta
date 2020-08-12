package cs

import (
	"net"
	"time"
)

type Dialer interface {
	DialTimeout(address string, timeout time.Duration) (net.Conn, error)
}
