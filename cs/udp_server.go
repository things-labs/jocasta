package cs

import (
	"net"

	"github.com/thinkgos/jocasta/lib/gpool"
)

// Message message
type Message struct {
	LocalAddr *net.UDPAddr
	SrcAddr   *net.UDPAddr
	Data      []byte
}

// UDPServer udp server
type UDPServer struct {
	Addr   string
	Status chan error
	*net.UDPConn
	GoPool  gpool.Pool
	Handler func(listen *net.UDPConn, message Message)
}

// ListenAndServe listen and server
func (sf *UDPServer) ListenAndServe() error {
	// h, p, err := net.SplitHostPort(sf.Addr)
	// if err != nil {
	// 	setStatus(sf.Status, err)
	// 	return err
	// }
	// port, err := strconv.Atoi(p)
	// if err != nil {
	// 	setStatus(sf.Status, err)
	// 	return err
	// }
	//
	// addr := &net.UDPAddr{IP: net.ParseIP(h), Port: port}
	addr, err := net.ResolveUDPAddr("udp", sf.Addr)
	if err != nil {
		setStatus(sf.Status, err)
		return err
	}

	sf.UDPConn, err = net.ListenUDP("udp", addr)
	if err != nil {
		setStatus(sf.Status, err)
		return err
	}
	defer sf.UDPConn.Close()
	setStatus(sf.Status, nil)
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
func (sf *UDPServer) Close() (err error) {
	if sf.UDPConn != nil {
		err = sf.UDPConn.Close()
	}
	return
}

// LocalAddr local address
func (sf *UDPServer) LocalAddr() (addr string) {
	if sf.UDPConn != nil {
		addr = sf.UDPConn.LocalAddr().String()
	}
	return
}
