package shadowsocks

import (
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
	"strings"

	"github.com/thinkgos/jocasta/lib/encrypt"
)

type Cipher struct {
	writer cipher.Stream
	reader cipher.Stream
	info   encrypt.CipherInfo
	key    []byte // hold key
	iv     []byte // hold iv
	ota    bool   // one-time auth
}

// NewCipher creates a cipher that can be used in Dial() etc.
// Use cipher.Clone() to create a new cipher with the same method and password
// to avoid the cost of repeated cipher initialization.
func NewCipher(method, password string) (c *Cipher, err error) {
	if password == "" {
		return nil, errors.New("empty password")
	}
	ota := false
	if strings.HasSuffix(strings.ToLower(method), "-auth") {
		method = method[:len(method)-5] // trim "-auth"
		ota = true
	}
	cipInfo, ok := encrypt.GetCipherInfo(method)
	if !ok {
		return nil, errors.New("Unsupported encryption method: " + method)
	}

	return &Cipher{
		info: cipInfo,
		key:  encrypt.Evp2Key(password, cipInfo.KeyLen),
		ota:  ota,
	}, nil
}

// Initializes the block cipher with CFB mode, returns IV.
func (c *Cipher) initEncrypt() ([]byte, error) {
	var err error

	if c.iv == nil {
		iv := make([]byte, c.info.IvLen)
		if _, err = io.ReadFull(rand.Reader, iv); err != nil {
			return nil, err
		}
		c.iv = iv
	}
	c.writer, err = c.info.NewStream(c.key, c.iv, true)
	return c.iv, err
}

func (c *Cipher) encrypt(dst, src []byte) {
	c.writer.XORKeyStream(dst, src)
}

func (c *Cipher) initDecrypt(iv []byte) (err error) {
	c.reader, err = c.info.NewStream(c.key, iv, false)
	if err != nil {
		return
	}
	if len(c.iv) == 0 {
		c.iv = iv
	}
	return
}

func (c *Cipher) decrypt(dst, src []byte) {
	c.reader.XORKeyStream(dst, src)
}

func (c *Cipher) Encrypt(src []byte) (cipherData []byte) {
	cip := c.Clone()
	iv, err := cip.initEncrypt()
	if err != nil {
		return
	}
	packetLen := len(src) + len(iv)
	cipherData = make([]byte, packetLen)
	copy(cipherData, iv)
	cip.encrypt(cipherData[len(iv):], src)
	return
}

func (c *Cipher) Decrypt(src []byte) (data []byte) {
	var err error

	cip := c.Clone()
	if len(src) < c.info.IvLen {
		return
	}
	iv := make([]byte, c.info.IvLen)
	copy(iv, src[:c.info.IvLen])
	cip.reader, err = c.info.NewStream(c.key, iv, false)
	if err != nil {
		return
	}
	data = make([]byte, len(src)-len(iv))
	cip.decrypt(data, src[c.info.IvLen:])
	return
}

// Clone creates a new cipher at it's initial state.
func (c *Cipher) Clone() *Cipher {
	// This optimization maybe not necessary. But without this function, we
	// need to maintain a table cache for newTableCipher and use lock to
	// protect concurrent access to that cache.

	// AES and DES ciphers does not return specific types, so it's difficult
	// to create copy. But their initizliation time is less than 4000ns on my
	// 2.26 GHz Intel Core 2 Duo processor. So no need to worry.

	// Currently, blow-fish and cast5 initialization cost is an order of
	// maganitude slower than other ciphers. (I'm not sure whether this is
	// because the current implementation is not highly optimized, or this is
	// the nature of the algorithm.)

	nc := *c
	nc.writer = nil
	nc.reader = nil
	return &nc
}
