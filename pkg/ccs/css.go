package ccs

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/thinkgos/jocasta/cs"
	"github.com/thinkgos/jocasta/lib/gopool"
)

// Config config
type Config struct {
	// 仅tls有效
	TLSConfig cs.TLSConfig
	// 仅stcp有效
	StcpConfig cs.StcpConfig
	// 仅KCP有效
	KcpConfig cs.KcpConfig
	// 不为空,使用相应代理, 支持tcp, tls, stcp
	ProxyURL *url.URL //only client used
}

// Dialer Client dialer
type Dialer struct {
	Protocol    string
	Timeout     time.Duration
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
			Timeout:     sf.Timeout,
			AfterChains: sf.AfterChains,
			Forward:     forward,
		}
	case "tls":
		tlsConfig, err := sf.TLSConfig.ClientConfig()
		if err != nil {
			return nil, err
		}
		d = &cs.TCPDialer{
			Timeout:       sf.Timeout,
			BaseAdornConn: cs.BaseAdornTLSClient(tlsConfig),
			AfterChains:   sf.AfterChains,
			Forward:       forward,
		}
	case "stcp":
		if ok := sf.StcpConfig.Valid(); !ok {
			return nil, errors.New("invalid stcp config")
		}
		d = &cs.TCPDialer{
			Timeout:       sf.Timeout,
			BaseAdornConn: cs.BaseAdornEncrypt(sf.StcpConfig.Method, sf.StcpConfig.Password),
			AfterChains:   sf.AfterChains,
			Forward:       forward,
		}
	case "kcp":
		d = &cs.KCPDialer{
			Config:      sf.KcpConfig,
			AfterChains: sf.AfterChains,
		}
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
	GoPool      gopool.Pool
	AfterChains cs.AdornConnsChain
	Handler     cs.Handler

	status chan error
}

// RunListenAndServe run listen and server no-block, return error chan indicate server is run sucess or failed
func (sf *Server) RunListenAndServe() (cs.Server, <-chan error) {
	var srv cs.Server

	sf.status = make(chan error, 1)
	switch sf.Protocol {
	case "tcp":
		srv = &cs.TCPServer{
			Addr:        sf.Addr,
			AfterChains: sf.AfterChains,
			Handler:     sf.Handler,
			Status:      sf.status,
			GoPool:      sf.GoPool,
		}
	case "tls":
		tlsConfig, err := sf.TLSConfig.ServerConfig()
		if err != nil {
			sf.status <- err
			return nil, sf.status
		}
		srv = &cs.TCPServer{
			Addr:          sf.Addr,
			BaseAdornConn: cs.BaseAdornTLSServer(tlsConfig),
			AfterChains:   sf.AfterChains,
			Handler:       sf.Handler,
			Status:        sf.status,
			GoPool:        sf.GoPool,
		}
	case "stcp":
		if ok := sf.StcpConfig.Valid(); !ok {
			err := errors.New("invalid stcp config")
			sf.status <- err
			return nil, sf.status
		}
		srv = &cs.TCPServer{
			Addr:          sf.Addr,
			BaseAdornConn: cs.BaseAdornEncrypt(sf.StcpConfig.Method, sf.StcpConfig.Password),
			AfterChains:   sf.AfterChains,
			Handler:       sf.Handler,
			Status:        sf.status,
			GoPool:        sf.GoPool,
		}
	case "kcp":
		srv = &cs.KCPServer{
			Addr:        sf.Addr,
			Config:      sf.KcpConfig,
			AfterChains: sf.AfterChains,
			Handler:     sf.Handler,
			Status:      sf.status,
			GoPool:      sf.GoPool,
		}
	default:
		sf.status <- fmt.Errorf("not support protocol: %s", sf.Protocol)
		return nil, sf.status
	}

	gopool.Go(sf.GoPool, func() { _ = srv.ListenAndServe() })

	return srv, sf.status
}
