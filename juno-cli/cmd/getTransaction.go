package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Command to get transaction info with hash
var getTransactionCmd = &cobra.Command{
	Use:   "get_transaction TRANSACTION_HASH [--network NETWORK (WIP)]",
	Short: "Prints out transaction information.",
	Long:  `See https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-transaction`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

		res, _ := getTxInfo(args[0], "")
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

func getTxInfo(txHash string, id string) (*feeder.TransactionInfo, error) {
	// Initialise new client
	fmt.Println(config.Runtime.Network)
	feeder_url := viper.GetString("network")
	client := feeder.NewClient(feeder_url, "/feeder_gateway", nil)

	// Call to get transaction info
	res, err := client.GetTransaction(txHash, id)
	if err != nil {
		log.Default.Fatal(err)
		return nil, err
	}
	return res, nil

}

func init() {
	rootCmd.AddCommand(getTransactionCmd)
}
