package cs

import (
	"net"
	"testing"
)

func TestKcp(t *testing.T) {
	var err error
	HasKcpBlockCrypt("blowfish")
	for _, method := range KcpBlockCryptMethods() {
		for _, compress := range []bool{true, false} {
			t.Logf("kcp crypt method: %s compress: %t", method, compress)
			config := KcpConfig{
				MTU:          1400,
				SndWnd:       32,
				RcvWnd:       32,
				DataShard:    10,
				ParityShard:  3,
				DSCP:         0,
				NoComp:       compress,
				AckNodelay:   true,
				NoDelay:      1,
				Interval:     10,
				Resend:       2,
				NoCongestion: 1,
				SockBuf:      4194304,
				KeepAlive:    10,
			}
			config.Block, err = NewKcpBlockCryptWithPbkdf2(method, "key", "thinkgos-goproxy")
			if err != nil {
				t.Fatal(err)
			}
			s, err := NewKcp(":", config, func(inconn net.Conn) {
				buf := make([]byte, 2048)
				_, err := inconn.Read(buf)
				if err != nil {
					t.Error(err)
					return
				}
				_, err = inconn.Write([]byte("okay"))
				if err != nil {
					t.Error(err)
					return
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			go func() {
				_ = s.ListenAndServe()
			}()

			if err = <-s.Status(); err != nil {
				t.Fatal(err)
			}
			defer s.Close()

			cli, err := DialKcp(s.Addr(), config)
			if err != nil {
				t.Fatal(err)
			}
			defer cli.Close()

			_, err = cli.Write([]byte("test"))
			if err != nil {
				t.Fatal(err)
			}
			b := make([]byte, 20)
			n, err := cli.Read(b)
			if err != nil {
				t.Fatal(err)
			}
			if string(b[:n]) != "okay" {
				t.Fatalf("client revecive okay excepted,revecived : %s", string(b[:n]))
			}
		}
	}
}
