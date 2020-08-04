package cs

import (
	"net"
	"runtime/debug"

	"github.com/thinkgos/jocasta/lib/gpool"
)

type Message struct {
	LocalAddr *net.UDPAddr
	SrcAddr   *net.UDPAddr
	Data      []byte
}

type UDP struct {
	common
	*net.UDPConn
	Handler func(listen *net.UDPConn, message Message)
	gPool   gpool.Pool
}

func NewUDP(addr string, handler func(listen *net.UDPConn, message Message), opts ...UDPOption) (*UDP, error) {
	c, err := newCommon(addr)
	if err != nil {
		return nil, err
	}
	p := &UDP{
		common:  c,
		Handler: handler,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p, nil
}

func (sf *UDP) ListenAndServe() (err error) {
	addr := &net.UDPAddr{IP: net.ParseIP(sf.ip), Port: sf.port}
	sf.UDPConn, err = net.ListenUDP("udp", addr)
	if err != nil {
		sf.status <- err
		return err
	}
	defer sf.UDPConn.Close()
	sf.status <- nil
	for {
		buf := make([]byte, 2048)
		n, srcAddr, err := sf.UDPConn.ReadFromUDP(buf)
		if err != nil {
			return err
		}
		data := buf[0:n]
		sf.submit(func() {
			defer func() {
				if e := recover(); e != nil {
					sf.log.Errorf("UDP handler crashed, %s , \ntrace: %s", e, string(debug.Stack()))
				}
			}()
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

func (sf *UDP) Addr() (addr string) {
	if sf.UDPConn != nil {
		addr = sf.UDPConn.LocalAddr().String()
	}
	return
}

func (sf *UDP) submit(f func()) {
	if sf.gPool == nil || sf.gPool.Submit(f) != nil {
		go f()
	}
}

// UDPOption udp option
type UDPOption func(udp *UDP)

// WithUDPGPool with gpool.Pool
func WithUDPGPool(pool gpool.Pool) UDPOption {
	return func(p *UDP) {
		if pool != nil {
			p.gPool = pool
		}
	}
}
