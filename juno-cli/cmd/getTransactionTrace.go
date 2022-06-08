package cmd

import (
	"encoding/json"
	"fmt"

	// "github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// getTransactionTraceCmd represents the get_transaction_trace command
var getTransactionTraceCmd = &cobra.Command{
	Use:   "get_transaction_trace --hash TRANSACTION_HASH [--network NETWORK (WIP)]",
	Short: `Print internal transaction information.`,
	Long:  `https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-transaction-trace`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		res, _ := getTxTrace(args[0], "")
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

func getTxTrace(txHash string, id string) (*feeder.TransactionTrace, error) {
	// Initialise new client
	feeder_url := viper.GetString("network")
	client := feeder.NewClient(feeder_url, "/feeder_gateway", nil)

	// Call to get transaction info
	res, err := client.GetTransactionTrace(txHash, id)
	if err != nil {
		log.Default.Fatal(err)
		return nil, err
	}
	return res, nil

}

func init() {
	rootCmd.AddCommand(getTransactionTraceCmd)
}
