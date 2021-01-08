package ccs

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/thinkgos/x/extnet"
	"github.com/thinkgos/x/gopool"

	"github.com/thinkgos/jocasta/cs"
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
	AfterChains extnet.AdornConnsChain
	Config
}

// Dial connects to the address on the named network.
func (sf *Dialer) Dial(network, addr string) (net.Conn, error) {
	return sf.DialContext(context.Background(), network, addr)
}

// DialContext connects to the address on the named network using the provided context.
func (sf *Dialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	var d extnet.ContextDialer
	var forward extnet.Dialer

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
		d = &extnet.Client{
			Timeout:          sf.Timeout,
			AfterAdornChains: sf.AfterChains,
			Forward:          forward,
		}
	case "tls":
		tlsConfig, err := sf.TLSConfig.ClientConfig()
		if err != nil {
			return nil, err
		}
		d = &extnet.Client{
			Timeout:          sf.Timeout,
			BaseAdorn:        extnet.BaseAdornTLSClient(tlsConfig),
			AfterAdornChains: sf.AfterChains,
			Forward:          forward,
		}
	case "stcp":
		if ok := sf.StcpConfig.Valid(); !ok {
			return nil, errors.New("invalid stcp config")
		}
		d = &extnet.Client{
			Timeout:          sf.Timeout,
			BaseAdorn:        extnet.BaseAdornStcp(sf.StcpConfig.Method, sf.StcpConfig.Password),
			AfterAdornChains: sf.AfterChains,
			Forward:          forward,
		}
	case "kcp":
		d = &cs.KCPClient{
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
	AfterChains extnet.AdornConnsChain
	Handler     cs.Handler
}

// RunListenAndServe run listen and server no-block, return error chan indicate server is run sucess or failed
func (sf *Server) Listen() (net.Listener, error) {
	switch sf.Protocol {
	case "tcp":
		return extnet.ListenWith("tcp", sf.Addr, nil, sf.AfterChains...)
	case "tls":
		tlsConfig, err := sf.TLSConfig.ServerConfig()
		if err != nil {
			return nil, err
		}
		return extnet.ListenWith("tcp", sf.Addr, extnet.BaseAdornTLSServer(tlsConfig), sf.AfterChains...)
	case "stcp":
		if ok := sf.StcpConfig.Valid(); !ok {
			return nil, errors.New("invalid stcp config")
		}
		return extnet.ListenWith("tcp", sf.Addr, extnet.BaseAdornStcp(sf.StcpConfig.Method, sf.StcpConfig.Password), sf.AfterChains...)
	case "kcp":
		return cs.ListenKCP("", sf.Addr, sf.KcpConfig, sf.AfterChains...)
	default:
		return nil, fmt.Errorf("not support protocol: %s", sf.Protocol)
	}
}

func (sf *Server) Server(ln net.Listener) {
	defer ln.Close()
	if sf.Handler == nil {
		sf.Handler = new(cs.NopHandler)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		gopool.Go(sf.GoPool, func() { sf.Handler.ServerConn(conn) })
	}
}
