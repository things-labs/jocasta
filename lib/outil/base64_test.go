package outil

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBase64(t *testing.T) {
	b := []byte{1, 2, 3, 4, 5, 6}
	Base64Encode(b)

	orig := "helloworld"
	bs64 := Base64EncodeString(orig)

	rawByte, err := Base64Decode(bs64)
	require.NoError(t, err)
	require.Equal(t, orig, string(rawByte))

	raw, err := Base64DecodeString(bs64)
	require.NoError(t, err)
	require.Equal(t, orig, raw)
}
