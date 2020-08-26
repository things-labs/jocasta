package cs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thinkgos/go-core-package/extnet"
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
				ln, err := KCPListen("", "127.0.0.1:0", config, extnet.AdornSnappy(compress))
				require.NoError(t, err)
				defer ln.Close()
				go func() {
					for {
						conn, err := ln.Accept()
						if err != nil {
							return
						}
						go func() {
							buf := make([]byte, 20)
							n, err := conn.Read(buf)
							if !assert.NoError(t, err) {
								return
							}
							assert.Equal(t, "ping", string(buf[:n]))
							_, err = conn.Write([]byte("pong"))
							if !assert.NoError(t, err) {
								return
							}
						}()

					}
				}()

				// client
				d := &KCPClient{
					Config:      config,
					AfterChains: extnet.AdornConnsChain{extnet.AdornSnappy(compress)},
				}
				cli, err := d.Dial("", ln.Addr().String())
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
