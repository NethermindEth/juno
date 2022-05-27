package cmd

import (
	"fmt"
	"os/exec"

	"github.com/NethermindEth/juno/internal/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var getTransactionReceiptCmd = &cobra.Command{ // Get_Transaction Receipt CLI command
	Use:   "get_transaction_receipt TRANSACTION_HASH [--network NETWORK (WIP)]",
	Short: "Prints out transaction receipt information.",
	Long:  `https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-transaction-receipt`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Add optional network flag to specify the network to use
		network := viper.GetString("starknet_network")
		res, err := exec.Command("starknet", "get_transaction", "--hash",
			args[0], "--network", network).CombinedOutput()
		if err != nil {
			log.Default.Error(err)
		}
		fmt.Println(string(res))
	},
}

func init() {
	rootCmd.AddCommand(getTransactionReceiptCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// getTransactionReceiptCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// getTransactionReceiptCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
