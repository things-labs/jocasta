package cs

import (
	"net"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKcp(t *testing.T) {
	var err error
	runtime.GOMAXPROCS(2)
	require.True(t, HasKcpBlockCrypt("blowfish"))

	fun := func(method string, compress bool) {
		// t.Logf("kcp crypt method: %s compress: %t", method, compress)
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
		require.NoError(t, err)

		// server
		srv := &KCPServer{
			Addr:   ":",
			Config: config,
			Status: make(chan error, 1),
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
		// start server
		go func() { _ = srv.ListenAndServe() }()
		require.NoError(t, <-srv.Status)
		defer srv.Close()

		// client
		d := KCPDialer{config}
		cli, err := d.DialTimeout(srv.LocalAddr(), time.Second)
		require.NoError(t, err)
		defer cli.Close()

		_, err = cli.Write([]byte("test"))
		require.NoError(t, err)

		b := make([]byte, 20)
		n, err := cli.Read(b)
		require.NoError(t, err)
		require.Equal(t, "okay", string(b[:n]))
	}

	for _, method := range KcpBlockCryptMethods() {
		for _, compress := range []bool{true, false} {
			fun(method, compress)
		}
	}
}
