package cmd

import (
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Command to get transaction info with hash
var getTransactionCmd = &cobra.Command{
	Use:   "get_transaction [TRANSACTION_HASH or TRANSACTION_NUMBER] [flags]",
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

func getTxInfo(input string) (*feeder.TransactionInfo, error) {
	// TODO: Make an overarching function for transactions that takes all inputs?
	txHash := ""
	txID := ""

	if isInteger(input) {
		txID = input
	} else {
		txHash = input
	}

	// Initialise new client
	feeder_url := viper.GetString("network")
	client := feeder.NewClient(feeder_url, "/feeder_gateway", nil)

	// Call to get transaction info
	res, _ := client.GetTransaction(txHash, txID)
	return res, nil

}

func init() {
	rootCmd.AddCommand(getTransactionCmd)
}
