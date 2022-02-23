package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "juno",
	Short: "Juno, Starknet Client in Go",
	Long:  "Juno, StarkNet Client in Go",
	Run: func(cmd *cobra.Command, args []string) {
		log.Info(cmd.Short)
	},
}

func init() {
	rootCmd.AddCommand(cmdCall)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.WithField("Error", err).Error("Error executing CLI")
		return
	}
}
