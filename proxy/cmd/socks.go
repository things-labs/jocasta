package cmd

import (
	"log"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	ssock "github.com/thinkgos/jocasta/services/socks"
)

var socksCfg ssock.Config

var socksCmd = &cobra.Command{
	Use:   "socks",
	Short: "proxy on socks mode",
	Run: func(cmd *cobra.Command, args []string) {
		if forever {
			return
		}
		socksCfg.SKCPConfig = kcpCfg
		socksCfg.Debug = hasDebug

		server = ssock.New(zap.S(), socksCfg)
		err := server.Start()
		if err != nil {
			log.Fatalf("run service [%s],%s", cmd.Name(), err)
		}
	},
}

func init() {
	flags := socksCmd.Flags()

	// parent
	flags.StringVarP(&socksCfg.ParentType, "parent-type", "T", "", "parent protocol type <tcp|tls|stcp|kcp|ssh>")
	flags.StringSliceVarP(&socksCfg.Parent, "parent", "P", nil, "parent address, such as: \"23.32.32.19:28008\"")
	flags.BoolVarP(&socksCfg.ParentCompress, "parent-compress", "M", false, "auto compress/decompress data on parent connection")
	flags.StringVarP(&socksCfg.ParentKey, "parent-key", "Z", "", "the password for auto encrypt/decrypt parent connection data")
	flags.StringVarP(&socksCfg.ParentAuth, "parent-auth", "A", "", "parent socks auth username and password, such as: -A user1:pass1")
	// local
	flags.StringVarP(&socksCfg.LocalType, "local-type", "t", "tcp", "local protocol type <tcp|tls|stcp|kcp>")
	flags.StringVarP(&socksCfg.Local, "local", "p", ":28080", "local ip:port to listen,multiple address use comma split,such as: 0.0.0.0:80,0.0.0.0:443")
	flags.BoolVarP(&socksCfg.LocalCompress, "local-compress", "m", false, "auto compress/decompress data on local connection")
	flags.StringVarP(&socksCfg.LocalKey, "local-key", "z", "", "the password for auto encrypt/decrypt local connection data")
	// tls
	flags.StringVarP(&socksCfg.CertFile, "cert", "C", "proxy.crt", "cert file for tls")
	flags.StringVarP(&socksCfg.KeyFile, "key", "K", "proxy.key", "key file for tls")
	flags.StringVar(&socksCfg.CaCertFile, "ca", "", "ca cert file for tls")
	// stcp
	socksCfg.STCPConfig = stcpCfg
	// ssh
	flags.StringVarP(&socksCfg.SSHConfig.User, "ssh-user", "u", "", "user for ssh")
	flags.StringVarP(&socksCfg.SSHConfig.KeyFile, "ssh-key", "S", "", "private key file for ssh")
	flags.StringVarP(&socksCfg.SSHConfig.KeyFileSalt, "ssh-keysalt", "s", "", "salt of ssh private key")
	flags.StringVarP(&socksCfg.SSHConfig.Password, "ssh-password", "D", "", "password for ssh")
	// 其它
	flags.DurationVar(&socksCfg.Timeout, "timeout", 5*time.Second, "tcp timeout duration when connect to real server or parent proxy")
	flags.BoolVar(&socksCfg.Always, "always", false, "always use parent proxy")
	// 代理过滤
	flags.StringVar(&socksCfg.FilterConfig.Intelligent, "intelligent", "intelligent", "settting intelligent HTTP, SOCKS5 proxy mode, can be <intelligent|direct|parent>")
	flags.StringVarP(&socksCfg.FilterConfig.ProxyFile, "blocked", "b", "blocked", "blocked domain file , one domain each line")
	flags.StringVarP(&socksCfg.FilterConfig.DirectFile, "direct", "d", "direct", "direct domain file , one domain each line")
	flags.DurationVar(&socksCfg.FilterConfig.Interval, "interval", 10*time.Second, "check domain if blocked every interval duration")
	// basic auth 配置
	flags.StringVarP(&socksCfg.AuthConfig.File, "auth-file", "F", "", "http basic auth file,\"username:password\" each line in file")
	flags.StringSliceVarP(&socksCfg.AuthConfig.UserPasses, "auth", "a", nil, "http basic auth username and password, multiple user repeat -a ,such as: -a user1:pass1 -a user2:pass2")
	flags.StringVar(&socksCfg.AuthConfig.URL, "auth-url", "", "http basic auth username and password will send to this url,response http code equal to 'auth-code' means ok,others means fail.")
	flags.DurationVar(&socksCfg.AuthConfig.Timeout, "auth-timeout", 3*time.Second, "access 'auth-url' timeout duration")
	flags.IntVar(&socksCfg.AuthConfig.OkCode, "auth-code", 204, "access 'auth-url' success http code")
	flags.UintVar(&socksCfg.AuthConfig.Retry, "auth-retry", 0, "access 'auth-url' fail and retry count")
	// dns域名解析
	flags.StringVarP(&socksCfg.DNSConfig.Addr, "dns-address", "q", "", "if set this, proxy will use this dns for resolve doamin")
	flags.IntVarP(&socksCfg.DNSConfig.TTL, "dns-ttl", "e", 300, "caching seconds of dns query result")
	// 负载均衡
	flags.StringVar(&socksCfg.LbConfig.Method, "lb-method", "roundrobin", "load balance method when use multiple parent,can be <roundrobin|leastconn|leasttime|hash|weight>")
	flags.DurationVar(&socksCfg.LbConfig.Timeout, "lb-timeout", 500*time.Millisecond, "tcp duration timeout of connecting to parent")
	flags.DurationVar(&socksCfg.LbConfig.RetryTime, "lb-retrytime", 1*time.Second, "sleep time duration after checking")
	flags.BoolVar(&socksCfg.LbConfig.HashTarget, "lb-hashtarget", false, "use target address to choose parent for LB")
	// 限速器
	flags.StringVarP(&socksCfg.RateLimit, "rate-limit", "l", "0", "rate limit (bytes/second) of each connection, such as: 100K 1.5M . 0 means no limitation")
	flags.StringSliceVarP(&socksCfg.LocalIPS, "local-bind-ips", "g", nil, "if your host behind a nat,set your public ip here avoid dead loop")
	flags.BoolVarP(&socksCfg.BindListen, "bind-listen", "B", false, "using listener binding IP when connect to target")

	rootCmd.AddCommand(socksCmd)
}
