package goaes

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCFB(t *testing.T) {
	key := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	want := []byte("helloworld")
	text, err := EncryptCFB(key, want)
	require.NoError(t, err)
	got, err := DecryptCFB(key, text)
	require.NoError(t, err)
	assert.Equal(t, want, got)

	key = []byte{1, 2, 3}
	text, err = EncryptCFB(key, want)
	require.Error(t, err)

	_, err = DecryptCFB(key, text)
	require.Error(t, err)
}

func TestCBC(t *testing.T) {
	key := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	want := []byte("helloworld")
	text, err := EncryptCBC(key, want)
	require.NoError(t, err)
	got, err := DecryptCBC(key, text)
	require.NoError(t, err)
	assert.Equal(t, want, got)

	key = []byte{1, 2, 3}
	_, err = EncryptCBC(key, want)
	require.Error(t, err)

	_, err = DecryptCBC(key, text)
	require.Error(t, err)
}
