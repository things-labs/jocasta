package cmd

import (
	"log"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/thinkgos/jocasta/pkg/sword"
	sudp "github.com/thinkgos/jocasta/services/udp"
)

var udpCfg sudp.Config

var udpCmd = &cobra.Command{
	Use:   "udp",
	Short: "proxy on udp mode",
	Run: func(cmd *cobra.Command, args []string) {
		if forever {
			return
		}

		log.Println(udpCfg.SKCPConfig)
		srv := sudp.New(udpCfg,
			sudp.WithGPool(sword.GPool),
			sudp.WithLogger(zap.S()))
		err := srv.Start()
		if err != nil {
			log.Fatalf("run service [%s],%s", cmd.Name(), err)
		}
		server = srv
	},
}

func init() {
	flags := udpCmd.Flags()
	// parent
	flags.StringVarP(&udpCfg.ParentType, "parent-type", "T", "", "parent protocol type <tcp|tls|stcp|kcp|udp>")
	flags.StringVarP(&udpCfg.Parent, "parent", "P", "", "parent address, such as: \"192.168.100.100:100008\"")
	flags.BoolVarP(&udpCfg.ParentCompress, "parent-compress", "M", false, "auto compress/decompress data on parent connection")
	// local
	flags.StringVarP(&udpCfg.Local, "local", "p", ":22800", "local ip:port to listen")
	// tls
	flags.StringVarP(&udpCfg.CertFile, "cert", "C", "proxy.crt", "cert file for tls")
	flags.StringVarP(&udpCfg.KeyFile, "key", "K", "proxy.key", "key file for tls")
	flags.StringVar(&udpCfg.CaCertFile, "ca", "", "ca cert file for tls")
	// stcp
	udpCfg.STCPConfig = stcpCfg
	// kcp
	udpCfg.SKCPConfig = &kcpCfg
	// 其它
	flags.DurationVarP(&udpCfg.Timeout, "timeout", "e", time.Second*2, "tcp timeout duration when connect to real server or parent proxy")

	rootCmd.AddCommand(udpCmd)
}
