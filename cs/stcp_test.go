package cs

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thinkgos/go-socks5"

	"github.com/thinkgos/jocasta/lib/encrypt"
)

func TestSTCP_Forward_Direct(t *testing.T) {
	password := "pass_word"
	for _, method := range encrypt.CipherMethods() {
		for _, compress := range []bool{true, false} {
			func() {
				// server
				srv := &StcpServer{
					Addr:     "127.0.0.1:0",
					Method:   method,
					Password: password,
					Status:   make(chan error, 1),
					AfterChains: AdornConnsChain{
						AdornCsnappy(compress),
					},
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

				d := &StcpDialer{
					Method:   method,
					Password: password,
					Timeout:  time.Second,
					AfterChains: AdornConnsChain{
						AdornCsnappy(compress),
					},
				}
				cli, err := d.Dial("tcp", srv.LocalAddr())
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

func TestStcp_Forward_socks5(t *testing.T) {
	password := "pass_word"
	for _, method := range encrypt.CipherMethods() {
		for _, compress := range []bool{true, false} {
			func() {
				// server
				srv := &StcpServer{
					Addr:     "127.0.0.1:0",
					Method:   method,
					Password: password,
					Status:   make(chan error, 1),
					AfterChains: AdornConnsChain{
						AdornCsnappy(compress),
					},
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

				// start socks5 proxy server
				cator := &socks5.UserPassAuthenticator{Credentials: socks5.StaticCredentials{"user": "password"}}
				proxySrv := socks5.NewServer(
					socks5.WithAuthMethods(
						[]socks5.Authenticator{
							new(socks5.NoAuthAuthenticator),
							cator,
						}),
				)
				proxyLn, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)
				defer proxyLn.Close() // nolint: errcheck

				go func() {
					proxySrv.Serve(proxyLn) // nolint: errcheck
				}()

				time.Sleep(time.Millisecond * 100)
				// t.Logf("proxy server address: %v", proxyLn.Addr().String())

				// start jumper to socks5
				proxyURL := "socks5://" + "user:password@" + proxyLn.Addr().String()
				pURL, err := ParseProxyURL(proxyURL)
				require.NoError(t, err)
				// t.Logf("socks5 proxy url: %v", proxyURL)

				d := &StcpDialer{
					Method:   method,
					Password: password,
					Timeout:  time.Second,
					Forward:  Socks5{pURL.Host, ProxyAuth(pURL), time.Second, nil},
					AfterChains: AdornConnsChain{
						AdornCsnappy(compress),
					},
				}
				conn, err := d.Dial("tcp", srv.LocalAddr())
				require.NoError(t, err)
				defer conn.Close() // nolint: errcheck
				_, err = conn.Write([]byte("ping"))
				require.NoError(t, err)
				b := make([]byte, 20)
				n, err := conn.Read(b)
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
		Status:   make(chan error, 1),
		AfterChains: AdornConnsChain{
			AdornCsnappy(compress),
		},
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
			d := StcpDialer{
				Method:   method,
				Password: password,
				Timeout:  time.Second,
				AfterChains: AdornConnsChain{
					AdornCsnappy(compress),
				},
			}
			cli, err := d.Dial("tcp", srv.LocalAddr())
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
