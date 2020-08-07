package shadowsocks

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thinkgos/jocasta/internal/mock"
	"github.com/thinkgos/jocasta/lib/encrypt"
)

func TestConn(t *testing.T) {
	pssword := "password"
	data := []byte("hello word")

	handle := func(method string) {
		cip, err := NewCipher(method, pssword)
		require.NoError(t, err)

		buff := new(bytes.Buffer)
		conn := New(mock.New(buff), cip)
		defer conn.Close()

		n, err := conn.Write(data)
		require.NoError(t, err)
		assert.Equal(t, len(data), n)

		wantData := make([]byte, len(data))
		n, err = conn.Read(wantData)
		require.NoError(t, err)
		assert.Equal(t, n, len(data))
		assert.Equal(t, wantData, data)

		assert.Equal(t, conn.iv, conn.Iv())
		assert.Equal(t, conn.key, conn.Key())
		assert.False(t, conn.ota, conn.Ota())
	}

	for _, method := range encrypt.CipherMethods() {
		handle(method)
	}
}

func TestNewConnWithRawAddr(t *testing.T) {
	pssword := "password"
	data := []byte("hello word")
	addr := "localhost:8080"

	handle := func(method string) {
		cip, err := NewCipher(method, pssword)
		require.NoError(t, err)

		rawAddr, err := ParseAddrSpec(addr)
		require.NoError(t, err)

		conn, err := NewConnWithRawAddr(mock.New(new(bytes.Buffer)), rawAddr, cip)
		require.NoError(t, err)
		defer conn.Close()

		n, err := conn.Write(data)
		require.NoError(t, err)
		assert.Equal(t, n, len(data))

		gotAddr, err := ParseRequest(conn)
		require.NoError(t, err)
		assert.Equal(t, addr, gotAddr)

		gotData := make([]byte, len(data))
		n, err = conn.Read(gotData)
		require.NoError(t, err)
		assert.Equal(t, len(data), n)
		assert.Equal(t, data, gotData)
	}

	for _, method := range encrypt.CipherMethods() {
		handle(method)
	}
}

func TestDial(t *testing.T) {

}
