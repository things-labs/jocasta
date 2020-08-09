package ccs

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/thinkgos/jocasta/lib/encrypt"
)

func TestAnyStcp(t *testing.T) {
	password := "pass_word"
	want := []byte("1flkdfladnfadkfna;kdnga;kdnva;ldk;adkfpiehrqeiphr23r[ingkdnv;ifefqiefn")
	for _, method := range encrypt.CipherMethods() {
		for _, compress := range []bool{true, false} {
			testFunc := func() {
				cfg := Config{STCPMethod: method, STCPPassword: password, Compress: compress}
				t.Logf("stcp method: %s compress: %t", method, compress)
				s, err := ListenAndServeAny("stcp", ":", func(inconn net.Conn) {
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
				}, cfg)
				if err != nil {
					t.Fatal(err)
				}
				defer s.Close()

				cli, err := DialTimeout("stcp", s.Addr(), 5*time.Second, cfg)
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
