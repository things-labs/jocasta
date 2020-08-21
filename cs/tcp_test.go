package cs

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thinkgos/go-socks5"

	"github.com/thinkgos/jocasta/lib/cert"
	"github.com/thinkgos/jocasta/lib/encrypt"
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
				Timeout:          time.Second,
				AfterAdornChains: AdornConnsChain{AdornCsnappy(compress)},
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
				socks5.WithAuthMethods([]socks5.Authenticator{new(socks5.NoAuthAuthenticator), cator}),
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
				Timeout:          time.Second,
				Forward:          Socks5{pURL.Host, ProxyAuth(pURL), time.Second, nil},
				AfterAdornChains: AdornConnsChain{AdornCsnappy(compress)},
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

var base64CaCrt = `base64://LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURsakNDQW42Z0F3SUJBZ0lCQVRBTkJna3Foa2lHOXcwQkFRc0ZBREJjTVFzd0NRWURWUVFHRXdKQlRERUoKTUFjR0ExVUVDQk1BTVFrd0J3WURWUVFIRXdBeEVUQVBCZ05WQkFvVENIVnFjbkZtTG0xNk1SRXdEd1lEVlFRTApFd2gxYW5KeFppNXRlakVSTUE4R0ExVUVBeE1JZFdweWNXWXViWG93SUJjTk1qQXdPREl3TURRMU1UUXdXaGdQCk1qRXlNREEzTWpjd05UVXhOREJhTUZ3eEN6QUpCZ05WQkFZVEFrRk1NUWt3QndZRFZRUUlFd0F4Q1RBSEJnTlYKQkFjVEFERVJNQThHQTFVRUNoTUlkV3B5Y1dZdWJYb3hFVEFQQmdOVkJBc1RDSFZxY25GbUxtMTZNUkV3RHdZRApWUVFERXdoMWFuSnhaaTV0ZWpDQ0FTSXdEUVlKS29aSWh2Y05BUUVCQlFBRGdnRVBBRENDQVFvQ2dnRUJBTzhOCkYrbmQ2dzZZWUVucEhYMlZ1SjlWdkZUbWhzM3lpWnFFeVBoT3dQNk1ReHgyTlJsUm8zUTJrQWRwbWtHQmwrdkYKaC9Gei9HRFpaQmVYU0VyaGQ4NXVtWGlDSk93UDJzWWxXTk13VHRkckhmMDBDcy9nR0loME9XMUxuNHVDYzNORApvcEFLK3o5NTAxMlAwakVna0VkME1SeDd3OHhPaEp3TEltZkgvOUtkSEYzRmtrWGxmUHRZb09RYW0vQTV5UXZMCmhPK0lFcDdmMElSRGZyZi9pODlyNVZtYkRXb3BWZWVhcmFYQ0ltdDZuOHVhR3l5aWtoL0JrUjdjRWo2VHh6bHgKRVBsOWZJSmdDNGI2SC91ZmxJSWJCSlFzWnYydytRZ3g4NEh5Nm85YXhzR3BLTHJiVTExa3pERWwrams4RDN1NwoyWVEvOGlvdml2eTV5SDFjcjZFQ0F3RUFBYU5oTUY4d0RnWURWUjBQQVFIL0JBUURBZ0trTUIwR0ExVWRKUVFXCk1CUUdDQ3NHQVFVRkJ3TUJCZ2dyQmdFRkJRY0RBakFQQmdOVkhSTUJBZjhFQlRBREFRSC9NQjBHQTFVZERnUVcKQkJUaW9kc3B0T2NWN2pQdTFLVlh6bjd1TzNocDBqQU5CZ2txaGtpRzl3MEJBUXNGQUFPQ0FRRUFuaGNVSWlzegpvKzE0VVNIK2p5ZzdrSklQM3FuYnBudUxmbTNuekxTUHZMRk9qNFRNY2RWN3pKNWlLQWNyN2QxNGhCZ0hJb3pjCk0wUkhWdGU4UVZUU2dRUmZWYjhJZ1VxMmRnQzN2WDVYT1J6QnA3aEZRMlY1Z2tsV3JvamVBaGE0UE92a1d5T3UKTWgxYklyQWdaVW5SSU5rM3pNVFA5RDlmMUF5YTVZWVpOL2lEOURXTjNwS296clVDYnNlcUt2eUpNV3lNb1ZUVApmMTVnM1YraVFleDhjWnRSQWlzc1lMQS9vUUdiRFhocG1HaHRTQkhYT3NTYjJwODU1NTFjeGk1allWRzF5dmtkClZqZmxKeFBpSit5cU8vUWluWlF4Y2FieDl2Ung2QXc0aXJyRW0xSzl2L3B3Q2Y3MmJVNkFCTWJMcjM3aWowUUIKdmRpNmptRTNkSXMrQ3c9PQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==`
var base64Key = `base64://LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFBS0NBUUVBcml2RkxGUUQ3Mi9zNHQwR25mZC9zMDFaWHVURDkzVkc0dUJKUTZrK2FOK3VwRGx3CmJrdkF2eEhzd2RacWlRcTBzWklIMEZEZVZNUDdGVnZKNFRoajBSbGlhL1ZtZENiZXV3YXhKMFFRdTdmbFowQnEKa1RRc0k2Ryt3MW9OZ3BldUs0cGtLYSswRlg1Uy9Pa0w3QS9BQ3hrR0xUSFkyc0hzVTZFUWVwZFBJcUdFVFIrOAorS0hzYVNXQkU5TUg0N0x1TmhOdFV1UjJtL3ZXa0ROWmd5MjlqcHVSbGJMVG9jYXlocmRuMlJXL0hpSkYyejlDClh1VnRwN21wU2RPUFQxcURneml6SnpNcXJBbFhqUnZ2Y3AzK09ZcWgvUWNjZWdQZTRrbFBwU3d2QkxtTHlHSUoKNmIvalJ4L2t1RlJxVWpQWktzVEs0WFNQcGt1TFJJOStJb3NDaVFJREFRQUJBb0lCQUdxaTE5UzlGNis0U2tQWAo2VjQ4RTA1M05CbWFYTFVjYzZzaWdSalEvU24ra1pGd3JYcUtITmVmSk9MRTRYTklQN3RjV2paYjNOUHZMY0kwCjhaUllEeDBOdXFtNGxPTFdqWlhha2dvbGJVMVJXVDN2UFkvcmJuN2VYOHFCVHpZK29kOThtcmkvSDdYdEl1Zy8KMHhIVHp0TzhuWUI4UFhZcU53UTQzN3IrK3UrTEMydGVJcEZFMHRWUlVHRlVsNC9Ick5IUldSVUIydjNHL1hHdgpRc2NQbkh5MFJXNG1KSnpaTkNTazdYS2FhN3BMblFpRVprVVJ6MFAxSU1SdDhDMEIxUGF3Skh4eTNKTHU1bndzCnUxQ2xFRitCWXRzWmJwVUxPTzEwcTg5NVErTldSZ2tCSjVvWUw4WGZjbnh4QVgrYW13MzNqZUwvK3pncFhvaUEKZWlPdW8wVUNnWUVBM2pWQ2U1MExkNSt2L2xsYzN6ZTlobmcvMm5seFBBTHlIQ3I1SHVXdDZuUzNWdlVYL2tNdQpFemxUQW9RSjFsMEdUS0dOaWpPNEtPQmVaRmFxSGFScW5QSkRhdHJLMithTkt5VmJFMUd6dlY1T01uWE5nV01iCmVkaEZXTGQxSklxVVdlamladUhWazQ3b3VjNzk1aUNmeU1KZEpWUXVUUjlrYjBZYUkwNXRDdzhDZ1lFQXlLaGoKWnlUd28wSW45b0oyeEFMWFBpd1pMK2UzWVhldFFRcldMSU5qdmhJSVp6SWZ2cFI2RzBMeWt5NnRlMjhMOXIvRApGL3MvMGRzdFJ2Z0JtTDcyemRjV1RGSU5OT2JPR29JOFVUMFVvcy91b09UZ1NYZGdKZzBDb3NTcjdLOU5qWDdPCnVnS2o2OGdycm1SN2wxL25RWUx5eS9tMStTNU1rU29tSmd1WmVPY0NnWUEzYjVZY2ZMdUZwb01qczJ1bURLUHQKemtZdjNTaFF6bk5VYjZ3aTl5NENWZ2YvVzdvV3B1L3h2OVZqeFVrbS82STRyLzA5dzFWb2JVRmZTdmU1MlhEMwpvc0VMQVhhbWtjZ2FRWUdOT2s3YktaSGNxT01TS29XS1NsaFNuOEpMMFg3ZFIycGtQR2NxSHZjVWsvMW4waS9LCnlDS3cwMldSdkFrMGNZckgwQ01JSlFLQmdCVXByVkl6a3kya3pKSXpNRk43K1RlVmJ4S2VYYmpwNWx2MXlLRHAKT3B2UlN4R1hOOEhoNXY3NUVKeDFUOWEwMzQ0bXEyemNFdDhQUHJjOWNPMnJObHgvVXRxNXpLckhyUFVleHozSwpNMkw0aXpKczhUck9YRlduQ2FBaEhnRFlmRlZ3Q2wyVDZiWDBacWI5OGdzRkVsN1djTVVTa1I2MncxeU1GWG50CnYwSUJBb0dCQUx0UTBrb09EUzJtb1NoQnE5VEphLy95Q2o3RStxUXFPOGNBMk5MamxsZlM0UHArdWRBTFdQTE4KeUY4Q1QxQm5hajUxYk16UVNJN1lRYytzbzc1N3Qwc1VmbXo5VDdseWF3bXlFTEFIRGs3NzIvUmhsTENRTXFCNQpZZjFuWnp4RnRhNHpzSnV1UElnV24vWlhBckc0dUg4N3ZFNVZXY1I2bld1YlJ0dXRpREplCi0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg==`
var base64Crt = `base64://LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURzVENDQXBtZ0F3SUJBZ0lJT0FjRXUzdE5mQU13RFFZSktvWklodmNOQVFFTEJRQXdYREVMTUFrR0ExVUUKQmhNQ1FVd3hDVEFIQmdOVkJBZ1RBREVKTUFjR0ExVUVCeE1BTVJFd0R3WURWUVFLRXdoMWFuSnhaaTV0ZWpFUgpNQThHQTFVRUN4TUlkV3B5Y1dZdWJYb3hFVEFQQmdOVkJBTVRDSFZxY25GbUxtMTZNQ0FYRFRJd01EZ3lNREEwCk5UWXdNVm9ZRHpJeE1qQXdOekkzTURVMU5qQXhXakJjTVFzd0NRWURWUVFHRXdKVFRERUpNQWNHQTFVRUNCTUEKTVFrd0J3WURWUVFIRXdBeEVUQVBCZ05WQkFvVENIZGhaR2szTG0xNE1SRXdEd1lEVlFRTEV3aDNZV1JwTnk1dAplREVSTUE4R0ExVUVBeE1JZDJGa2FUY3ViWGd3Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLCkFvSUJBUUN1SzhVc1ZBUHZiK3ppM1FhZDkzK3pUVmxlNU1QM2RVYmk0RWxEcVQ1bzM2NmtPWEJ1UzhDL0VlekIKMW1xSkNyU3hrZ2ZRVU41VXcvc1ZXOG5oT0dQUkdXSnI5V1owSnQ2N0JyRW5SQkM3dCtWblFHcVJOQ3dqb2I3RApXZzJDbDY0cmltUXByN1FWZmxMODZRdnNEOEFMR1FZdE1kamF3ZXhUb1JCNmwwOGlvWVJOSDd6NG9leHBKWUVUCjB3ZmpzdTQyRTIxUzVIYWIrOWFRTTFtRExiMk9tNUdWc3RPaHhyS0d0MmZaRmI4ZUlrWGJQMEplNVcybnVhbEoKMDQ5UFdvT0RPTE1uTXlxc0NWZU5HKzl5bmY0NWlxSDlCeHg2QTk3aVNVK2xMQzhFdVl2SVlnbnB2K05ISCtTNApWR3BTTTlrcXhNcmhkSSttUzR0RWozNGlpd0tKQWdNQkFBR2pkVEJ6TUE0R0ExVWREd0VCL3dRRUF3SUVrREFkCkJnTlZIU1VFRmpBVUJnZ3JCZ0VGQlFjREFRWUlLd1lCQlFVSEF3SXdEQVlEVlIwVEFRSC9CQUl3QURBZkJnTlYKSFNNRUdEQVdnQlRpb2RzcHRPY1Y3alB1MUtWWHpuN3VPM2hwMGpBVEJnTlZIUkVFRERBS2dnaDNZV1JwTnk1dAplREFOQmdrcWhraUc5dzBCQVFzRkFBT0NBUUVBWTJidlQwYmEwbi9Oc0pmcThjbUdZazc1WnQxRFZMeExTaXVLClJWTHBHQ1FZS2FsaGFjSUIxaDMwam1SZmlPaG90WHA5d3BpWUI5T1NZWUk1UFdpcnROalFtbFprUGZkYWFPbXQKMXZjUk0yZ1hrcUlUTW1vVHFQOFlLeDRxTm9rTzM2OFB0K3hmTXNIdEgyQlJQaHNpblZSZmxLS2NHYmowNzlhegpVR25nU1lVWldpS1piT0twbkttaXVlUnFuMm5rMG5wZnJDaWhDN3N6ejBNMjNCLy9BZUk4czNmNERwaEZ2bTJECmFGM0FYR3Jrd3FjTXhPZTVwS3lhQk9FWC91ZWFkNk85Rnk3R2JQbzlIaDdvcWpyU0szZ216VFErSVZ2L3MxcXoKV0N0dlBRenk4MU1RM0o0aFU1RUFMSzFhVW1CcVBpS3NvUEM2b1dqSSswUDFIVE9NM1E9PQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==`

