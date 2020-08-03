package cs

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net"
	"time"

	"github.com/thinkgos/jocasta/connection/cencrypt"
	"github.com/thinkgos/jocasta/connection/csnappy"
	"github.com/thinkgos/jocasta/lib/encrypt"
)

// DialTcpTimeout dial tcp with timeout
func DialTcpTimeout(address string, compress bool, timeout time.Duration) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return nil, err
	}
	if compress {
		conn = csnappy.New(conn)
	}
	return conn, nil
}

// DialTcpTlsTimeout dial tcp tls with timeout
func DialTcpTlsTimeout(address string, cert, key, caCert []byte, timeout time.Duration) (*tls.Conn, error) {
	conf, err := TlsConfig(cert, key, caCert)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return nil, err
	}
	return tls.Client(conn, conf), err
}

// DialTcpSingleTlsTimeout dial single tls with timeout
func DialTcpSingleTlsTimeout(address string, caCertBytes []byte, timeout time.Duration) (*tls.Conn, error) {
	conf, err := SingleTlsConfig(caCertBytes)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return nil, err
	}
	return tls.Client(conn, conf), err
}

// DialStcpTimeout dial tcps with timeout
func DialStcpTimeout(address, method, password string, compress bool, timeout time.Duration) (net.Conn, error) {
	cip, err := encrypt.NewCipher(method, password)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return nil, err
	}
	if compress {
		conn = csnappy.New(conn)
	}
	return cencrypt.New(conn, cip), nil
}

// TlsConfig tls config
func TlsConfig(cert, key, caCert []byte) (*tls.Config, error) {
	certificate, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}
	serverCertPool := x509.NewCertPool()
	caBytes := cert
	if caCert != nil {
		caBytes = caCert
	}
	ok := serverCertPool.AppendCertsFromPEM(caBytes)
	if !ok {
		return nil, errors.New("failed to parse root certificate")
	}
	block, _ := pem.Decode(caBytes)
	if block == nil {
		return nil, errors.New("failed to parse certificate PEM")
	}
	x509Cert, err1 := x509.ParseCertificate(block.Bytes)
	if err1 != nil || x509Cert == nil {
		return nil, errors.New("failed to parse block")
	}

	return &tls.Config{
		RootCAs:            serverCertPool,
		Certificates:       []tls.Certificate{certificate},
		InsecureSkipVerify: true,
		ServerName:         x509Cert.Subject.CommonName,
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			opts := x509.VerifyOptions{
				Roots: serverCertPool,
			}
			for _, rawCert := range rawCerts {
				cert, _ := x509.ParseCertificate(rawCert)
				_, err := cert.Verify(opts)
				if err != nil {
					return err
				}
			}
			return nil
		},
	}, nil
}

// SingleTlsConfig single tls config
func SingleTlsConfig(caCertBytes []byte) (*tls.Config, error) {
	if len(caCertBytes) == 0 {
		return nil, errors.New("invalid root certificate")
	}

	serverCertPool := x509.NewCertPool()
	ok := serverCertPool.AppendCertsFromPEM(caCertBytes)
	if !ok {
		return nil, errors.New("failed to parse root certificate")
	}
	return &tls.Config{
		RootCAs:            serverCertPool,
		InsecureSkipVerify: true,
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			opts := x509.VerifyOptions{
				Roots: serverCertPool,
			}
			for _, rawCert := range rawCerts {
				cert, _ := x509.ParseCertificate(rawCert)
				_, err := cert.Verify(opts)
				if err != nil {
					return err
				}
			}
			return nil
		},
	}, nil
}
