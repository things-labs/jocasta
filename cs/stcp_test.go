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
			// server
			srv := &StcpServer{
				Addr:     ":",
				Method:   method,
				Password: password,
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

			d := &StcpDialer{method, password, compress}
			cli, err := d.DialTimeout(srv.LocalAddr(), 5*time.Second)
			require.NoError(t, err)
			defer cli.Close()

			_, err = cli.Write([]byte("test"))
			require.NoError(t, err)
			b := make([]byte, 20)
			n, err := cli.Read(b)
			require.NoError(t, err)
			require.Equal(t, "okay", string(b[:n]))
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
		Addr:     ":",
		Method:   method,
		Password: password,
		Compress: compress,
		Status:   make(chan error, 1),
		Handler: HandlerFunc(func(inconn net.Conn) {
			buf := make([]byte, 2048)
			n, err := inconn.Read(buf)
			if !assert.NoError(t, err) {
				return
			}
			_, err = inconn.Write(buf[:n])
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

			got := make([]byte, 2048)
			n, err := cli.Read(got)
			require.NoError(t, err)
			require.Equal(t, want, got[:n])
		}()
	}
}
