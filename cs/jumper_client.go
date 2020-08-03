package cs

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"time"

	"golang.org/x/net/proxy"

	"github.com/thinkgos/jocasta/connection/cencrypt"
	"github.com/thinkgos/jocasta/connection/csnappy"
	"github.com/thinkgos/jocasta/lib/encrypt"
)

type Jumper struct {
	proxyURL *url.URL
}

func ValidJumper(proxyURL string) bool {
	_, err := url.Parse(proxyURL)
	return err == nil
}

// 创建跳板
// proxyURL格式如下
// https://username:password@host:port
// https://host:port
// socks5://username:password@host:port
// socks5://host:port
func NewJumper(proxyURL string) (*Jumper, error) {
	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}
	return &Jumper{proxyURL: u}, nil
}

func (sf *Jumper) DialTimeoutTcp(address string, timeout time.Duration) (net.Conn, error) {
	switch sf.proxyURL.Scheme {
	case "https":
		return sf.dialHTTPS(address, timeout)
	case "socks5":
		return sf.dialSOCKS5(address, timeout)
	default:
		return nil, fmt.Errorf("unkown scheme of %s", sf.proxyURL.String())
	}
}

func (sf *Jumper) DialTimeoutTcpTls(address string, certBytes, keyBytes, caCertBytes []byte, timeout time.Duration) (*tls.Conn, error) {
	conf, err := TlsConfig(certBytes, keyBytes, caCertBytes)
	if err != nil {
		return nil, err
	}
	conn, err := sf.DialTimeoutTcp(address, timeout)
	if err != nil {
		return nil, err
	}
	return tls.Client(conn, conf), nil
}

func (sf *Jumper) DialTimeoutTcpSingleTls(address string, caCertBytes []byte, timeout time.Duration) (*tls.Conn, error) {
	conf, err := SingleTlsConfig(caCertBytes)
	if err != nil {
		return nil, err
	}
	conn, err := sf.DialTimeoutTcp(address, timeout)
	if err != nil {
		return nil, err

	}
	return tls.Client(conn, conf), nil
}

func (sf *Jumper) DialTimeoutStcp(address, method, password string, compress bool, timeout time.Duration) (net.Conn, error) {
	cip, err := encrypt.NewCipher(method, password)
	if err != nil {
		return nil, err
	}
	conn, err := sf.DialTimeoutTcp(address, timeout)
	if err != nil {
		return nil, err
	}
	if compress {
		conn = csnappy.New(conn)
	}
	return cencrypt.New(conn, cip), nil
}

func (sf *Jumper) dialHTTPS(address string, timeout time.Duration) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", sf.proxyURL.Host, timeout)
	if err != nil {
		return nil, err
	}
	pb := new(bytes.Buffer)
	pb.WriteString(fmt.Sprintf("CONNECT %s HTTP/1.1\r\n", address))
	pb.WriteString(fmt.Sprintf("Host: %s\r\n", address))
	pb.WriteString(fmt.Sprintf("Proxy-Host: %s\r\n", address))
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
	conn.SetDeadline(time.Now().Add(timeout))
	n, err := conn.Read(reply)
	conn.SetDeadline(time.Time{})
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

func (sf *Jumper) dialSOCKS5(address string, timeout time.Duration) (net.Conn, error) {
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
	return dialSocksProxy.Dial("tcp", address)
}

type directTimeout struct {
	timeout time.Duration
}

func (s directTimeout) Dial(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, s.timeout)
}
