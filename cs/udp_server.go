package cs

import (
	"net"
	"strconv"

	"github.com/thinkgos/jocasta/lib/gpool"
)

// Message message
type Message struct {
	LocalAddr *net.UDPAddr
	SrcAddr   *net.UDPAddr
	Data      []byte
}

// UDP udp server
type UDP struct {
	Addr string
	*net.UDPConn
	GoPool  gpool.Pool
	Handler func(listen *net.UDPConn, message Message)
}

// ListenAndServe listen and server
func (sf *UDP) ListenAndServe() error {
	h, p, err := net.SplitHostPort(sf.Addr)
	if err != nil {
		return err
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		return err
	}

	addr := &net.UDPAddr{IP: net.ParseIP(h), Port: port}
	sf.UDPConn, err = net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	defer sf.UDPConn.Close()
	for {
		buf := make([]byte, 2048)
		n, srcAddr, err := sf.UDPConn.ReadFromUDP(buf)
		if err != nil {
			return err
		}
		data := buf[0:n]

		gpool.Go(sf.GoPool, func() {
			sf.Handler(sf.UDPConn, Message{
				LocalAddr: addr,
				SrcAddr:   srcAddr,
				Data:      data,
			})
		})
	}
}

// Close close the server
func (sf *UDP) Close() (err error) {
	if sf.UDPConn != nil {
		err = sf.UDPConn.Close()
	}
	return
}

// LocalAddr local address
func (sf *UDP) LocalAddr() (addr string) {
	if sf.UDPConn != nil {
		addr = sf.UDPConn.LocalAddr().String()
	}
	return
}
