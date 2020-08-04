package issh

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var passPhrase = []byte("passPhrase")

func TestParsePrivateKeyFile2AuthMethod(t *testing.T) {
	_, err := ParsePrivateKeyFile2AuthMethod("./testdata/id_rsa")
	require.NoError(t, err)

	_, err = ParsePrivateKeyFile2AuthMethod("./testdata/id_rsa_passPhrase", passPhrase)
	require.NoError(t, err)

	_, err = ParsePrivateKeyFile2AuthMethod("./testdata/nofile")
	require.Error(t, err)
}
