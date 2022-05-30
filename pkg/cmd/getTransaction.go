package cmd

import (
	"fmt"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/spf13/cobra"
)

// getTransactionCmd represents the getTransaction command
var getTransactionCmd = &cobra.Command{
	Use:   "get_transaction TRANSACTION_HASH [--network NETWORK (WIP)]",
	Short: "Prints out transaction information.",
	Long:  `See https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-transaction`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		res, _ := getTxInfo(args[0])
		fmt.Println(res)
	},
}

// Points to feeder.Client?
var client *feeder.HttpClient

func getTxInfo(txh string) (*feeder.TransactionInfo, error) {
	// var p feeder.Client = feeder.HttpClient

	client := feeder.NewClient("https://alpha-mainnet.starknet.io", "/feeder_gateway", nil)
	res, err := client.GetTransaction("", "415")
	if err != nil {
		log.Default.Fatal(err)
		return nil, err
	}
	return res, nil
}

func init() {
	rootCmd.AddCommand(getTransactionCmd)
}
