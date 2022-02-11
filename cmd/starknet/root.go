package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "juno",
	Short: "Juno, Starknet Go Client",
	Long:  "Juno, StarkNet Go Client",
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
