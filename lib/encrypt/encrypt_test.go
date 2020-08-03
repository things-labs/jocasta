package encrypt

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncrypt(t *testing.T) {
	for _, method := range CipherMethods() {
		require.True(t, HasCipherMethod(method))

		cip, err := NewCipher(method, "pass_word")
		require.NoError(t, err)

		src := []byte("hello world")
		dst := make([]byte, len(src))
		cip.Write.XORKeyStream(dst, src)
		wantDst := make([]byte, len(dst))
		cip.Read.XORKeyStream(wantDst, dst)

		require.True(t, bytes.Equal(wantDst, src), "want '%s' but src '%s'", string(wantDst), string(src))
	}
}
