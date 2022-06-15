package cli

import (
	"github.com/spf13/cobra"
)

// getTransactionIDByHashCmd represents the getTransactionIDByHash command
var getTransactionIDByHashCmd = &cobra.Command{
	Use:   "get_transaction_id_by_hash [TRANSACTION_HASH] [flags]",
	Short: "Get transaction ID by transaction hash.",
	Long:  `See https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-transaction-id-by-hash`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		res, _ := getTransactionID(args[0])
		if pretty, _ := cmd.Flags().GetBool("pretty"); pretty {
			prettyPrint(res)
		} else {
			normalReturn(res)
		}
	},
}

func getTransactionID(transactionHash string) (interface{}, error) {
	client := initClient()

	// Get the transaction ID of the transaction with the given hash.
	res, _ := client.GetTransactionIDByHash(transactionHash)
	return res, nil
}

func init() {
	rootCmd.AddCommand(getTransactionIDByHashCmd)
}
