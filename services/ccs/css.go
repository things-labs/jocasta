package ccs

import (
	"fmt"
	"net"
	"time"

	"github.com/thinkgos/jocasta/cs"
	"github.com/thinkgos/jocasta/lib/gpool"
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
	Protocol string
	Addr     string
	Config
	Handler cs.Handler
	GoPool  gpool.Pool
}

func (sf *Server) ListenAndServe() (cs.Channel, error) {
	var srv cs.Channel

	switch sf.Protocol {
	case "tcp":
		srv = &cs.TCPServer{
			Addr:     sf.Addr,
			Compress: sf.Compress,
			Handler:  sf.Handler,
			GoPool:   sf.GoPool,
		}
	case "tls":
		srv = &cs.TCPTlsServer{
			Addr:    sf.Addr,
			CaCert:  sf.CaCert,
			Cert:    sf.Cert,
			Key:     sf.Key,
			Single:  false,
			Handler: sf.Handler,
			GoPool:  sf.GoPool,
		}
	case "stcp":
		srv = &cs.StcpServer{
			Addr:     sf.Addr,
			Method:   sf.STCPMethod,
			Password: sf.STCPPassword,
			Compress: sf.Compress,
			Handler:  sf.Handler,
			GoPool:   sf.GoPool,
		}
	case "kcp":
		srv = &cs.KCPServer{
			Addr:    sf.Addr,
			Config:  sf.KcpConfig,
			Handler: sf.Handler,
			GoPool:  sf.GoPool,
		}
	default:
		return nil, fmt.Errorf("not support protocol: %s", sf.Protocol)
	}

	sword.Go(func() { _ = srv.ListenAndServe() })

	return srv, nil
}
