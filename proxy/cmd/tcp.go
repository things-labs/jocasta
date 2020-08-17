package cmd

import (
	"log"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/thinkgos/jocasta/lib/encrypt"
	"github.com/thinkgos/jocasta/pkg/sword"

	stcp "github.com/thinkgos/jocasta/services/tcp"
)

var tcpCfg stcp.Config

var tcpCmd = &cobra.Command{
	Use:   "tcp",
	Short: "proxy on tcp mode",
	Run: func(cmd *cobra.Command, args []string) {
		if forever {
			return
		}
		srv := stcp.New(tcpCfg,
			stcp.WithGPool(sword.GPool),
			stcp.WithLogger(zap.S()))
		err := srv.Start()
		if err != nil {
			log.Fatalf("run service [%s],%s", cmd.Name(), err)
		}
		server = srv
	},
}

func init() {
	flags := tcpCmd.Flags()

	// parent
	flags.StringVarP(&tcpCfg.ParentType, "parent-type", "T", "", "parent protocol type <tcp|tls|stcp|kcp|udp>")
	flags.StringVarP(&tcpCfg.Parent, "parent", "P", "", "parent address, such as: \"192.168.100.100:10000\"")
	flags.BoolVarP(&tcpCfg.ParentCompress, "parent-compress", "M", false, "auto compress/decompress data on parent connection")
	// local
	flags.StringVarP(&tcpCfg.LocalType, "local-type", "t", "tcp", "local protocol type <tcp|tls|stcp|kcp>")
	flags.StringVarP(&tcpCfg.Local, "local", "p", ":22800", "local ip:port to listen")
	flags.BoolVarP(&tcpCfg.LocalCompress, "local-compress", "m", false, "auto compress/decompress data on local connection")
	// tls有效
	flags.StringVarP(&tcpCfg.CertFile, "cert", "C", "proxy.crt", "cert file for tls")
	flags.StringVarP(&tcpCfg.KeyFile, "key", "K", "proxy.key", "key file for tls")
	flags.StringVar(&tcpCfg.CaCertFile, "ca", "", "ca cert file for tls")
	// stcp 有效
	flags.StringVar(&tcpCfg.STCPConfig.Method, "stcp-method", "aes-192-cfb", "method of local stcp's encrpyt/decrypt, these below are supported :\n"+strings.Join(encrypt.CipherMethods(), ","))
	flags.StringVar(&tcpCfg.STCPConfig.Password, "stcp-password", "thinkgos's_jocasta", "password of local stcp's encrpyt/decrypt")
	// kcp 有效
	tcpCfg.SKCPConfig = &kcpCfg
	// 其它
	flags.DurationVarP(&tcpCfg.Timeout, "timeout", "e", time.Second*2, "tcp timeout duration when connect to real server or parent proxy")
	// 跳板机
	flags.StringVar(&tcpCfg.RawProxyURL, "proxy", "", "https or socks5 proxies used when connecting to parent, only worked of -T is tls or tcp, format is https://username:password@host:port https://host:port or socks5://username:password@host:port socks5://host:port")

	rootCmd.AddCommand(tcpCmd)
}
