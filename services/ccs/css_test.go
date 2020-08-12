package ccs

import (
	"bytes"
	"net"
	"testing"
	"time"

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

				srv := Server{config}
				s, err := srv.ListenAndServeAny("stcp", ":", func(inconn net.Conn) {
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
				})
				if err != nil {
					t.Fatal(err)
				}
				defer s.Close()

				d := Dialer{config}
				cli, err := d.DialTimeout("stcp", s.Addr(), 5*time.Second)
				if err != nil {
					t.Fatal(err)
				}
				defer cli.Close()

				_, err = cli.Write(want)
				if err != nil {
					t.Fatal(err)
				}
				b := make([]byte, 2048)
				n, err := cli.Read(b)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(b[:n], want) {
					t.Fatalf("client revecive okay excepted,revecived : %s", string(b[:n]))
				}
			}
			testFunc()
		}
	}
}
