// Package outil cfb cbc encrypt and decrypt
package outil

import (
	"crypto/aes"
	"crypto/cipher"

	"github.com/things-go/encrypt"
)

// EncryptCFB encrypt cfb
func EncryptCFB(key []byte, text []byte) ([]byte, error) {
	bc, err := encrypt.NewStreamCipher(key, aes.NewCipher, encrypt.WithStreamCodec(cipher.NewCFBEncrypter, cipher.NewCFBDecrypter))
	if err != nil {
		return nil, err
	}
	return bc.Encrypt(text)
}

// DecryptCFB decrypt cfb
func DecryptCFB(key []byte, text []byte) ([]byte, error) {
	bc, err := encrypt.NewStreamCipher(key, aes.NewCipher, encrypt.WithStreamCodec(cipher.NewCFBEncrypter, cipher.NewCFBDecrypter))
	if err != nil {
		return nil, err
	}
	return bc.Decrypt(text)
}
