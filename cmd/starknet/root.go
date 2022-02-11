package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "juno",
	Short: "Juno, Starknet Client in Go",
	Long:  "Juno, StarkNet Client in Go",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(cmd.Short)
	},
}

func init() {
	rootCmd.AddCommand(cmdCall)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		return
	}
}
