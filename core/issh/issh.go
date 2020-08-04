package issh

import (
	"fmt"
	"io/ioutil"

	"golang.org/x/crypto/ssh"
)

func ParsePrivateKey(pemBytes []byte, passPhrase ...[]byte) (ssh.Signer, error) {
	if len(passPhrase) > 0 {
		return ssh.ParsePrivateKeyWithPassphrase(pemBytes, passPhrase[0])
	}
	return ssh.ParsePrivateKey(pemBytes)
}

func ParsePrivateKeyFile(keyFile string, passPhrase ...[]byte) (ssh.Signer, error) {
	key, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("read key file %+v", err)
	}
	return ParsePrivateKey(key, passPhrase...)
}

func ParsePrivateKeyFile2AuthMethod(keyFile string, passPhrase ...[]byte) (ssh.AuthMethod, error) {
	signer, err := ParsePrivateKeyFile(keyFile, passPhrase...)
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeys(signer), nil
}
