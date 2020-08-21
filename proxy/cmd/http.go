package cmd

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/thinkgos/jocasta/core/loadbalance"
	shttp "github.com/thinkgos/jocasta/services/http"
)

var httpCmd = &cobra.Command{
	Use:   "http",
	Short: "proxy on http mode",
	Run: func(cmd *cobra.Command, args []string) {
		if forever {
			return
		}

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
	// stcp 有效
	httpCfg.STCPConfig = stcpCfg
	// kcp 有效
	httpCfg.SKCPConfig = kcpCfg
	// ssh 有效
	flags.StringVarP(&httpCfg.SSHConfig.User, "ssh-user", "u", "", "user for ssh")
	flags.StringVarP(&httpCfg.SSHConfig.KeyFile, "ssh-key", "S", "", "private key file for ssh")
	flags.StringVarP(&httpCfg.SSHConfig.KeyFileSalt, "ssh-keysalt", "s", "", "salt of ssh private key")
	flags.StringVarP(&httpCfg.SSHConfig.Password, "ssh-password", "A", "", "password for ssh")
	// 其它
	flags.BoolVar(&httpCfg.Always, "always", false, "always use parent proxy")
	flags.DurationVar(&httpCfg.Timeout, "timeout", 2*time.Second, "tcp timeout when connect to real server or parent proxy")
	// 代理过滤
	flags.StringVar(&httpCfg.FilterConfig.Intelligent, "intelligent", "intelligent", "settting intelligent HTTP, SOCKS5 proxy mode, can be <intelligent|direct|parent>")
	flags.StringVarP(&httpCfg.FilterConfig.ProxyFile, "blocked", "b", "blocked", "blocked domain file , one domain each line")
	flags.StringVarP(&httpCfg.FilterConfig.DirectFile, "direct", "d", "direct", "direct domain file , one domain each line")
	flags.DurationVar(&httpCfg.FilterConfig.Timeout, "http-timeout", 3*time.Second, "check domain if blocked , http request timeout duration when connect to host")
	flags.DurationVar(&httpCfg.FilterConfig.Interval, "interval", 10*time.Second, "check domain if blocked every interval duration")
	// basic auth 配置
	flags.StringVarP(&httpCfg.AuthConfig.File, "auth-file", "F", "", "http basic auth file,\"username:password\" each line in file")
	flags.StringSliceVarP(&httpCfg.AuthConfig.UserPasses, "auth", "a", nil, "http basic auth username and password, multiple user repeat -a ,such as: -a user1:pass1 -a user2:pass2")
	flags.StringVar(&httpCfg.AuthConfig.URL, "auth-url", "", "http basic auth username and password will send to this url,response http code equal to 'auth-code' means ok,others means fail.")
	flags.DurationVar(&httpCfg.AuthConfig.Timeout, "auth-timeout", 3*time.Second, "access 'auth-url' timeout duration")
	flags.IntVar(&httpCfg.AuthConfig.OkCode, "auth-code", 204, "access 'auth-url' success http code")
	flags.UintVar(&httpCfg.AuthConfig.Retry, "auth-retry", 1, "access 'auth-url' fail and retry count")
	// dns服务
	flags.StringVarP(&httpCfg.DNSConfig.Addr, "dns-address", "q", "", "if set this, proxy will use this dns for resolve doamin")
	flags.IntVarP(&httpCfg.DNSConfig.TTL, "dns-ttl", "e", 300, "caching seconds of dns query result")
	// 负载均衡
	flags.StringVar(&httpCfg.LbConfig.Method, "lb-method", "roundrobin", fmt.Sprintf("load balance method when use multiple parent,can be one of <%s>", strings.Join(loadbalance.Methods(), ", ")))
	flags.DurationVar(&httpCfg.LbConfig.Timeout, "lb-timeout", 500*time.Millisecond, "tcp timeout duration of connecting to parent")
	flags.DurationVar(&httpCfg.LbConfig.RetryTime, "lb-retrytime", time.Second, "sleep time duration after checking")
	flags.BoolVar(&httpCfg.LbConfig.HashTarget, "lb-hashtarget", false, "use target address to choose parent for LB")
	// 限速器
	flags.StringVarP(&httpCfg.RateLimit, "rate-limit", "l", "0", "rate limit (bytes/second) of each connection, such as: 100K 1.5M . 0 means no limitation")
	flags.BoolVarP(&httpCfg.BindListen, "bind-listen", "B", false, "using listener binding IP when connect to target")
	flags.StringSliceVarP(&httpCfg.LocalIPS, "local-bind-ips", "g", nil, "if your host behind a nat,set your public ip here avoid dead loop")
	// 跳板机
	flags.StringVar(&httpCfg.RawProxyURL, "proxy", "", "https or socks5 proxies used when connecting to parent, only worked of -T is tls or tcp, format is https://username:password@host:port https://host:port or socks5://username:password@host:port socks5://host:port")
	rootCmd.AddCommand(httpCmd)
}
