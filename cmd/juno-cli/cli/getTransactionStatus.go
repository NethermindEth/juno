package cli

import (
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/spf13/cobra"
)

// getTransactionStatusCmd represents the getTransactionStatus command
var getTransactionStatusCmd = &cobra.Command{
	Use:   "get_transaction_status TRANSACTION_HASH [flags]",
	Short: "Prints out transaction status information.",
	Long:  `See https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-transaction-status`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		res, _ := getTxStatus(args[0])
		if pretty, _ := cmd.Flags().GetBool("pretty"); pretty {
			prettyPrint(res)
		} else {
			normalReturn(res)
		}
	},
}

func getTxStatus(txHash string) (*feeder.TransactionStatus, error) {
	client := initClient()

	// Call to get transaction receipt
	res, _ := client.GetTransactionStatus(txHash, "")
	return res, nil
}

func init() {
	rootCmd.AddCommand(getTransactionStatusCmd)
}
