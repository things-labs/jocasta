package ccs

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/thinkgos/jocasta/cs"
	"github.com/thinkgos/jocasta/lib/encrypt"
)

func TestStream_Stcp(t *testing.T) {
	password := "pass_word"
	want := []byte("1flkdfladnfadkfna;kdnga;kdnva;ldk;adkfpiehrqeiphr23r[ingkdnv;ifefqiefn")
	for _, method := range encrypt.CipherMethods() {
		for _, compress := range []bool{true, false} {
			testFunc := func() {
				config := Config{STCPMethod: method, STCPPassword: password, Compress: compress}
				// t.Logf("stcp method: %s compress: %t", method, compress)

				// server
				srv := &Server{
					Protocol: "stcp",
					Addr:     ":",
					Config:   config,
					Handler: cs.HandlerFunc(func(inconn net.Conn) {
						buf := make([]byte, 2048)
						_, err := inconn.Read(buf)
						if err != nil {
							t.Error(err)
							return
						}
						_, err = inconn.Write(want)
						if err != nil {
							t.Error(err)
							return
						}
					}),
				}
				s, errChan := srv.RunListenAndServe()
				require.NoError(t, <-errChan)
				defer s.Close()

				// client
				d := Dialer{config}
				cli, err := d.DialTimeout("stcp", s.LocalAddr(), 5*time.Second)
				require.NoError(t, err)
				defer cli.Close()

				_, err = cli.Write(want)
				require.NoError(t, err)
				b := make([]byte, 2048)
				n, err := cli.Read(b)
				require.NoError(t, err)
				require.Equal(t, want, b[:n])
			}
			testFunc()
		}
	}
}
