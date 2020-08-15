package cs

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

// Jumper 跳板机
type Jumper struct {
	proxyURL *url.URL
}

// ValidJumperProxyURL 校验proxyURL是否正确
func ValidJumperProxyURL(proxyURL string) bool {
	_, err := parseProxyURL(proxyURL)
	return err == nil
}

// parseProxyURL parse proxy url
func parseProxyURL(proxyURL string) (*url.URL, error) {
	if strings.HasPrefix(proxyURL, "socks5://") ||
		strings.HasPrefix(proxyURL, "https://") {
		return url.Parse(proxyURL)
	}
	return nil, errors.New("invalid proxy url")
}

// NewJumper 创建跳板
// proxyURL格式如下
// https://username:password@host:port
// https://host:port
// socks5://username:password@host:port
// socks5://host:port
func NewJumper(proxyURL string) (*Jumper, error) {
	u, err := parseProxyURL(proxyURL)
	if err != nil {
		return nil, err
	}
	return &Jumper{u}, nil
}

func (sf *Jumper) dialTimeout(addr string, timeout time.Duration) (net.Conn, error) {
	switch sf.proxyURL.Scheme {
	case "https":
		return sf.dialHTTPS(addr, timeout)
	case "socks5":
		return sf.dialSOCKS5(addr, timeout)
	default:
		return nil, fmt.Errorf("unkown scheme of %s", sf.proxyURL.String())
	}
}

func (sf *Jumper) dialHTTPS(addr string, timeout time.Duration) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", sf.proxyURL.Host, timeout)
	if err != nil {
		return nil, err
	}
	pb := new(bytes.Buffer)
	pb.WriteString(fmt.Sprintf("CONNECT %s HTTP/1.1\r\n", addr))
	pb.WriteString(fmt.Sprintf("Host: %s\r\n", addr))
	pb.WriteString(fmt.Sprintf("Proxy-Host: %s\r\n", addr))
	pb.WriteString("Proxy-Connection: Keep-Alive\r\n")
	pb.WriteString("Connection: Keep-Alive\r\n")
	if sf.proxyURL.User != nil {
		p, _ := sf.proxyURL.User.Password()
		u := fmt.Sprintf("%s:%s", sf.proxyURL.User.Username(), p)
		pb.WriteString(fmt.Sprintf("Proxy-Authorization: Basic %s\r\n", base64.StdEncoding.EncodeToString([]byte(u))))
	}
	pb.WriteString("\r\n")

	_, err = conn.Write(pb.Bytes())
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("connecting to proxy, %#v", err)
	}

	reply := make([]byte, 1024)
	conn.SetDeadline(time.Now().Add(timeout)) // nolint: errcheck
	n, err := conn.Read(reply)
	conn.SetDeadline(time.Time{}) // nolint: errcheck
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("ead reply from proxy, %#v", err)
	}
	if !bytes.Contains(reply[:n], []byte("200")) {
		conn.Close()
		return nil, fmt.Errorf("greeting to proxy, response %s", string(reply[:n]))
	}
	return conn, nil
}

func (sf *Jumper) dialSOCKS5(addr string, timeout time.Duration) (net.Conn, error) {
	var auth *proxy.Auth

	if sf.proxyURL.User != nil {
		pwd, _ := sf.proxyURL.User.Password()
		auth = &proxy.Auth{
			User:     sf.proxyURL.User.Username(),
			Password: pwd,
		}
	}

	dialSocksProxy, err := proxy.SOCKS5("tcp", sf.proxyURL.Host, auth, directTimeout{timeout: timeout})
	if err != nil {
		return nil, fmt.Errorf("connecting to proxy, %+v", err)
	}
	return dialSocksProxy.Dial("tcp", addr)
}

type directTimeout struct {
	timeout time.Duration
}

func (s directTimeout) Dial(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, s.timeout)
}
