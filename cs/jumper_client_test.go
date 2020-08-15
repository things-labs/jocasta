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

func TestValidJumperProxyURL(t *testing.T) {
	tests := []struct {
		name     string
		proxyURL string
		want     bool
	}{
		{"https://host:8080", "https://host:8080", true},
		{"https://username:password@host:8080", "https://username:password@host:8080", true},
		{"socks5://host:8080", "socks5://host:8080", true},
		{"socks5://username:password@host:8080", "socks5://username:password@host:8080", true},
		{"invalid", "invalid", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidJumperProxyURL(tt.proxyURL); got != tt.want {
				t.Errorf("ValidJumperProxyURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJumper(t *testing.T) {
	_, err := NewJumper("invalid")
	require.Error(t, err)

	jump, err := NewJumper("socks5://username:password@host:8080")
	require.NoError(t, err)
	require.NotNil(t, jump)
}

func TestJumper_socks5_tcp(t *testing.T) {
	for _, compress := range []bool{true, false} {
		func() {
			// start server
			srv := &TCPServer{
				Addr:     "127.0.0.1:0",
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
			jump, err := NewJumper(proxyURL)
			require.NoError(t, err)
			// t.Logf("socks5 proxy url: %v", proxyURL)

			// client
			cli := &JumperTCP{jump, compress}
			conn, err := cli.DialTimeout(srv.LocalAddr(), time.Second)
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

func TestJumper_socks5_tls(t *testing.T) {
	for _, single := range []bool{true, false} {
		func() {
			// server
			srv := &TCPTlsServer{
				Addr:   "127.0.0.1:0",
				CaCert: nil,
				Cert:   []byte(crt),
				Key:    []byte(key),
				Single: single,
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
			jump, err := NewJumper(proxyURL)
			require.NoError(t, err)
			// t.Logf("socks5 proxy url: %v", proxyURL)

			jumptcp := &JumperTCPTls{
				Jumper: jump,
				CaCert: []byte(crt),
				Cert:   []byte(crt),
				Key:    []byte(key),
				Single: single,
			}
			conn, err := jumptcp.DialTimeout(srv.LocalAddr(), time.Second)
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

func TestJumper_socks5_stcp(t *testing.T) {
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
				jump, err := NewJumper(proxyURL)
				require.NoError(t, err)
				// t.Logf("socks5 proxy url: %v", proxyURL)

				jumptcp := &JumperStcp{
					Jumper:   jump,
					Method:   method,
					Password: password,
					Compress: compress,
				}
				conn, err := jumptcp.DialTimeout(srv.LocalAddr(), time.Second)
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
