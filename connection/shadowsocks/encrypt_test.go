package shadowsocks

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thinkgos/jocasta/lib/encrypt"
)

func TestCipher(t *testing.T) {
	_, err := NewCipher("invalid_method", "")
	require.Error(t, err)
	_, err = NewCipher("invalid_method", "pass_word")
	require.Error(t, err)

	password := "pass_word"
	src := []byte("this is just a test data")

	for _, method := range encrypt.CipherMethods() {
		cip, err := NewCipher(method, password)
		require.NoError(t, err)

		iv, err := cip.initEncrypt()
		require.NoError(t, err)
		err = cip.initDecrypt(iv)
		require.NoError(t, err)

		for i := 0; i < 3; i++ {
			// encrypt
			enc := make([]byte, len(src))
			cip.encrypt(enc, src)
			// decrypt
			dec := make([]byte, len(enc))
			cip.decrypt(dec, enc)

			assert.Equal(t, dec, src)
		}
	}
}

func TestCipher_Encrypt_Decrypt(t *testing.T) {
	password := "pass_word"
	src := []byte("this is just a test data")

	for _, method := range encrypt.CipherMethods() {
		cip, err := NewCipher(method, password)
		require.NoError(t, err)

		// encrypt
		enc, err := cip.Encrypt(src)
		require.NoError(t, err)
		// decrypt
		dec, err := cip.Decrypt(enc)
		require.NoError(t, err)

		assert.Equal(t, dec, src)
	}
}

func TestCipher_Invalid_Input(t *testing.T) {
	password := "pass_word"

	methods := encrypt.CipherMethods()
	randMethod := methods[rand.New(rand.NewSource(time.Now().UnixNano())).Intn(len(methods))]

	cip, err := NewCipher(randMethod, password)
	require.NoError(t, err)

	// decrypt
	dec, err := cip.Decrypt([]byte{1, 2})
	require.Error(t, err)
	require.Nil(t, dec)
}
