package cmd

import (
	"log"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	ssps "github.com/thinkgos/jocasta/services/sps"
)

var spsCfg ssps.Config
var spsCmd = &cobra.Command{
	Use:   "sps",
	Short: "proxy on sps mode",
	Run: func(cmd *cobra.Command, args []string) {
		if forever {
			return
		}
		spsCfg.SKCPConfig = kcpCfg
		spsCfg.Debug = hasDebug
		server = ssps.New(zap.S(), spsCfg)
		err := server.Start()
		if err != nil {
			log.Fatalf("run service [%s],%s", cmd.Name(), err)
		}
	},
}

func init() {
	flags := spsCmd.Flags()

	// parent
	flags.StringVarP(&spsCfg.ParentType, "parent-type", "T", "", "parent protocol type <tcp|tls|stcp|kcp>")
	flags.StringSliceVarP(&spsCfg.Parent, "parent", "P", nil, "parent address, such as: \"23.32.32.19:28008\"")
	flags.BoolVarP(&spsCfg.ParentCompress, "parent-compress", "M", false, "auto compress/decompress data on parent connection")
	flags.StringVarP(&spsCfg.ParentKey, "parent-key", "Z", "", "the password for auto encrypt/decrypt parent connection data")
	flags.StringVarP(&spsCfg.ParentAuth, "parent-auth", "A", "", "parent socks auth username and password, such as: -A user1:pass1")
	flags.BoolVar(&spsCfg.ParentTLSSingle, "parent-tls-single", false, "conntect to parent insecure skip verify")

	// local
	flags.StringVarP(&spsCfg.LocalType, "local-type", "t", "tcp", "local protocol type <tcp|tls|stcp|kcp>")
	flags.StringVarP(&spsCfg.Local, "local", "p", ":28080", "local ip:port to listen,multiple address use comma split,such as: 0.0.0.0:80,0.0.0.0:443")
	flags.BoolVarP(&spsCfg.LocalCompress, "local-compress", "m", false, "auto compress/decompress data on local connection")
	flags.StringVarP(&spsCfg.LocalKey, "local-key", "z", "", "the password for auto encrypt/decrypt local connection data")
	// tls有效
	flags.StringVarP(&spsCfg.CertFile, "cert", "C", "proxy.crt", "cert file for tls")
	flags.StringVarP(&spsCfg.KeyFile, "key", "K", "proxy.key", "key file for tls")
	flags.StringVar(&spsCfg.CaCertFile, "ca", "", "ca cert file for tls")
	// stcp有效
	spsCfg.STCPConfig = stcpCfg
	// 其它
	flags.DurationVar(&spsCfg.Timeout, "timeout", 5*time.Second, "tcp timeout duration when connect to real server or parent proxy")
	// basic auth 配置
	flags.StringVarP(&spsCfg.AuthFile, "auth-file", "F", "", "http basic auth file,\"username:password\" each line in file")
	flags.StringSliceVarP(&spsCfg.Auth, "auth", "a", nil, "http basic auth username and password, multiple user repeat -a ,such as: -a user1:pass1 -a user2:pass2")
	flags.StringVar(&spsCfg.AuthURL, "auth-url", "", "http basic auth username and password will send to this url,response http code equal to 'auth-code' means ok,others means fail.")
	flags.DurationVar(&spsCfg.AuthURLTimeout, "auth-timeout", 3*time.Second, "access 'auth-url' timeout duration")
	flags.IntVar(&spsCfg.AuthURLOkCode, "auth-code", 204, "access 'auth-url' success http code")
	flags.UintVar(&spsCfg.AuthURLRetry, "auth-retry", 0, "access 'auth-url' fail and retry count")
	// dns域名解析
	flags.StringVarP(&spsCfg.DNSAddress, "dns-address", "q", "", "if set this, proxy will use this dns for resolve doamin")
	flags.IntVarP(&spsCfg.DNSTTL, "dns-ttl", "e", 300, "caching seconds of dns query result")
	// 负载均衡
	flags.StringVar(&spsCfg.LoadBalanceMethod, "lb-method", "roundrobin", "load balance method when use multiple parent,can be <roundrobin|leastconn|leasttime|hash|weight>")
	flags.DurationVar(&spsCfg.LoadBalanceTimeout, "lb-timeout", 500*time.Millisecond, "tcp duration timeout of connecting to parent")
	flags.DurationVar(&spsCfg.LoadBalanceRetryTime, "lb-retrytime", time.Second, "sleep time duration after checking")
	flags.BoolVar(&spsCfg.LoadBalanceHashTarget, "lb-hashtarget", false, "use target address to choose parent for LB")
	// 限速器
	flags.StringVarP(&spsCfg.RateLimit, "rate-limit", "l", "0", "rate limit (bytes/second) of each connection, such as: 100K 1.5M . 0 means no limitation")
	flags.StringSliceVarP(&spsCfg.LocalIPS, "local-bind-ips", "g", nil, "if your host behind a nat,set your public ip here avoid dead loop")
	flags.StringVar(&spsCfg.RawProxyURL, "proxy", "", "https or socks5 proxies used when connecting to parent, only worked of -T is tls or tcp, format is https://username:password@host:port https://host:port or socks5://username:password@host:port socks5://host:port")

	flags.StringVarP(&spsCfg.ParentServiceType, "parent-service-type", "S", "", "parent service type <http|socks|ss>")
	flags.StringVarP(&spsCfg.ParentSSMethod, "parent-ss-method", "X", "aes-256-cfb", "the following methods are supported: aes-128-cfb, aes-192-cfb, aes-256-cfb, bf-cfb, cast5-cfb, des-cfb, rc4-md5, rc4-md5-6, chacha20, salsa20, rc4, table, des-cfb, chacha20-ietf; if you use ss server as parent, \"-T tcp\" is required")
	flags.StringVarP(&spsCfg.ParentSSKey, "parent-ss-key", "J", "sspassword", "if you use ss server as parent, \"-T tcp\" is required")
	flags.StringVarP(&spsCfg.SSMethod, "ss-method", "x", "aes-256-cfb", "the following methods are supported: aes-128-cfb, aes-192-cfb, aes-256-cfb, bf-cfb, cast5-cfb, des-cfb, rc4-md5, rc4-md5-6, chacha20, salsa20, rc4, table, des-cfb, chacha20-ietf; if you use ss client , \"-t tcp\" is required")
	flags.StringVarP(&spsCfg.SSKey, "ss-key", "j", "sspassword", "if you use ss client , \"-t tcp\" is required")
	flags.BoolVar(&spsCfg.DisableHTTP, "disable-http", false, "disable http(s) proxy")
	flags.BoolVar(&spsCfg.DisableSocks5, "disable-socks", false, "disable socks proxy")
	flags.BoolVar(&spsCfg.DisableSS, "disable-ss", false, "disable ss proxy")

	rootCmd.AddCommand(spsCmd)
}
