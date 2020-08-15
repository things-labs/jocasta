package cs

import (
	"crypto/tls"
	"net"
	"time"

	"github.com/thinkgos/jocasta/connection/cencrypt"
	"github.com/thinkgos/jocasta/connection/csnappy"
	"github.com/thinkgos/jocasta/lib/encrypt"
)

// JumperTCP tcp jumper
type JumperTCP struct {
	*Jumper
	Compress bool
}

// DialTimeout tcp dialer
func (sf *JumperTCP) DialTimeout(address string, timeout time.Duration) (net.Conn, error) {
	conn, err := sf.Jumper.dialTimeout(address, timeout)
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

	conn, err := sf.Jumper.dialTimeout(address, timeout)
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
	conn, err := sf.Jumper.dialTimeout(address, timeout)
	if err != nil {
		return nil, err
	}
	if sf.Compress {
		conn = csnappy.New(conn)
	}
	return cencrypt.New(conn, cip), nil
}
