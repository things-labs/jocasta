package cs

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"

	"github.com/thinkgos/jocasta/connection/cencrypt"
	"github.com/thinkgos/jocasta/connection/csnappy"
	"github.com/thinkgos/jocasta/lib/encrypt"
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

// JumperTCP tcp jumper
type JumperTCP struct {
	*Jumper
	Compress bool
}

// DialTimeout tcp dialer
func (sf *JumperTCP) DialTimeout(address string, timeout time.Duration) (net.Conn, error) {
	d := dialer{sf.Jumper}

	conn, err := d.DialTimeout(address, timeout)
	if err != nil {
		return nil, err
	}
	if sf.Compress {
		conn = csnappy.New(conn)
	}
	return conn, nil
}

// JumperTCPTls tcp tls jumper
type JumperTCPTls struct {
	*Jumper
	CaCert []byte
	Cert   []byte
	Key    []byte
	Single bool
}

// DialTimeout tcp tls dialer
func (sf *JumperTCPTls) DialTimeout(address string, timeout time.Duration) (net.Conn, error) {
	var err error
	var conf *tls.Config

	if sf.Single {
		conf, err = SingleTLSConfig(sf.CaCert)
	} else {
		conf, err = TLSConfig(sf.Cert, sf.Key, sf.CaCert)
	}
	if err != nil {
		return nil, err
	}

	d := dialer{sf.Jumper}
	conn, err := d.DialTimeout(address, timeout)
	if err != nil {
		return nil, err
	}
	return tls.Client(conn, conf), nil
}

// JumperStcp stcp jumper
type JumperStcp struct {
	*Jumper
	Method   string
	Password string
	Compress bool
}

// DialTimeout stcp dialer
func (sf *JumperStcp) DialTimeout(address string, timeout time.Duration) (net.Conn, error) {
	cip, err := encrypt.NewCipher(sf.Method, sf.Password)
	if err != nil {
		return nil, err
	}
	d := dialer{sf.Jumper}
	conn, err := d.DialTimeout(address, timeout)
	if err != nil {
		return nil, err
	}
	if sf.Compress {
		conn = csnappy.New(conn)
	}
	return cencrypt.New(conn, cip), nil
}

type dialer struct {
	*Jumper
}

func (sf *dialer) DialTimeout(addr string, timeout time.Duration) (net.Conn, error) {
	switch sf.proxyURL.Scheme {
	case "https":
		return sf.dialHTTPS(addr, timeout)
	case "socks5":
		return sf.dialSOCKS5(addr, timeout)
	default:
		return nil, fmt.Errorf("unkown scheme of %s", sf.proxyURL.String())
	}
}

func (sf *dialer) dialHTTPS(addr string, timeout time.Duration) (net.Conn, error) {
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

func (sf *dialer) dialSOCKS5(addr string, timeout time.Duration) (net.Conn, error) {
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
