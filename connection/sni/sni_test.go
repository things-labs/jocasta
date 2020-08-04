package sni

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerNameFromBytes(t *testing.T) {
	var payload = []byte{
		0x16,       // ContentType  handshake
		0x03, 0x01, // ProtocolVersion
		0x00, 0x45, // Body Length
		// Body
		0x01,             // ClientHello(1)
		0x00, 0x00, 0x00, // message length
		0x03, 0x01, // client want ProtocolVersion
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // GMT Unix timestamp and random number
		0x05, 0x00, 0x00, 0x00, 0x00, 0x00, // session ID(length + content)
		0x00, 0x01, 0x00, // CipherSuiteList(length + content)
		0x01, 0x00, // CompressionMethod(length + content)
		0x00, 0x12, // extensionsLength
		// extensions
		0x00, 0x00, // type
		0x00, 0x00, // length
		0x00, 0x01, // number of names
		0x00,       // name type
		0x00, 0x09, // name length
		'h', 'e', 'l', 'l', 'o', '.', 'c', 'o', 'm', // name
	}

	hostname, err := ServerNameFromBytes(payload)
	require.NoError(t, err)
	assert.Equal(t, "hello.com", hostname)

	hostname, conn, err := ServerNameFromConn(&bufferedConn{nil, bytes.NewReader(payload)})
	require.NoError(t, err)
	assert.Equal(t, "hello.com", hostname)

	hostname, conn, err = ServerNameFromConn(conn)
	require.NoError(t, err)
	assert.Equal(t, "hello.com", hostname)

	hostname, _, err = ServerNameFromConn(conn)
	require.NoError(t, err)
	assert.Equal(t, "hello.com", hostname)
}
