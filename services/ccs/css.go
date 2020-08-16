package ccs

import (
	"context"
	"fmt"
	"net"
	"net/url"
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
	// 不为空,使用相应代理, 支持tcp, tls, stcp
	ProxyURL *url.URL //only client used
}

// Dialer Client dialer
type Dialer struct {
	Protocol    string
	Timeout     time.Duration
	PreChains   cs.AdornConnsChain
	AfterChains cs.AdornConnsChain
	Config
}

// Dial connects to the address on the named network.
func (sf *Dialer) Dial(network, addr string) (net.Conn, error) {
	return sf.DialContext(context.Background(), network, addr)
}

// DialContext connects to the address on the named network using the provided context.

func (sf *Dialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	var d cs.ContextDialer
	var forward cs.Dialer

	if sf.ProxyURL != nil {
		switch sf.ProxyURL.Scheme {
		case "socks5":
			forward = cs.Socks5{
				ProxyHost: sf.ProxyURL.Host,
				Timeout:   sf.Timeout,
				Auth:      cs.ProxyAuth(sf.ProxyURL),
			}
		case "https":
			forward = cs.HTTPS{
				ProxyHost: sf.ProxyURL.Host,
				Timeout:   sf.Timeout,
				Auth:      cs.ProxyAuth(sf.ProxyURL),
			}
		default:
			return nil, fmt.Errorf("unkown scheme of %s", sf.ProxyURL.String())
		}
	}

	switch sf.Protocol {
	case "tcp":
		d = &cs.TCPDialer{
			Compress:    sf.Compress,
			Timeout:     sf.Timeout,
			Forward:     forward,
			PreChains:   sf.PreChains,
			AfterChains: sf.AfterChains,
		}
	case "tls":
		d = &cs.TCPTlsDialer{
			CaCert:      sf.CaCert,
			Cert:        sf.Cert,
			Key:         sf.Key,
			Single:      sf.SingleTLS,
			Timeout:     sf.Timeout,
			Forward:     forward,
			PreChains:   sf.PreChains,
			AfterChains: sf.AfterChains,
		}
	case "stcp":
		d = &cs.StcpDialer{
			Method:      sf.STCPMethod,
			Password:    sf.STCPPassword,
			Compress:    sf.Compress,
			Timeout:     sf.Timeout,
			Forward:     forward,
			PreChains:   sf.PreChains,
			AfterChains: sf.AfterChains,
		}
	case "kcp":
		d = &cs.KCPDialer{Config: sf.KcpConfig}
	default:
		return nil, fmt.Errorf("protocol support one of <tcp|tls|stcp|kcp> but give <%s>", sf.Protocol)
	}

	return d.DialContext(ctx, network, addr)
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
