package cs

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thinkgos/go-socks5"

	"github.com/thinkgos/jocasta/lib/cert"
)

func TestTCP_Forward_Direct(t *testing.T) {
	for _, compress := range []bool{true, false} {
		func() {
			// server
			srv := &TCPServer{
				Addr:        "127.0.0.1:0",
				Status:      make(chan error, 1),
				AfterChains: AdornConnsChain{AdornCsnappy(compress)},
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

			// client
			d := &TCPDialer{
				Timeout: time.Second,
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

func TestTCP_Forward_socks5(t *testing.T) {
	for _, compress := range []bool{true, false} {
		func() {
			// start server
			srv := &TCPServer{
				Addr:        "127.0.0.1:0",
				Status:      make(chan error, 1),
				AfterChains: AdornConnsChain{AdornCsnappy(compress)},
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

			// client
			cli := &TCPDialer{
				Timeout: time.Second,
				Forward: Socks5{pURL.Host, ProxyAuth(pURL), time.Second, nil},
				AfterChains: AdornConnsChain{
					AdornCsnappy(compress),
				},
			}
			conn, err := cli.Dial("tcp", srv.LocalAddr())
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

var base64Key = `base64://LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFcEFJQkFBS0NBUUVBd1pxdVpxUWJjNlRhWnlhYTBVVjVYUnFEZTdGWTZCTmhrN0Z4Rk12d1B5UTBqU2o5ClQzZGZtQk5rTEViZEt3T0VrM2ZyTUc1bzB6bDVaYlhqK0IrMjRLUTV2MFBCVmpMSHlKcHpkOGJwa3EzVy9lQWgKV0lLaWhZN1hzeHIyc0VTN2o5V1R0KzZLSVhiTUV4MklJS0RhT05jVkNYSDUxaElocDFxcVp3QnRWSXlrZFVuMwpMd3pEaWJHcXA0UktBQkR5OUN4WTN4OGFsUFBZYlQwYUJmNGY2MFU3WVBsSTEvazdRUGtYZytETGxvZyt1dHNuCmVvdkNlMzNWRlQ1SU9zektWeFBVRkdxenhxYnZOTWdVRmM1ZXJvbjJTQ0hLVWphcnl2ZTBqZFVZMWpuaU51cGIKQjQ5MDJhdytoUkdlcm82RnNmWmtCc05pVjJTVWdlRy8rNW9SMVFJREFRQUJBb0lCQVFDeDJaTFVuM1RJYTJ4bQp6Y1B5OHN0bWgvQzVORlhqKzhucmoxbStMUXBxTnF3LzhLT2kySnBzYllQY1dNemJzc09iWk5JZEQ1QWtXZXYzClQzdzRkNG5jRzRFZy92RWdhazIxTG8xY1B0SmErRzlEa1IyUTNaREcrRTJXTHZMblFueTZ5UXlHTHcrZFpqQmEKYndxYVRxbXBCWXhCdlA0eGRUNk5LbkRYWmtFSkpRQkc4bU81YlJNNm9aWnBwOUxpZHRvZGxVNGRhb3hJenZiZgpsRVBVWmt1S09zTGtlT2lNMmljWHVVOVNTWkVFeE9zL2lnNXRnTEVIZEhtVmhucHZBUXI3NXVrTy9JbVpPeXcxCk5lN0FiQzZYa2lSSnBvaDJPZTYzbzA0Zk9SQkkvTzVIZUpOdkpYUHR1eG9hcjdXSVZiWk1xcWhrdXJqcXR6NWwKY2pLcC96c0JBb0dCQU92eXRXVUNBcHhTUm9HaWZLamVDcGpwTUFmYlhwRkRJRjdpUjd6VU54M1p5N3ZmVFRtNApGelNiYVQ4N1lacEo4R3Fzc1lpbW1RS1JBSTMzZlFVTTBiRHhpS1Naa2FrU2VyV0VMVGdBVGhyOEJOMWU2aGZjCk9OaFZrREtBVmxCWWMza3NYTjFGcm1mS1NpL1luQkF3RXRXS0tlWU5ONzVzdkt3TjFSc1M0VHNSQW9HQkFOSU8KdlNTSnFwaENLaW8vWEZCcWoyT3p1NVVGZS9NVlZDNlhUMzhTdm9WYmRBaVJKcWVJZ29Fcm43TitxbzBScGpOagpUYU1EazZSNjU2NC8wc2dkUjhpWnhRLzlDeTV1aldRRjhqZWRrNFhMYzZXWGk5Qlhtb0hsQWZtTHloRDl3dWpjClpVVWVmUXNCWitpNEoxQ21Wb3Z1L0RiaFpZenVlM0V6a1AxTm5FS0ZBb0dBU0k4WkRYanl5SlBjcnQwRExRTXIKaXg2YThLK2JnMXg3UmZLY1VRdUo3NW9jdHlmU25kOW84M3FmZ1J5SHhXVGJsRktMUGhUTmxTWjJYekl1dGpEZApBMmNqdUVxcHFxN09JYWdHSitTZ0lGaEVQcmVEa2RiZGZGbkR3R1FMSnlZc1RLVkI0YUllSWpqcFc1Rm5YT3NMCnY3Ti9jd201ak12dnNaR0hhWTRDeWFFQ2dZQWxVd01ldyt0eEpJaVRlekN2QlZBM09nK0J1amk5QjdRdWx5cEQKL1JPblpJbW9vQW9MU01GUHJHMnpHalc1M1VIMzdaUTAvQVMyL0RQQWpZeXBqREplSFp5YmE2NFo4UURrbmYzZApEZjNSajBZY1RXSkZnZHR0YTBDL2s2d3krclF3WmtFRVdCZUY1aGtOaS9OSWJGWUNoVk9CZU9sdmNreXkzNlBLCnJvaXVkUUtCZ1FEbWE4eGExT2hjYmhYUUdMK1VWWTMwQktpaGFiakFOMk9BTjRVa3grOWtLZ3pvR1FQUFNURmEKaW4xMEJ3S3BmOWI5NXlxcVZpRjZWS2IrTlNPQmUyS2R5eHg1UFduR0trR05TZEdvYW4rdXJoN200TkpTYmtBaQpyRlZ4OFllRkV6UU0zNklzR1lVd0tXVm9COUVoTjVpZytxMEFjNE1uZG5oTkRzMWt0cThocmc9PQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQ==`
var base64Crt = `base64://LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURUekNDQWplZ0F3SUJBZ0lCQVRBTkJna3Foa2lHOXcwQkFRc0ZBREJKTVFzd0NRWURWUVFHRXdKYVRURVMKTUJBR0ExVUVDaE1KZGpKMVlqRnFMbU5uTVJJd0VBWURWUVFMRXdsMk1uVmlNV291WTJjeEVqQVFCZ05WQkFNVApDWFl5ZFdJeGFpNWpaekFlRncweU1EQXpNekV4TXpRMU1EZGFGdzB6TURBek1qa3hORFExTURkYU1Fa3hDekFKCkJnTlZCQVlUQWxwTk1SSXdFQVlEVlFRS0V3bDJNblZpTVdvdVkyY3hFakFRQmdOVkJBc1RDWFl5ZFdJeGFpNWoKWnpFU01CQUdBMVVFQXhNSmRqSjFZakZxTG1Obk1JSUJJakFOQmdrcWhraUc5dzBCQVFFRkFBT0NBUThBTUlJQgpDZ0tDQVFFQXdacXVacVFiYzZUYVp5YWEwVVY1WFJxRGU3Rlk2Qk5oazdGeEZNdndQeVEwalNqOVQzZGZtQk5rCkxFYmRLd09FazNmck1HNW8wemw1WmJYaitCKzI0S1E1djBQQlZqTEh5SnB6ZDhicGtxM1cvZUFoV0lLaWhZN1gKc3hyMnNFUzdqOVdUdCs2S0lYYk1FeDJJSUtEYU9OY1ZDWEg1MWhJaHAxcXFad0J0Vkl5a2RVbjNMd3pEaWJHcQpwNFJLQUJEeTlDeFkzeDhhbFBQWWJUMGFCZjRmNjBVN1lQbEkxL2s3UVBrWGcrRExsb2crdXRzbmVvdkNlMzNWCkZUNUlPc3pLVnhQVUZHcXp4cWJ2Tk1nVUZjNWVyb24yU0NIS1VqYXJ5dmUwamRVWTFqbmlOdXBiQjQ5MDJhdysKaFJHZXJvNkZzZlprQnNOaVYyU1VnZUcvKzVvUjFRSURBUUFCbzBJd1FEQU9CZ05WSFE4QkFmOEVCQU1DQXFRdwpIUVlEVlIwbEJCWXdGQVlJS3dZQkJRVUhBd0VHQ0NzR0FRVUZCd01DTUE4R0ExVWRFd0VCL3dRRk1BTUJBZjh3CkRRWUpLb1pJaHZjTkFRRUxCUUFEZ2dFQkFIbDV6QmhkZk4yb3hVc3hqbG1mYU9MZlJJRGE2d0VBeWVXcWFzcjAKQlcxWlArZWhocHZRTXhHOXhYalRsYkJIbmozNFc3ZlRrenZyajl4YzRtVTYxdE11Z2lmYklXbnpYSVBXclZldQp4VFFpdnc2aVZtWWNrVUJob0k2V2lIdVl2K05PaTJoNzJ1V0xtdjBKRGZHMU5GZGRGQmNjT0l6UWQ0aVRPK3ppCnVmcmczSXJiSngrN0VuQTg3dlhHZFpWSXRnejkySG9RRjNIUGZlWHp6U0ZNak5teEVKS05QMUlVN1ZtbFBTVXYKMEY5c0Ywd3VrTWlPR1VRMHRYZVl2M0FySHFFZnd0RjVIOU9INVJDdXNwRkZNeDZxUHlBYzFDY2pzNzNHTEo4SQpUTDQ0dEJUVTNFMEJsK2Z5QlNSa0FYYlZWVGNZc3hUZUhzU3VZbTNwQVJUcEtzdz0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQ==`

func TestTcpTls_Forward_Direct(t *testing.T) {
	crt, key, err := cert.LoadPair(base64Crt, base64Key)
	require.NoError(t, err)

	for _, compress := range []bool{true, false} {
		for _, single := range []bool{true, false} {
			func() {
				srvCfg := TLSConfig{
					CaCert: nil,
					Cert:   crt,
					Key:    key,
					Single: single,
				}

				serverConfig, err := srvCfg.ServerConfig()
				require.NoError(t, err)

				// server
				srv := &TCPServer{
					Addr:        "127.0.0.1:0",
					Config:      serverConfig,
					Status:      make(chan error, 1),
					AfterChains: AdornConnsChain{AdornCsnappy(compress)},
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

				// client
				cliConfig := TLSConfig{
					CaCert: crt,
					Cert:   crt,
					Key:    key,
					Single: single,
				}
				if !single {
					cliConfig.CaCert = nil
				}
				clientConfig, err := cliConfig.ClientConfig()
				require.NoError(t, err)

				d := &TCPDialer{
					Config:      clientConfig,
					Timeout:     time.Second,
					AfterChains: AdornConnsChain{AdornCsnappy(compress)},
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

func TestJumper_socks5_tls(t *testing.T) {
	crt, key, err := cert.LoadPair(base64Crt, base64Key)
	require.NoError(t, err)

	for _, compress := range []bool{true, false} {
		for _, single := range []bool{true, false} {
			func() {
				srvCfg := TLSConfig{
					CaCert: nil,
					Cert:   crt,
					Key:    key,
					Single: single,
				}

				serverConfig, err := srvCfg.ServerConfig()
				require.NoError(t, err)

				// server
				srv := &TCPServer{
					Addr:        "127.0.0.1:0",
					Config:      serverConfig,
					Status:      make(chan error, 1),
					AfterChains: AdornConnsChain{AdornCsnappy(compress)},
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

				// client
				cliConfig := TLSConfig{
					CaCert: crt,
					Cert:   crt,
					Key:    key,
					Single: single,
				}
				if !single {
					cliConfig.CaCert = nil
				}
				clientConfig, err := cliConfig.ClientConfig()
				require.NoError(t, err)
				d := &TCPDialer{
					Config:      clientConfig,
					Timeout:     time.Second,
					Forward:     Socks5{pURL.Host, ProxyAuth(pURL), time.Second, nil},
					AfterChains: AdornConnsChain{AdornCsnappy(compress)},
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
