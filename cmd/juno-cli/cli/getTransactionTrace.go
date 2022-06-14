package cli

import (
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/spf13/cobra"
)

// getTransactionTraceCmd represents the getTransactionTrace command
var getTransactionTraceCmd = &cobra.Command{
	Use:   "get_transaction_trace [TRANSACTION_HASH] [flags]",
	Short: "Information containing inner calls for an external transaction, in chronological order.",
	Long:  `See https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-transaction-trace`,
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
	// Call to get tx trace - txID not used.
	res, _ := client.GetTransactionTrace(txHash, "")
	return res, nil
}

func init() {
	rootCmd.AddCommand(getTransactionTraceCmd)
}
