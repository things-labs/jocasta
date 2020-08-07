package shadowsocks

import (
	"testing"

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

		// encrypt
		enc := make([]byte, len(src))
		cip.encrypt(enc, src)
		// decrypt
		dec := make([]byte, len(enc))
		cip.decrypt(dec, enc)

		assert.Equal(t, dec, src)
	}
}

func TestCipher_PublicEncDec(t *testing.T) {

}
