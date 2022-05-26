/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// getTransactionCmd represents the getTransaction command
var getTransactionCmd = &cobra.Command{
	Use:   "get_transaction --hash TRANSACTION_HASH",
	Short: "The result contains transaction information",
	Long:  `https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-transaction`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("getTransaction called")
	},
}

func init() {
	rootCmd.AddCommand(getTransactionCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// getTransactionCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// getTransactionCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
