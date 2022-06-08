package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var getTransactionReceiptCmd = &cobra.Command{ // Get_Transaction Receipt CLI command
	Use:   "get_transaction_receipt TRANSACTION_HASH [--network NETWORK (WIP)]",
	Short: "Prints out transaction receipt information.",
	Long:  `https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-transaction-receipt`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		res, _ := getTxReceipt(args[0], "")
		if pretty, _ := cmd.Flags().GetBool("pretty"); pretty {
			// Pretty prints through json.MarshalIndent
			resJSON, err := json.MarshalIndent(res, "", "  ")
			if err != nil {
				log.Default.Fatal(err)
			}
			fmt.Println(string(resJSON))
		} else {
			fmt.Println(res)
		}
	},
}

func getTxReceipt(txHash, txID string) (*feeder.TransactionReceipt, error) {
	//Initialize the client
	feeder_url := viper.GetString("network")
	client := feeder.NewClient(feeder_url, "/feeder_gateway", nil)

	// Call to get transaction receipt
	res, err := client.GetTransactionReceipt(txHash, txID)
	if err != nil {
		log.Default.Fatal(err)
		return nil, err
	}
	return res, nil
}

func init() {
	rootCmd.AddCommand(getTransactionReceiptCmd)
}
