package cs

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thinkgos/jocasta/lib/encrypt"
)

func TestSTCP(t *testing.T) {
	password := "pass_word"
	for _, method := range encrypt.CipherMethods() {
		for _, compress := range []bool{true, false} {
			func() {
				// server
				srv := &StcpServer{
					Addr:     "127.0.0.1:0",
					Method:   method,
					Password: password,
					Compress: compress,
					Status:   make(chan error, 1),
					Handler: HandlerFunc(func(inconn net.Conn) {
						buf := make([]byte, 20)
						n, err := inconn.Read(buf)
						if !assert.NoError(t, err) {
							return
						}
						assert.Equal(t, "ping", string(buf[:n]))
						_, err = inconn.Write([]byte("pong"))
						if !assert.NoError(t, err) {
							return
						}
					}),
				}
				go func() { _ = srv.ListenAndServe() }()
				require.NoError(t, <-srv.Status)
				defer srv.Close()

				d := &StcpDialer{method, password, compress}
				cli, err := d.DialTimeout(srv.LocalAddr(), 5*time.Second)
				require.NoError(t, err)
				defer cli.Close()

				_, err = cli.Write([]byte("ping"))
				require.NoError(t, err)
				b := make([]byte, 20)
				n, err := cli.Read(b)
				require.NoError(t, err)
				require.Equal(t, "pong", string(b[:n]))
			}()
		}
	}
}

func TestSSSSTCP(t *testing.T) {
	password := "pass_word"
	method := "aes-192-cfb"
	compress := false
	want := []byte("1flkdfladnfadkfna;kdnga;kdnva;ldk;adkfpiehrqeiphr23r[ingkdnv;ifefqiefn")

	// server
	srv := &StcpServer{
		Addr:     "127.0.0.1:0",
		Method:   method,
		Password: password,
		Compress: compress,
		Status:   make(chan error, 1),
		Handler: HandlerFunc(func(inconn net.Conn) {
			buf := make([]byte, 512)
			n, err := inconn.Read(buf)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, want, buf[:n])
			_, err = inconn.Write(want)
			if !assert.NoError(t, err) {
				return
			}
		}),
	}
	// server
	go func() { _ = srv.ListenAndServe() }()
	require.NoError(t, <-srv.Status)
	defer srv.Close()

	// client twice
	for i := 0; i < 2; i++ {
		func() {
			d := StcpDialer{method, password, compress}
			cli, err := d.DialTimeout(srv.LocalAddr(), 5*time.Second)
			require.NoError(t, err)
			defer cli.Close()

			// t.Log(cli.LocalAddr())
			_, err = cli.Write(want)
			require.NoError(t, err)

			got := make([]byte, 512)
			n, err := cli.Read(got)
			require.NoError(t, err)
			require.Equal(t, want, got[:n])
		}()
	}
}
