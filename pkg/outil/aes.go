// Package outil cfb cbc encrypt and decrypt
package outil

import (
	"crypto/aes"
	"crypto/cipher"

	"github.com/thinkgos/x/lib/encrypt"
)

func NewAesCFB(key []byte) (encrypt.BlockCrypt, error) {
	bsc := encrypt.BlockStreamCipher{
		NewEncrypt: cipher.NewCFBEncrypter,
		NewDecrypt: cipher.NewCFBDecrypter,
	}
	return bsc.New(key, aes.NewCipher)
}

// EncryptCFB encrypt cfb
func EncryptCFB(key []byte, text []byte) ([]byte, error) {
	bc, err := NewAesCFB(key)
	if err != nil {
		return nil, err
	}
	return bc.Encrypt(text)
}

// DecryptCFB decrypt cfb
func DecryptCFB(key []byte, text []byte) ([]byte, error) {
	bc, err := NewAesCFB(key)
	if err != nil {
		return nil, err
	}
	return bc.Decrypt(text)
}
