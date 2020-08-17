package csnappy

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/thinkgos/jocasta/internal/mock"
)

func TestConn(t *testing.T) {
	data := []byte(
		`hello worldhello worldhello worldhello worldhello worldhello worldhello world
hello worldhello worldhello worldhello worldhello worldhello worldhello world
hello worldhello worldhello worldhello worldhello worldhello worldhello world
hello worldhello worldhello worldhello worldhello worldhello worldhello world
hello worldhello worldhello worldhello worldhello worldhello worldhello world
hello worldhello worldhello worldhello worldhello worldhello worldhello world
hello worldhello worldhello worldhello worldhello worldhello worldhello world`,
	)
	buf := new(bytes.Buffer)

	mconn := mock.New(buf)
	conn := New(mconn)

	start := time.Now()
	// write
	n, err := conn.Write(data)
	require.NoError(t, err)
	require.Equal(t, len(data), n)

	t.Log(time.Now().Sub(start).String(), buf.Len())

	// read
	rd := make([]byte, len(data))
	n, err = conn.Read(rd)
	require.NoError(t, err)
	require.Equal(t, len(data), n)

	// same
	require.Equal(t, rd[:n], data)
}
