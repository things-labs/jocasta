package com

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubStr(t *testing.T) {
	s := "abcedf"
	assert.Equal(t, "", SubStr("", 0, 0))
	assert.Equal(t, "ab", SubStr(s, 0, 2))
	assert.Equal(t, "bc", SubStr(s, 1, 2))
	assert.Equal(t, s, SubStr(s, 0, -1))
	assert.Equal(t, s, SubStr(s, -1, -1))
}

func TestSubBytes(t *testing.T) {
	b := []byte{1, 2, 3, 4, 5, 6}
	assert.Equal(t, []byte{}, SubBytes([]byte{}, 0, 0))
	assert.Equal(t, b[:2], SubBytes(b, 0, 2))
	assert.Equal(t, b[1:3], SubBytes(b, 1, 2))
	assert.Equal(t, b, SubBytes(b, 0, -1))
	assert.Equal(t, b, SubBytes(b, -1, -1))
}
