package cmd

import (
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var getTransactionReceiptCmd = &cobra.Command{ // Get_Transaction Receipt CLI command
	Use:   "get_transaction_receipt [TRANSACTION_HASH or TRANSACTION_ID] [--network NETWORK (WIP)]",
	Short: "Prints out transaction receipt information.",
	Long:  `https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-transaction-receipt`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		res, _ := getTxReceipt(args[0])
		if pretty, _ := cmd.Flags().GetBool("pretty"); pretty {
			prettyPrint(res)
		} else {
			normalReturn(res)
		}
	},
}

func getTxReceipt(input string) (*feeder.TransactionReceipt, error) {
	txHash := ""
	txID := ""

	if isInteger(input) {
		txID = input
	} else {
		txHash = input
	}

	//Initialize the client
	feeder_url := viper.GetString("network")
	client := feeder.NewClient(feeder_url, "/feeder_gateway", nil)

	// Call to get transaction receipt
	res, _ := client.GetTransactionReceipt(txHash, txID)
	return res, nil
}

func init() {
	rootCmd.AddCommand(getTransactionReceiptCmd)
}
