package cmd

import (
	"log"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	shttp "github.com/thinkgos/jocasta/services/http"
)

var httpCmd = &cobra.Command{
	Use:   "http",
	Short: "proxy on http mode",
	Run: func(cmd *cobra.Command, args []string) {
		if forever {
			return
		}
		httpCfg.SKCPConfig = kcpCfg

		srv := shttp.New(zap.S(), httpCfg)
		err := srv.Start()
		if err != nil {
			log.Fatalf("run service [%s],%s", cmd.Name(), err)
		}
		server = srv
	},
}
var httpCfg shttp.Config

func init() {
	flags := httpCmd.Flags()
	// parent
	flags.StringVarP(&httpCfg.ParentType, "parent-type", "T", "", "parent protocol type <tcp|tls|stcp|ssh|kcp>")
	flags.StringSliceVarP(&httpCfg.Parent, "parent", "P", nil, "parent address, such as: \"23.32.32.19:28008\"")
	flags.BoolVarP(&httpCfg.ParentCompress, "parent-compress", "M", false, "auto compress/decompress data on parent connection")
	flags.StringVarP(&httpCfg.ParentKey, "parent-key", "Z", "", "the password for auto encrypt/decrypt parent connection data")
	// local
	flags.StringVarP(&httpCfg.LocalType, "local-type", "t", "tcp", "local protocol type <tcp|tls|stcp|kcp>")
	flags.StringVarP(&httpCfg.Local, "local", "p", ":28080", "local ip:port to listen,multiple address use comma split,such as: 0.0.0.0:80,0.0.0.0:443")
	flags.BoolVarP(&httpCfg.LocalCompress, "local-compress", "m", false, "auto compress/decompress data on local connection")
	flags.StringVarP(&httpCfg.LocalKey, "local-key", "z", "", "the password for auto encrypt/decrypt local connection data")
	// tls有效
	flags.StringVarP(&httpCfg.CertFile, "cert", "C", "proxy.crt", "cert file for tls")
	flags.StringVarP(&httpCfg.KeyFile, "key", "K", "proxy.key", "key file for tls")
	flags.StringVar(&httpCfg.CaCertFile, "ca", "", "ca cert file for tls")
	// ssh 有效
	flags.StringVarP(&httpCfg.SSHUser, "ssh-user", "u", "", "user for ssh")
	flags.StringVarP(&httpCfg.SSHKeyFile, "ssh-key", "S", "", "private key file for ssh")
	flags.StringVarP(&httpCfg.SSHKeyFileSalt, "ssh-keysalt", "s", "", "salt of ssh private key")
	flags.StringVarP(&httpCfg.SSHPassword, "ssh-password", "A", "", "password for ssh")
	// 其它
	flags.BoolVar(&httpCfg.Always, "always", false, "always use parent proxy")
	flags.DurationVar(&httpCfg.Timeout, "timeout", 2*time.Second, "tcp timeout when connect to real server or parent proxy")
	// 代理过滤
	flags.StringVarP(&httpCfg.ProxyFile, "blocked", "b", "blocked", "blocked domain file , one domain each line")
	flags.StringVarP(&httpCfg.DirectFile, "direct", "d", "direct", "direct domain file , one domain each line")
	flags.DurationVar(&httpCfg.HTTPTimeout, "http-timeout", 3*time.Second, "check domain if blocked , http request timeout duration when connect to host")
	flags.DurationVar(&httpCfg.Interval, "interval", 10*time.Second, "check domain if blocked every interval duration")
	// basic auth 配置
	flags.StringVarP(&httpCfg.AuthFile, "auth-file", "F", "", "http basic auth file,\"username:password\" each line in file")
	flags.StringSliceVarP(&httpCfg.Auth, "auth", "a", nil, "http basic auth username and password, multiple user repeat -a ,such as: -a user1:pass1 -a user2:pass2")
	flags.StringVar(&httpCfg.AuthURL, "auth-url", "", "http basic auth username and password will send to this url,response http code equal to 'auth-code' means ok,others means fail.")
	flags.DurationVar(&httpCfg.AuthURLTimeout, "auth-timeout", 3*time.Second, "access 'auth-url' timeout duration")
	flags.IntVar(&httpCfg.AuthURLOkCode, "auth-code", 204, "access 'auth-url' success http code")
	flags.UintVar(&httpCfg.AuthURLRetry, "auth-retry", 1, "access 'auth-url' fail and retry count")
	// dns服务
	flags.StringVarP(&httpCfg.DNSAddress, "dns-address", "q", "", "if set this, proxy will use this dns for resolve doamin")
	flags.IntVarP(&httpCfg.DNSTTL, "dns-ttl", "e", 300, "caching seconds of dns query result")
	// 代理模式
	flags.StringVar(&httpCfg.Intelligent, "intelligent", "intelligent", "settting intelligent HTTP, SOCKS5 proxy mode, can be <intelligent|direct|parent>")
	// 负载均衡
	flags.StringVar(&httpCfg.LoadBalanceMethod, "lb-method", "roundrobin", "load balance method when use multiple parent,can be <roundrobin|leastconn|leasttime|hash|weight>")
	flags.DurationVar(&httpCfg.LoadBalanceTimeout, "lb-timeout", 500*time.Millisecond, "tcp timeout duration of connecting to parent")
	flags.DurationVar(&httpCfg.LoadBalanceRetryTime, "lb-retrytime", time.Second, "sleep time duration after checking")
	flags.BoolVar(&httpCfg.LoadBalanceHashTarget, "lb-hashtarget", false, "use target address to choose parent for LB")
	flags.BoolVar(&httpCfg.LoadBalanceOnlyHA, "lb-onlyha", false, "use only `high availability mode` to choose parent for LB")
	// 限速器
	flags.StringVarP(&httpCfg.RateLimit, "rate-limit", "l", "0", "rate limit (bytes/second) of each connection, such as: 100K 1.5M . 0 means no limitation")
	flags.BoolVarP(&httpCfg.BindListen, "bind-listen", "B", false, "using listener binding IP when connect to target")
	flags.StringSliceVarP(&httpCfg.LocalIPS, "local-bind-ips", "g", nil, "if your host behind a nat,set your public ip here avoid dead loop")
	// 跳板机
	flags.StringVarP(&httpCfg.Jumper, "jumper", "J", "", "https or socks5 proxies used when connecting to parent, only worked of -T is tls or tcp, format is https://username:password@host:port https://host:port or socks5://username:password@host:port socks5://host:port")
	rootCmd.AddCommand(httpCmd)
}
