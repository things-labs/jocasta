package enet

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsHTTP(t *testing.T) {
	assert.True(t, IsHTTP([]byte("get")))
	assert.True(t, IsHTTP([]byte("GET")))

	assert.False(t, IsHTTP([]byte("Get")))
	assert.False(t, IsHTTP([]byte("false")))
}

func BenchmarkIsHTTP(b *testing.B) {
	v := []byte("abcedefad")
	for i := 0; i < b.N; i++ {
		IsHTTP(v)
	}
}
