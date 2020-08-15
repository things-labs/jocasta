package ccs

import (
	"fmt"
	"net"
	"time"

	"github.com/thinkgos/jocasta/cs"
	"github.com/thinkgos/jocasta/lib/gpool"
)

// Config config
type Config struct {
	// 仅tls有效
	Cert      []byte
	Key       []byte
	CaCert    []byte
	SingleTLS bool
	// 仅stcp有效
	STCPMethod   string
	STCPPassword string
	// 仅KCP有效
	KcpConfig cs.KcpConfig
	// stcp支持压缩,tcp支持压缩,但jumper的tcp暂不支持压缩
	Compress bool // 是否压缩
	// 支持tcp(暂不支持压缩), tls, stcp
	Jumper *cs.Jumper //only client used
}

// Dialer Client dialer
type Dialer struct {
	Protocol string
	Config
}

// DialTimeout dial tthe remote server
func (sf *Dialer) DialTimeout(address string, timeout time.Duration) (net.Conn, error) {
	var dialer cs.Dialer

	switch sf.Protocol {
	case "tcp":
		if sf.Jumper != nil {
			dialer = &cs.JumperTCP{Jumper: sf.Jumper, Compress: sf.Compress}
		} else {
			dialer = &cs.TCPDialer{Compress: sf.Compress}
		}
	case "tls":
		if sf.Jumper != nil {
			dialer = &cs.JumperTCPTls{
				Jumper: sf.Jumper,
				CaCert: sf.CaCert, Cert: sf.Cert, Key: sf.Key, Single: sf.SingleTLS,
			}
		} else {
			dialer = &cs.TCPTlsDialer{
				CaCert: sf.CaCert, Cert: sf.Cert, Key: sf.Key, Single: sf.SingleTLS,
			}
		}
	case "stcp":
		if sf.Jumper != nil {
			dialer = &cs.JumperStcp{
				Method: sf.STCPMethod, Password: sf.STCPPassword, Compress: sf.Compress,
			}
			return nil, fmt.Errorf("protocol <stcp> not support jumper")
		}
		dialer = &cs.StcpDialer{
			Method: sf.STCPMethod, Password: sf.STCPPassword, Compress: sf.Compress,
		}
	case "kcp":
		dialer = &cs.KCPDialer{Config: sf.KcpConfig}
	default:
		return nil, fmt.Errorf("protocol support one of <tcp|tls|stcp|kcp> but give <%s>", sf.Protocol)
	}
	return dialer.DialTimeout(address, timeout)
}

// Server server
type Server struct {
	Protocol string
	Addr     string
	Config
	GoPool  gpool.Pool
	Handler cs.Handler

	status chan error
}

// RunListenAndServe run listen and server no-block, return error chan indicate server is run sucess or failed
func (sf *Server) RunListenAndServe() (cs.Server, <-chan error) {
	var srv cs.Server

	sf.status = make(chan error, 1)
	switch sf.Protocol {
	case "tcp":
		srv = &cs.TCPServer{
			Addr:     sf.Addr,
			Compress: sf.Compress,
			Status:   sf.status,
			GoPool:   sf.GoPool,
			Handler:  sf.Handler,
		}
	case "tls":
		srv = &cs.TCPTlsServer{
			Addr:    sf.Addr,
			CaCert:  sf.CaCert,
			Cert:    sf.Cert,
			Key:     sf.Key,
			Single:  sf.SingleTLS,
			Status:  sf.status,
			GoPool:  sf.GoPool,
			Handler: sf.Handler,
		}
	case "stcp":
		srv = &cs.StcpServer{
			Addr:     sf.Addr,
			Method:   sf.STCPMethod,
			Password: sf.STCPPassword,
			Compress: sf.Compress,
			Status:   sf.status,
			GoPool:   sf.GoPool,
			Handler:  sf.Handler,
		}
	case "kcp":
		srv = &cs.KCPServer{
			Addr:    sf.Addr,
			Config:  sf.KcpConfig,
			Status:  sf.status,
			GoPool:  sf.GoPool,
			Handler: sf.Handler,
		}
	default:
		sf.status <- fmt.Errorf("not support protocol: %s", sf.Protocol)
		return nil, sf.status
	}

	gpool.Go(sf.GoPool, func() { _ = srv.ListenAndServe() })

	return srv, sf.status
}
