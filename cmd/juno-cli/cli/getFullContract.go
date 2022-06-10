/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cli

import (
	"fmt"
	"github.com/NethermindEth/juno/cmd/juno-cli"

	"github.com/spf13/cobra"
)

// getFullContractCmd represents the getFullContract command
var getFullContractCmd = &cobra.Command{
	Use:   "getFullContract",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("getFullContract called")
	},
}

func init() {
	main.rootCmd.AddCommand(getFullContractCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// getFullContractCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// getFullContractCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
