package cli

import (
	"github.com/spf13/cobra"
)

// getTransactionHashByIDCmd represents the getTransactionHashByID command
var getTransactionHashByIDCmd = &cobra.Command{
	Use:   "get_transaction_hash_by_id [TRANSACTION_ID] [flags]",
	Short: "Get transaction hash from transaction ID.",
	Long:  `See https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-transaction-hash-by-id`,
	Run: func(cmd *cobra.Command, args []string) {
		res, _ := getTransactionHash(args[0])
		if pretty, _ := cmd.Flags().GetBool("pretty"); pretty {
			prettyPrint(res)
		} else {
			normalReturn(*res)
		}
	},
}

func getTransactionHash(transactionID string) (*string, error) {
	client := initClient()

	// Get the transaction hash of the transaction with the given ID.
	res, _ := client.GetTransactionHashByID(transactionID)
	return res, nil
}

func init() {
	rootCmd.AddCommand(getTransactionHashByIDCmd)
}
