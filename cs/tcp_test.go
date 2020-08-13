package cs

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTCP(t *testing.T) {
	for _, compress := range []bool{true, false} {
		// server
		srv := &TCPServer{
			Addr:     ":",
			Compress: compress,
			Status:   make(chan error, 1),
			Handler: HandlerFunc(func(inconn net.Conn) {
				buf := make([]byte, 2048)
				_, err := inconn.Read(buf)
				if !assert.NoError(t, err) {
					return
				}
				_, err = inconn.Write([]byte("okay"))
				if !assert.NoError(t, err) {
					return
				}
			}),
		}
		go func() { _ = srv.ListenAndServe() }()
		require.NoError(t, <-srv.Status)
		defer srv.Close()

		// client
		d := &TCPDialer{compress}
		cli, err := d.DialTimeout(srv.LocalAddr(), 5*time.Second)
		require.NoError(t, err)
		defer cli.Close()

		_, err = cli.Write([]byte("test"))
		require.NoError(t, err)
		b := make([]byte, 20)
		n, err := cli.Read(b)
		require.NoError(t, err)
		require.Equal(t, "okay", string(b[:n]), "client revecive okay excepted,revecived : %s", string(b[:n]))
	}
}
