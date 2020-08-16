package cs

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net"
	"time"

	"golang.org/x/net/proxy"
)

// Socks5 sock5 proxy
type Socks5 struct {
	ProxyHost string
	Auth      *proxy.Auth
	Timeout   time.Duration
	Forward   proxy.Dialer
}

// Dial socks5 dial
func (sf Socks5) Dial(network, addr string) (net.Conn, error) {
	var forward proxy.Dialer = &net.Dialer{Timeout: sf.Timeout}

	if sf.Forward != nil {
		forward = sf.Forward
	}

	dialSocksProxy, err := proxy.SOCKS5(network, sf.ProxyHost, sf.Auth, forward)
	if err != nil {
		return nil, fmt.Errorf("connecting to proxy, %+v", err)
	}
	return dialSocksProxy.Dial(network, addr)
}

// HTTPS https proxy
type HTTPS struct {
	ProxyHost string
	Auth      *proxy.Auth
	Timeout   time.Duration
}

// Dial https dial
func (sf HTTPS) Dial(network, addr string) (net.Conn, error) {
	conn, err := net.DialTimeout(network, sf.ProxyHost, sf.Timeout)
	if err != nil {
		return nil, err
	}
	pb := new(bytes.Buffer)
	pb.WriteString(fmt.Sprintf("CONNECT %s HTTP/1.1\r\n", addr))
	pb.WriteString(fmt.Sprintf("Host: %s\r\n", addr))
	pb.WriteString(fmt.Sprintf("Proxy-Host: %s\r\n", addr))
	pb.WriteString("Proxy-Connection: Keep-Alive\r\n")
	pb.WriteString("Connection: Keep-Alive\r\n")
	if sf.Auth != nil {
		u := fmt.Sprintf("%s:%s", sf.Auth.User, sf.Auth.Password)
		pb.WriteString(fmt.Sprintf("Proxy-Authorization: Basic %s\r\n", base64.StdEncoding.EncodeToString([]byte(u))))
	}
	pb.WriteString("\r\n")

	_, err = conn.Write(pb.Bytes())
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("connecting to proxy, %#v", err)
	}

	reply := make([]byte, 1024)
	conn.SetDeadline(time.Now().Add(sf.Timeout)) // nolint: errcheck
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
