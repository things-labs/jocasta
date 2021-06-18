package cmd

import (
	"github.com/spf13/cobra"

	"github.com/thinkgos/jocasta/pkg/builder"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "show the version",
	Run: func(cmd *cobra.Command, args []string) {
		if forever {
			return
		}
		builder.PrintVersion()
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
