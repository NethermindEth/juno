package cli

import (
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/spf13/cobra"
)

// Command to get transaction info with hash
var getTransactionCmd = &cobra.Command{
	Use:   "get_transaction [TRANSACTION_HASH] [flags]",
	Short: "Prints out transaction information.",
	Long:  `See https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-transaction`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		res, _ := getTxInfo(args[0])
		if pretty, _ := cmd.Flags().GetBool("pretty"); pretty {
			prettyPrint(res)
		} else {
			normalReturn(res)
		}
	},
}

func getTxInfo(txHash string) (*feeder.TransactionInfo, error) {
	client := initClient()
	// Call to get transaction info - txID no longer used.
	res, _ := client.GetTransaction(txHash, "")
	return res, nil
}

func init() {
	rootCmd.AddCommand(getTransactionCmd)
}
