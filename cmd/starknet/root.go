package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "starknet",
	Short: "Starknet Go Client",
	Long:  "StarkNet Go Client Long",
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
