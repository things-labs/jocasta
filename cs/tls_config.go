package cs

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
)

// TCPTlsConfig tcp tls config
type TCPTlsConfig struct {
	CaCert []byte
	Cert   []byte
	Key    []byte
	Single bool
}

// ClientConfig client tls config
func (sf *TCPTlsConfig) ClientConfig() (*tls.Config, error) {
	if sf.Single {
		return singleTLSConfig(sf.CaCert)
	}
	return tlsConfig(sf.Cert, sf.Key, sf.CaCert)
}

// ServerConfig server tls config
func (sf *TCPTlsConfig) ServerConfig() (*tls.Config, error) {
	cert, err := tls.X509KeyPair(sf.Cert, sf.Key)
	if err != nil {
		return nil, err
	}
	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	if !sf.Single {
		clientCertPool := x509.NewCertPool()
		caBytes := sf.Cert
		if sf.CaCert != nil {
			caBytes = sf.CaCert
		}
		ok := clientCertPool.AppendCertsFromPEM(caBytes)
		if !ok {
			return nil, errors.New("parse root certificate")
		}
		config.ClientCAs = clientCertPool
		config.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return config, nil
}

// tlsConfig tls config
func tlsConfig(cert, key, caCert []byte) (*tls.Config, error) {
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

// singleTLSConfig single tls config
func singleTLSConfig(caCertBytes []byte) (*tls.Config, error) {
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