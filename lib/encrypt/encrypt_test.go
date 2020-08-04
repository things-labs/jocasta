package encrypt

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncrypt(t *testing.T) {
	_, err := NewCipher("invalid_method", "")
	require.Error(t, err)
	_, err = NewCipher("invalid_method", "pass_word")
	require.Error(t, err)

	for _, method := range CipherMethods() {
		require.True(t, HasCipherMethod(method))

		cip, err := NewCipher(method, "pass_word")
		require.NoError(t, err)

		src := []byte("hello world")
		dst := make([]byte, len(src))
		cip.Write.XORKeyStream(dst, src)
		wantDst := make([]byte, len(dst))
		cip.Read.XORKeyStream(wantDst, dst)

		require.Equal(t, string(wantDst), string(src))
	}
}
