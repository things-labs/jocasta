package cmd

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/thinkgos/jocasta/cs"
	"github.com/thinkgos/jocasta/pkg/ccs"
)

var kcpCfg ccs.SKCPConfig

func kcp(cmd *cobra.Command) {
	persistent := cmd.PersistentFlags()
	persistent.StringVar(&kcpCfg.Key, "kcp-key", "secrect", "pre-shared secret between client and server")
	persistent.StringVar(&kcpCfg.Method, "kcp-method", "aes", "encrypt/decrypt method, can be on of "+strings.Join(cs.KcpBlockCryptMethods(), ","))
	persistent.StringVar(&kcpCfg.Mode, "kcp-mode", "fast", "profiles: fast3, fast2, fast, normal, manual")
	persistent.IntVar(&kcpCfg.MTU, "kcp-mtu", 450, "set maximum transmission unit for UDP packets")
	persistent.IntVar(&kcpCfg.SndWnd, "kcp-sndwnd", 1024, "set send window size(num of packets)")
	persistent.IntVar(&kcpCfg.RcvWnd, "kcp-rcvwnd", 1024, "set receive window size(num of packets)")
	persistent.IntVar(&kcpCfg.DataShard, "kcp-ds", 10, "set reed-solomon erasure coding - datashard")
	persistent.IntVar(&kcpCfg.ParityShard, "kcp-ps", 3, "set reed-solomon erasure coding - parityshard")
	persistent.IntVar(&kcpCfg.DSCP, "kcp-dscp", 0, "set DSCP(6bit)")
	persistent.BoolVar(&kcpCfg.AckNodelay, "kcp-acknodelay", true, "be carefully! flush ack immediately when a packet is received")
	persistent.IntVar(&kcpCfg.NoDelay, "kcp-nodelay", 0, "be carefully!")
	persistent.IntVar(&kcpCfg.Interval, "kcp-interval", 50, "be carefully!")
	persistent.IntVar(&kcpCfg.Resend, "kcp-resend", 0, "be carefully!")
	persistent.IntVar(&kcpCfg.NoCongestion, "kcp-nc", 0, "be carefully! no congestion")
	persistent.IntVar(&kcpCfg.SockBuf, "kcp-sockbuf", 4194304, "be carefully!")
	persistent.IntVar(&kcpCfg.KeepAlive, "kcp-keepalive", 10, "be carefully!")
}
