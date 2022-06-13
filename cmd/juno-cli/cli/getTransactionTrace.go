package cli

import (
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/spf13/cobra"
)

// getTransactionTraceCmd represents the get_transaction_trace command
var getTransactionTraceCmd = &cobra.Command{
	Use:   "get_transaction_trace TRANSACTION_HASH [flags]",
	Short: `Print internal transaction information.`,
	Long:  `https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-transaction-trace`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		res, _ := getTxTrace(args[0])
		if pretty, _ := cmd.Flags().GetBool("pretty"); pretty {
			prettyPrint(res)
		} else {
			normalReturn(res)
		}
	},
}

func getTxTrace(txHash string) (*feeder.TransactionTrace, error) {
	client := initClient()

	// Call to get transaction info, txID no longer used.
	res, _ := client.GetTransactionTrace(txHash, "")
	return res, nil
}

func init() {
	rootCmd.AddCommand(getTransactionTraceCmd)
}
