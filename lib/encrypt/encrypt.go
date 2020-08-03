package encrypt

import (
	"crypto/cipher"
	"crypto/sha256"
	"errors"
)

// Cipher write and read cipher.Stream
type Cipher struct {
	Write cipher.Stream
	Read  cipher.Stream
}

// NewCipher new cipher
// method support:
// 		aes-128-cfb
// 		aes-192-cfb
// 		aes-256-cfb
// 		aes-128-ctr
// 		aes-192-ctr
// 		aes-256-ctr
// 		des-cfb
// 		bf-cfb
// 		cast5-cfb
// 		rc4-md5
// 		rc4-md5-6
// 		chacha20
// 		chacha20-ietf
func NewCipher(method, password string) (*Cipher, error) {
	if password == "" {
		return nil, errors.New("empty password")
	}
	info, ok := GetCipherInfo(method)
	if !ok {
		return nil, errors.New("Unsupported encryption method: " + method)
	}
	key := Evp2Key(password, info.KeyLen)

	//hash(key) -> read IV
	riv := sha256.New().Sum(key)[:info.IvLen]
	rd, err := info.NewStream(key, riv, false)
	if err != nil {
		return nil, err
	}
	//hash(read IV) -> write IV
	wiv := sha256.New().Sum(riv)[:info.IvLen]
	wr, err := info.NewStream(key, wiv, true)
	if err != nil {
		return nil, err
	}
	return &Cipher{wr, rd}, nil
}
