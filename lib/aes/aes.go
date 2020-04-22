// Playbook - http://play.golang.org/p/3wFl4lacjX

package goaes

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
)

// ErrInputNotFullBlocks input not full blocks
var ErrInputNotMultipleBlocks = errors.New("decoded message length must be multiple of block size")

// ErrUnPaddingSizeTooShort unPadding out of range
var ErrUnPaddingSizeTooShort = errors.New("unPadding size too short")

// PCKSPadding PKCS#5和PKCS#7 填充
func PCKSPadding(origData []byte, blockSize int) []byte {
	padSize := blockSize - len(origData)%blockSize
	padText := bytes.Repeat([]byte{byte(padSize)}, padSize)
	return append(origData, padText...)
}

// PCKSUnPadding PKCS#5和PKCS#7 解填充
func PCKSUnPadding(origData []byte) ([]byte, error) {
	length := len(origData)
	unPadSize := int(origData[length-1])
	if unPadSize > length {
		return nil, ErrUnPaddingSizeTooShort
	}
	return origData[:(length - unPadSize)], nil
}

func EncryptCFB(key []byte, text []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	msg := PCKSPadding(text, block.BlockSize())
	cipherText := make([]byte, aes.BlockSize+len(msg))
	iv := cipherText[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}
	cipher.NewCFBEncrypter(block, iv).
		XORKeyStream(cipherText[aes.BlockSize:], msg)
	return cipherText, nil
}

func DecryptCFB(key []byte, text []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(text) == 0 || len(text)%aes.BlockSize != 0 {
		return nil, ErrInputNotMultipleBlocks
	}
	iv := text[:aes.BlockSize]
	msg := text[aes.BlockSize:]

	cipher.NewCFBDecrypter(block, iv).
		XORKeyStream(msg, msg)
	return PCKSUnPadding(msg)
}

// Encrypt 加密
func EncryptCBC(key, text []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	orig := PCKSPadding(text, block.BlockSize())
	out := make([]byte, aes.BlockSize+len(orig))
	iv := out[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}
	cipher.NewCBCEncrypter(block, iv).
		CryptBlocks(out[aes.BlockSize:], orig)
	return out, nil
}

// Decrypt 解密
func DecryptCBC(key, text []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(text) == 0 || len(text)%block.BlockSize() != 0 {
		return nil, ErrInputNotMultipleBlocks
	}
	iv := text[:aes.BlockSize]
	msg := text[aes.BlockSize:]
	cipher.NewCBCDecrypter(block, iv).
		CryptBlocks(msg, msg)
	return PCKSUnPadding(msg)
}
