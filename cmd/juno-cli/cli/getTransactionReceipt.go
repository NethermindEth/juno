package cli

import (
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/spf13/cobra"
)

var getTransactionReceiptCmd = &cobra.Command{ // Get_Transaction Receipt CLI command
	Use:   "get_transaction_receipt TRANSACTION_HASH [flags]",
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

func getTxReceipt(txHash string) (*feeder.TransactionReceipt, error) {
	client := initClient()

	// Call to get transaction receipt - txID no longer used.
	res, _ := client.GetTransactionReceipt(txHash, "")
	return res, nil
}

func init() {
	rootCmd.AddCommand(getTransactionReceiptCmd)
}
