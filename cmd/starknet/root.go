package cmd

import (
	"github.com/NethermindEth/juno/internal/log"
	"github.com/spf13/cobra"
)

var logger = log.GetLogger()

var rootCmd = &cobra.Command{
	Use:   "juno",
	Short: "Juno, Starknet Client in Go",
	Long:  "Juno, StarkNet Client in Go",
	Run: func(cmd *cobra.Command, args []string) {
		logger.Info(cmd.Short)
	},
}

func init() {
	rootCmd.AddCommand(cmdCall)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logger.With("Error", err).Error("Error executing CLI")
		return
	}
}
