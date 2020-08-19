package cmd

import (
	"log"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/thinkgos/jocasta/pkg/sword"
	"github.com/thinkgos/jocasta/services/mux"
)

var muxServer mux.ServerConfig

var muxServerCmd = &cobra.Command{
	Use:   "server",
	Short: "proxy on mux server mode",
	Run: func(cmd *cobra.Command, args []string) {
		if forever {
			return
		}
		muxServer.SKCPConfig = kcpCfg

		srv := mux.NewServer(muxServer,
			mux.WithServerLogger(zap.S()),
			mux.WithServerGPool(sword.GPool))
		err := srv.Start()
		if err != nil {
			log.Fatalf("run service [%s],%s", cmd.Name(), err)
		}
		server = srv
	},
}

func init() {
	flags := muxServerCmd.Flags()

	flags.StringVarP(&muxServer.ParentType, "parent-type", "T", "tcp", "parent protocol type <tcp|tls|stcp|kcp>")
	flags.StringVarP(&muxServer.Parent, "parent", "P", "", "parent address, such as: \"23.32.32.19:28008\"")
	flags.BoolVar(&muxServer.Compress, "compress", false, "compress data when tcp|tls|stcp mode")
	flags.StringVar(&muxServer.SecretKey, "sk", "default", "key same with server")
	// tls
	flags.StringVarP(&muxServer.CertFile, "cert", "C", "proxy.crt", "cert file for tls")
	flags.StringVarP(&muxServer.KeyFile, "key", "K", "proxy.key", "key file for tls")
	// stcp
	muxServer.STCPConfig = stcpCfg
	// 其它
	flags.DurationVarP(&muxServer.Timeout, "timeout", "i", time.Second*2, "tcp timeout duration when connect to real server or parent proxy")
	// 路由
	flags.BoolVar(&muxServer.IsUDP, "udp", false, "proxy on udp mux server mode")
	flags.StringVarP(&muxServer.Route, "route", "r", "", "local route to client's network, such as: PROTOCOL://LOCAL_IP:LOCAL_PORT@[CLIENT_KEY]CLIENT_LOCAL_HOST:CLIENT_LOCAL_PORT")
	// proxy
	flags.StringVar(&muxServer.RawProxyURL, "proxy", "", "https or socks5 proxies used when connecting to parent, only worked of -T is tls or tcp, format is https://username:password@host:port https://host:port or socks5://username:password@host:port socks5://host:port")

	rootCmd.AddCommand(muxServerCmd)
}