func TestTcpTls_Forward_Direct(t *testing.T) {
	caCrt, err := cert.LoadCrt(base64CaCrt)
	require.NoError(t, err)
	crt, key, err := cert.LoadPair(base64Crt, base64Key)
	require.NoError(t, err)

	for _, compress := range []bool{true, false} {
		for _, single := range []bool{true, false} {
			func() {
				srvCfg := TLSConfig{
					CaCert: caCrt,
					Cert:   crt,
					Key:    key,
					Single: single,
				}

				serverConfig, err := srvCfg.ServerConfig()
				require.NoError(t, err)

				// server
				srv := &TCPServer{
					Addr:          "127.0.0.1:0",
					Status:        make(chan error, 1),
					BaseAdornConn: BaseTLSAdornServer(serverConfig),
					AfterChains:   AdornConnsChain{AdornCsnappy(compress)},
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
					CaCert: caCrt,
					Cert:   crt,
					Key:    key,
					Single: single,
				}
				clientConfig, err := cliConfig.ClientConfig()
				require.NoError(t, err)

				d := &TCPDialer{
					Timeout:          time.Second,
					BaseAdorn:        BaseTLSAdornClient(clientConfig),
					AfterAdornChains: AdornConnsChain{AdornCsnappy(compress)},
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
	caCrt, err := cert.LoadCrt(base64CaCrt)
	require.NoError(t, err)
	crt, key, err := cert.LoadPair(base64Crt, base64Key)
	require.NoError(t, err)

	for _, compress := range []bool{true, false} {
		for _, single := range []bool{true, false} {
			func() {
				srvCfg := TLSConfig{
					CaCert: caCrt,
					Cert:   crt,
					Key:    key,
					Single: single,
				}

				serverConfig, err := srvCfg.ServerConfig()
				require.NoError(t, err)

				// server
				srv := &TCPServer{
					Addr:          "127.0.0.1:0",
					Status:        make(chan error, 1),
					BaseAdornConn: BaseTLSAdornServer(serverConfig),
					AfterChains:   AdornConnsChain{AdornCsnappy(compress)},
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
						[]socks5.Authenticator{new(socks5.NoAuthAuthenticator), cator}),
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
					CaCert: caCrt,
					Cert:   crt,
					Key:    key,
					Single: single,
				}
				clientConfig, err := cliConfig.ClientConfig()
				require.NoError(t, err)
				d := &TCPDialer{
					Timeout:          time.Second,
					Forward:          Socks5{pURL.Host, ProxyAuth(pURL), time.Second, nil},
					BaseAdorn:        BaseTLSAdornClient(clientConfig),
					AfterAdornChains: AdornConnsChain{AdornCsnappy(compress)},
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

func TestSTCP_Forward_Direct(t *testing.T) {
	password := "pass_word"
	for _, method := range encrypt.CipherMethods() {
		for _, compress := range []bool{true, false} {
			func() {
				// server
				srv := &TCPServer{
					Addr:          "127.0.0.1:0",
					Status:        make(chan error, 1),
					BaseAdornConn: BaseStcpAdorn(method, password),
					AfterChains:   AdornConnsChain{AdornCsnappy(compress)},
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

				d := &TCPDialer{
					Timeout:          time.Second,
					BaseAdorn:        BaseStcpAdorn(method, password),
					AfterAdornChains: AdornConnsChain{AdornCsnappy(compress)},
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
				srv := &TCPServer{
					Addr:          "127.0.0.1:0",
					Status:        make(chan error, 1),
					BaseAdornConn: BaseStcpAdorn(method, password),
					AfterChains:   AdornConnsChain{AdornCsnappy(compress)},
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

				d := &TCPDialer{
					Timeout:          time.Second,
					Forward:          Socks5{pURL.Host, ProxyAuth(pURL), time.Second, nil},
					BaseAdorn:        BaseStcpAdorn(method, password),
					AfterAdornChains: AdornConnsChain{AdornCsnappy(compress)},
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
	srv := &TCPServer{
		Addr:          "127.0.0.1:0",
		Status:        make(chan error, 1),
		BaseAdornConn: BaseStcpAdorn(method, password),
		AfterChains:   AdornConnsChain{AdornCsnappy(compress)},
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
			d := TCPDialer{
				Timeout:          time.Second,
				BaseAdorn:        BaseStcpAdorn(method, password),
				AfterAdornChains: AdornConnsChain{AdornCsnappy(compress)},
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
