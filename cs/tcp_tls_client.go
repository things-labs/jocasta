package cs

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net"
	"time"

	"golang.org/x/net/proxy"
)

// TCPTlsDialer tcp tls dialer
type TCPTlsDialer struct {
	CaCert      []byte
	Cert        []byte
	Key         []byte
	Single      bool
	Timeout     time.Duration
	Forward     proxy.Dialer
	PreChains   AdornConnsChain
	AfterChains AdornConnsChain
}

// Dial connects to the address on the named network.
func (sf *TCPTlsDialer) Dial(network, addr string) (net.Conn, error) {
	return sf.DialContext(context.Background(), network, addr)
}

// DialContext connects to the address on the named network using the provided context.
func (sf *TCPTlsDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
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

	d := Dialer{
		sf.Timeout,
		sf.Forward,
		AdornConnsChain{
			ChainTls(conf),
		},
		sf.PreChains,
		sf.AfterChains,
	}
	return d.DialContext(ctx, network, addr)
}

// TLSConfig tls config
func TLSConfig(cert, key, caCert []byte) (*tls.Config, error) {
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

// SingleTLSConfig single tls config
func SingleTLSConfig(caCertBytes []byte) (*tls.Config, error) {
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
