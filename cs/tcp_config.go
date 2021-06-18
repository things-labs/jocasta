package cs

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"

	"github.com/things-go/encrypt"
)

// StcpConfig stcp config
type StcpConfig struct {
	Method   string
	Password string
}

// Valid  valid the config
func (sf StcpConfig) Valid() bool {
	_, err := encrypt.NewCipher(sf.Method, sf.Password)
	return err == nil
}

// TLSConfig tcp tls config
// Single == true,  单向认证
//      客户端必须有提供ca证书
//      服务端必须有私钥和由ca签发的证书
// Single == false  双向认证
//      客户端必须有私钥和由ca签发的证书,ca证书可选(无将使用由ca签发的证书)
//      服务端必须有私钥和由ca签发的证书,ca证书可选(无将使用由ca签发的证书)
type TLSConfig struct {
	CaCert []byte
	Cert   []byte
	Key    []byte
	Single bool
}

// ClientConfig client tls config
func (sf *TLSConfig) ClientConfig() (*tls.Config, error) {
	if sf.Single {
		if len(sf.CaCert) == 0 {
			return nil, errors.New("invalid root certificate")
		}

		certPool := x509.NewCertPool()
		ok := certPool.AppendCertsFromPEM(sf.CaCert)
		if !ok {
			return nil, errors.New("failed to parse root certificate")
		}
		return &tls.Config{
			RootCAs:            certPool,
			InsecureSkipVerify: true,
			VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
				opts := x509.VerifyOptions{Roots: certPool}
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

	certificate, err := tls.X509KeyPair(sf.Cert, sf.Key)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	caBytes := sf.Cert
	if sf.CaCert != nil {
		caBytes = sf.CaCert
	}
	ok := certPool.AppendCertsFromPEM(caBytes)
	if !ok {
		return nil, errors.New("failed to parse root certificate")
	}
	block, _ := pem.Decode(caBytes)
	if block == nil {
		return nil, errors.New("failed to parse certificate PEM")
	}
	x509Cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil || x509Cert == nil {
		return nil, errors.New("failed to parse block")
	}

	return &tls.Config{
		RootCAs:            certPool,
		Certificates:       []tls.Certificate{certificate},
		InsecureSkipVerify: true,
		ServerName:         x509Cert.Subject.CommonName,
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			opts := x509.VerifyOptions{Roots: certPool}
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

// ServerConfig server tls config
func (sf *TLSConfig) ServerConfig() (*tls.Config, error) {
	certificate, err := tls.X509KeyPair(sf.Cert, sf.Key)
	if err != nil {
		return nil, err
	}
	config := &tls.Config{Certificates: []tls.Certificate{certificate}}
	if !sf.Single {
		certPool := x509.NewCertPool()
		caBytes := sf.Cert
		if sf.CaCert != nil {
			caBytes = sf.CaCert
		}
		ok := certPool.AppendCertsFromPEM(caBytes)
		if !ok {
			return nil, errors.New("parse root certificate")
		}
		config.ClientCAs = certPool
		config.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return config, nil
}
