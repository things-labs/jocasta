package cs

import (
	"net"
	"strconv"

	"github.com/thinkgos/jocasta/lib/gpool"
)

type Message struct {
	LocalAddr *net.UDPAddr
	SrcAddr   *net.UDPAddr
	Data      []byte
}

type UDP struct {
	Addr string
	*net.UDPConn
	Handler func(listen *net.UDPConn, message Message)
	GoPool  gpool.Pool
}

func (sf *UDP) ListenAndServe() error {
	h, port, err := net.SplitHostPort(sf.Addr)
	if err != nil {
		return err
	}
	p, err := strconv.Atoi(port)
	if err != nil {
		return err
	}

	addr := &net.UDPAddr{IP: net.ParseIP(h), Port: p}
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
		sf.goFunc(func() {
			sf.Handler(sf.UDPConn, Message{
				LocalAddr: addr,
				SrcAddr:   srcAddr,
				Data:      data,
			})
		})
	}
}

func (sf *UDP) Close() (err error) {
	if sf.UDPConn != nil {
		err = sf.UDPConn.Close()
	}
	return
}

func (sf *UDP) LocalAddr() (addr string) {
	if sf.UDPConn != nil {
		addr = sf.UDPConn.LocalAddr().String()
	}
	return
}

func (sf *UDP) goFunc(f func()) {
	if sf.GoPool == nil {
		sf.GoPool.Go(f)
	} else {
		go f()
	}
}
