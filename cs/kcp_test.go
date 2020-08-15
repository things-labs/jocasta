package cs

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKcp(t *testing.T) {
	key := []byte("qwertyuiopasdfghjklzxcvbnm123456")

	require.True(t, HasKcpBlockCrypt("blowfish"))
	_, err := NewKcpBlockCrypt("invalidMethod", key)
	require.Error(t, err)
	_, err = NewKcpBlockCrypt("blowfish", key[:8])
	require.Error(t, err)

	for _, method := range KcpBlockCryptMethods() {
		for _, compress := range []bool{true, false} {
			func() {
				var err error

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
				config.Block, err = NewKcpBlockCryptWithPbkdf2(method, "key", "thinkgos-jocasta")
				require.NoError(t, err)

				// server
				srv := &KCPServer{
					Addr:   "127.0.0.1:0",
					Config: config,
					Status: make(chan error, 1),
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
				// start server
				go func() { _ = srv.ListenAndServe() }()
				require.NoError(t, <-srv.Status)
				defer srv.Close()

				// client
				d := &KCPDialer{config}
				cli, err := d.DialTimeout(srv.LocalAddr(), time.Second)
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
