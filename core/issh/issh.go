package issh

import (
	"fmt"
	"io/ioutil"

	"golang.org/x/crypto/ssh"
)

// ParsePrivateKey parse private key
func ParsePrivateKey(pemBytes []byte, passPhrase ...[]byte) (ssh.Signer, error) {
	if len(passPhrase) > 0 {
		return ssh.ParsePrivateKeyWithPassphrase(pemBytes, passPhrase[0])
	}
	return ssh.ParsePrivateKey(pemBytes)
}

// ParsePrivateKeyFile parse private key file
func ParsePrivateKeyFile(keyFilename string, passPhrase ...[]byte) (ssh.Signer, error) {
	key, err := ioutil.ReadFile(keyFilename)
	if err != nil {
		return nil, fmt.Errorf("read key file %+v", err)
	}
	return ParsePrivateKey(key, passPhrase...)
}

// ParsePrivateKeyFile2AuthMethod parse private key file to ssh.AuthMethod
func ParsePrivateKeyFile2AuthMethod(keyFilename string, passPhrase ...[]byte) (ssh.AuthMethod, error) {
	signer, err := ParsePrivateKeyFile(keyFilename, passPhrase...)
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeys(signer), nil
}
