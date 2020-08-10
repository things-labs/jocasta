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
	// tls有效
	Cert      []byte
	Key       []byte
	CaCert    []byte
	SingleTls bool
	// stcp有效
	STCPMethod   string
	STCPPassword string
	// kcp有效
	KcpConfig cs.KcpConfig
	// stcp支持压缩
	Compress bool // 是否压缩
	// 支持tcp,stcp,tls
	Jumper *cs.Jumper //only client used
}

func DialTimeout(protocol, address string, timeout time.Duration, c Config) (net.Conn, error) {
	switch protocol {
	case "tcp":
		if c.Jumper != nil {
			return c.Jumper.DialTCPTimeout(address, timeout)
		}
		return cs.DialTCPTimeout(address, c.Compress, timeout)
	case "tls":
		if c.Jumper != nil {
			if c.SingleTls {
				return c.Jumper.DialTCPSingleTLSTimeout(address, c.CaCert, timeout)
			}
			return c.Jumper.DialTCPTLSTimeout(address, c.Cert, c.Key, c.CaCert, timeout)
		}
		if c.SingleTls {
			return cs.DialTCPSingleTLSTimeout(address, c.CaCert, timeout)
		}
		return cs.DialTCPTLSTimeout(address, c.Cert, c.Key, c.CaCert, timeout)
	case "stcp":
		if c.Jumper != nil {
			return c.Jumper.DialStcpTimeout(address, c.STCPMethod, c.STCPPassword, c.Compress, timeout)
		}
		return cs.DialStcpTimeout(address, c.STCPMethod, c.STCPPassword, c.Compress, timeout)
	case "kcp":
		return cs.DialKcp(address, c.KcpConfig)
	default:
		return nil, fmt.Errorf("protocol support one of <tcp|tls|stcp|kcp> but give %s", protocol)
	}
}

// not support udp
func NewAny(protocol, address string, handler func(conn net.Conn), c Config) (cs.Channel, error) {
	switch protocol {
	case "tcp":
		return cs.NewTCP(address, c.Compress, handler, cs.WithTCPGPool(sword.GPool))
	case "tls":
		return cs.NewTCPTLS(address, c.Cert, c.Key, c.CaCert, false, handler, cs.WithTCPGPool(sword.GPool))
	case "stcp":
		return cs.NewStcp(address, c.STCPMethod, c.STCPPassword, c.Compress, handler, cs.WithTCPGPool(sword.GPool))
	case "kcp":
		return cs.NewKcp(address, c.KcpConfig, handler, cs.WithKcpGPool(sword.GPool))
	default:
		return nil, fmt.Errorf("not support protocol: %s", protocol)
	}
}

func ListenAndServeAny(protocol, address string, handler func(conn net.Conn), c Config) (cs.Channel, error) {
	channel, err := NewAny(protocol, address, handler, c)
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
