package encrypt

import (
	"bytes"
	"testing"
)

func TestEncrypt(t *testing.T) {
	for _, method := range CipherMethods() {
		if !HasCipherMethod(method) {
			t.Fatalf("do not have method: %s", method)
		}
		cip, err := NewCipher(method, "pass_word")
		if err != nil {
			t.Fatalf("%+v", err)
		}
		src := []byte("hello world")
		dst := make([]byte, len(src))
		cip.Write.XORKeyStream(dst, src)
		wantDst := make([]byte, len(dst))
		cip.Read.XORKeyStream(wantDst, dst)
		if !bytes.Equal(wantDst, src) {
			t.Fatalf("want '%s' but src '%s'", string(wantDst), string(src))
		}
	}
}
