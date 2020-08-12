package ccs

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/thinkgos/jocasta/cs"
	"github.com/thinkgos/jocasta/pkg/sword"
)

type Config struct {
	// 仅tls有效
	Cert      []byte
	Key       []byte
	CaCert    []byte
	SingleTls bool
	// 仅stcp有效
	STCPMethod   string
	STCPPassword string
	// 仅KCP有效
	KcpConfig cs.KcpConfig
	// stcp支持压缩,tcp支持压缩,但jumper的tcp暂不支持压缩
	Compress bool // 是否压缩
	// 支持tcp(暂不支持压缩),tls,stcp
	Jumper *cs.Jumper //only client used
}

type Dialer struct {
	Config
}

func (sf *Dialer) DialTimeout(protocol string, address string, timeout time.Duration) (net.Conn, error) {
	var dialer cs.Dialer

	switch protocol {
	case "tcp":
		if sf.Jumper != nil {
			dialer = &cs.JumperTCP{Jumper: sf.Jumper}
		} else {
			dialer = &cs.TCPDialer{Compress: sf.Compress}
		}
	case "tls":
		if sf.Jumper != nil {
			dialer = &cs.JumperTCPTls{
				Jumper: sf.Jumper,
				CaCert: sf.CaCert, Cert: sf.Cert, Key: sf.Key, Single: sf.SingleTls,
			}
		} else {
			dialer = &cs.TCPTlsDialer{
				CaCert: sf.CaCert, Cert: sf.Cert, Key: sf.Key, Single: sf.SingleTls,
			}
		}
	case "stcp":
		if sf.Jumper != nil {
			dialer = &cs.JumperStcp{
				Jumper: sf.Jumper,
				Method: sf.STCPMethod, Password: sf.STCPPassword, Compress: sf.Compress,
			}
		} else {
			dialer = &cs.StcpDialer{
				Method: sf.STCPMethod, Password: sf.STCPPassword, Compress: sf.Compress,
			}
		}
	case "kcp":
		dialer = &cs.KCPDialer{Config: sf.KcpConfig}
	default:
		return nil, fmt.Errorf("protocol support one of <tcp|tls|stcp|kcp> but give %s", protocol)
	}
	return dialer.DialTimeout(address, timeout)
}

type Server struct {
	Config
	Handler func(conn net.Conn)
}

func (sf *Server) New(protocol, address string) (cs.Channel, error) {
	switch protocol {
	case "tcp":
		return cs.NewTCP(address, sf.Compress, sf.Handler, cs.WithTCPGPool(sword.GPool))
	case "tls":
		return cs.NewTCPTLS(address, sf.Cert, sf.Key, sf.CaCert, false, sf.Handler, cs.WithTCPGPool(sword.GPool))
	case "stcp":
		return cs.NewStcp(address, sf.STCPMethod, sf.STCPPassword, sf.Compress, sf.Handler, cs.WithTCPGPool(sword.GPool))
	case "kcp":
		return cs.NewKcp(address, sf.KcpConfig, sf.Handler, cs.WithKcpGPool(sword.GPool))
	default:
		return nil, fmt.Errorf("not support protocol: %s", protocol)
	}
}

func (sf *Server) ListenAndServe(protocol, address string) (cs.Channel, error) {
	channel, err := sf.New(protocol, address)
	if err != nil {
		return nil, err
	}

	sword.Go(func() { _ = channel.ListenAndServe() })

	t := time.NewTimer(time.Second)
	defer t.Stop()
	select {
	case err = <-channel.Status():
	case <-t.C:
		err = errors.New("waiting status timeout")
	}
	return channel, err
}

func ListenAndServeUDP(address string, handler func(listen *net.UDPConn, message cs.Message)) (*cs.UDP, error) {
	channel, err := cs.NewUDP(address, handler, cs.WithUDPGPool(sword.GPool))
	if err != nil {
		return nil, err
	}
	sword.Go(func() { _ = channel.ListenAndServe() })

	t := time.NewTimer(time.Second)
	defer t.Stop()
	select {
	case err = <-channel.Status():
	case <-t.C:
		err = errors.New("waiting status timeout")
	}
	return channel, err
}
