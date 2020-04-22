// ca 证书生成
package cert

import (
	cryptoRand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"io/ioutil"
	"math/big"
	"math/rand"
	"net"
	"strings"
	"time"
)

// 文件后缀
const (
	base64Prefix   = "base64://"
	CertFileSuffix = ".crt"
	KeyFileSuffix  = ".key"
)

// Names 定义Name
type Names struct {
	Country          string // 国家
	Province         string // 省/州
	Locality         string // 地区
	Organization     string // 组织
	OrganizationUnit string // 组织单位
}

// Config 定义配置
type Config struct {
	CommonName string
	Names      Names
	Host       []string
	Expire     uint64 // 小时
}

// CreateSignFile 根据rootCA rootKey生成签发证书文件
func CreateSignFile(rootCA *x509.Certificate, rootKey *rsa.PrivateKey, filenamePrefix string, cfg Config) error {
	cert, key, err := CreateSign(rootCA, rootKey, cfg)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filenamePrefix+CertFileSuffix, cert, 0755)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filenamePrefix+KeyFileSuffix, key, 0755)
}

// CreateSign 根据rootCA rootKey生成签发证书
func CreateSign(rootCA *x509.Certificate, rootKey *rsa.PrivateKey, cfg Config) (cert []byte, key []byte, err error) {
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(rand.Int63()),
		Subject: pkix.Name{
			CommonName:         cfg.CommonName,
			Country:            []string{cfg.Names.Country},
			Organization:       []string{cfg.Names.Organization},
			OrganizationalUnit: []string{cfg.Names.OrganizationUnit},
			Province:           []string{cfg.Names.Province},
			Locality:           []string{cfg.Names.Locality},
		},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(time.Hour * time.Duration(cfg.Expire)),

		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageDataEncipherment,
		EmailAddresses:        []string{},
		IPAddresses:           []net.IP{},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}
	for _, host := range cfg.Host {
		if ip := net.ParseIP(host); ip != nil {
			tpl.IPAddresses = append(tpl.IPAddresses, ip)
		} else {
			tpl.DNSNames = append(tpl.DNSNames, host)
		}
	}

	//生成公钥私钥对
	priKey, err := rsa.GenerateKey(cryptoRand.Reader, 2048)
	if err != nil {
		return
	}
	cert, err = x509.CreateCertificate(cryptoRand.Reader, tpl, rootCA, &priKey.PublicKey, rootKey)
	if err != nil {
		return
	}
	// Generate cert
	cert = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert,
	})
	// Generate key
	key = pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priKey),
	})
	return
}

func CreateCAFile(filenamePrefix string, cfg Config) (err error) {
	ca, key, err := CreateCA(cfg)
	if err != nil {
		return
	}
	err = ioutil.WriteFile(filenamePrefix+CertFileSuffix, ca, 0755)
	if err != nil {
		return
	}
	err = ioutil.WriteFile(filenamePrefix+KeyFileSuffix, key, 0755)
	return
}

// CreateCA 生成ca证书
func CreateCA(cfg Config) (ca []byte, key []byte, err error) {
	privateKey, err := rsa.GenerateKey(cryptoRand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:         cfg.CommonName,
			Country:            []string{cfg.Names.Country},
			Organization:       []string{cfg.Names.Organization},
			OrganizationalUnit: []string{cfg.Names.OrganizationUnit},
			Province:           []string{cfg.Names.Province},
			Locality:           []string{cfg.Names.Locality},
		},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(time.Hour * time.Duration(cfg.Expire)),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	crt, err := x509.CreateCertificate(cryptoRand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return
	}
	// Generate cert
	ca = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: crt,
	})
	// Generate key
	key = pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	return
}

// ParseCrt 解析根证书
func ParseCrt(b []byte) (*x509.Certificate, error) {
	caBlock, _ := pem.Decode(b)
	return x509.ParseCertificate(caBlock.Bytes)
}

// ParseKey 解析私钥
func ParseKey(b []byte) (*rsa.PrivateKey, error) {
	keyBlock, _ := pem.Decode(b)
	return x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
}

// ParseCrtAndKey 解析根证书和私钥
func ParseCrtAndKey(crt, key []byte) (ca *x509.Certificate, privateKey *rsa.PrivateKey, err error) {
	ca, err = ParseCrt(crt)
	if err != nil {
		return
	}
	privateKey, err = ParseKey(key)
	return
}

// ParseCrtFile 解析ca文件
func ParseCrtFile(filename string) (*x509.Certificate, error) {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return ParseCrt(b)
}

func ReadCrtFile(filename string) ([]byte, error) {
	return ioutil.ReadFile(filename)
}

// ParseKeyFile 解析私钥文件
func ParseKeyFile(filename string) (*rsa.PrivateKey, error) {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return ParseKey(b)
}

func ReadKeyFile(filename string) ([]byte, error) {
	return ioutil.ReadFile(filename)
}

// ParseCrtAndKeyFile 解析ca和私钥文件
func ParseCrtAndKeyFile(crtFilename, keyFilename string) (ca *x509.Certificate, key *rsa.PrivateKey, err error) {
	ca, err = ParseCrtFile(crtFilename)
	if err != nil {
		return
	}
	key, err = ParseKeyFile(keyFilename)
	return
}

func ReadCrtAndKeyFile(crtFilename, keyFilename string) (crt []byte, key []byte, err error) {
	crt, err = ReadCrtFile(crtFilename)
	if err != nil {
		return
	}
	key, err = ReadKeyFile(keyFilename)
	return
}

func ParseTls(cert, key string) (certBytes, keyBytes []byte, err error) {
	if strings.HasPrefix(cert, base64Prefix) {
		certBytes, err = base64.StdEncoding.DecodeString(cert[len(base64Prefix):])
	} else {
		certBytes, err = ioutil.ReadFile(cert)
	}
	if err != nil {
		return
	}
	if strings.HasPrefix(key, base64Prefix) {
		keyBytes, err = base64.StdEncoding.DecodeString(key[len(base64Prefix):])
	} else {
		keyBytes, err = ioutil.ReadFile(key)
	}
	return
}
