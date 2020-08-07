package csnappy

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thinkgos/jocasta/internal/mock"
)

func TestConn(t *testing.T) {
	data := []byte("hello world")

	mconn := mock.New(new(bytes.Buffer))
	conn := New(mconn)

	// write
	n, err := conn.Write(data)
	require.NoError(t, err)
	require.Equal(t, len(data), n)

	// read
	rd := make([]byte, len(data))
	n, err = conn.Read(rd)
	require.NoError(t, err)
	require.Equal(t, len(data), n)

	// same
	require.Equal(t, rd[:n], data)
}
