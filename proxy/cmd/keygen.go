package cmd

import (
	"log"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/thinkgos/jocasta/services/keygen"
)

var keygenCfg keygen.Config

var keygenCmd = &cobra.Command{
	Use:   "keygen",
	Short: "create certificate for proxy",
	Run: func(cmd *cobra.Command, args []string) {
		if forever {
			return
		}
		err := keygen.New(keygenCfg).Start()
		if err != nil {
			log.Fatalf("run service [%s],%s", cmd.Name(), err)
		}
		zap.S().Infof("success")
	},
}

func init() {
	flags := keygenCmd.Flags()

	flags.StringVarP(&keygenCfg.CaFilePrefix, "ca", "C", "", "ca file name prefix")
	flags.StringVarP(&keygenCfg.CertFilePrefix, "cert", "c", "", "cert file name prefix of sign")
	flags.StringVarP(&keygenCfg.CommonName, "cn", "n", "", "common name, if empty it will generate rand common name")
	flags.IntVarP(&keygenCfg.SignDays, "days", "d", 365, "days of sign")
	flags.BoolVarP(&keygenCfg.Sign, "sign", "s", false, "cert is to sign")

	rootCmd.AddCommand(keygenCmd)
}
