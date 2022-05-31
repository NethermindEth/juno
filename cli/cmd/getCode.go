package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// getCodeCmd represents the getCode command
var getCodeCmd = &cobra.Command{
	Use:   "get_code CONTRACT_ADDRESS",
	Short: `List of bytecodes of deployed transaction.`,
	Long:  `https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-code`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("getCode called")
	},
}

func init() {
	rootCmd.AddCommand(getCodeCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// getCodeCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// getCodeCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
