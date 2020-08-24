package shadowsocks

import (
	"bytes"
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thinkgos/go-core-package/lib/encrypt"
	"github.com/thinkgos/jocasta/internal/mock"
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

		for i := 0; i < 3; i++ {
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
		}
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

		gotAddr, err := ParseRequest(conn)
		require.NoError(t, err)
		assert.Equal(t, addr, gotAddr)

		n, err := conn.Write(data)
		require.NoError(t, err)
		assert.Equal(t, n, len(data))

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
	pssword := "password"
	wdata := []byte("hello world")
	rdata := []byte("i was a word")
	addr := "localhost:8080"

	methods := encrypt.CipherMethods()
	randMethod := methods[rand.New(rand.NewSource(time.Now().UnixNano())).Intn(len(methods))]

	cip, err := NewCipher(randMethod, pssword)
	require.NoError(t, err)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go func() {
			conn = New(conn, cip.Clone())

			gotAddr, err := ParseRequest(conn)
			require.NoError(t, err)
			assert.Equal(t, addr, gotAddr)

			wd := make([]byte, len(wdata))
			n, err := conn.Read(wd)
			require.NoError(t, err)
			assert.Equal(t, len(wdata), n)

			n, err = conn.Write(rdata)
			require.NoError(t, err)
			assert.Equal(t, len(rdata), n)
		}()
	}()
	time.Sleep(time.Millisecond * 200)
	conn, err := Dial(addr, ln.Addr().String(), cip.Clone())
	require.NoError(t, err)

	n, err := conn.Write(wdata)
	require.NoError(t, err)
	assert.Equal(t, len(wdata), n)

	rd := make([]byte, len(rdata))
	n, err = conn.Read(rd)
	require.NoError(t, err)
	assert.Equal(t, len(rdata), n)
}
