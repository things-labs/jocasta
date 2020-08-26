package cs

import (
	"net"
)

// Message message
type Message struct {
	LocalAddr *net.UDPAddr
	SrcAddr   *net.UDPAddr
	Data      []byte
}
