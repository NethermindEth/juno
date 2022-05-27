package cmd

import (
	"fmt"

	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// getTransactionCmd represents the getTransaction command
var getTransactionCmd = &cobra.Command{
	Use:   "get_transaction TRANSACTION_HASH [--network NETWORK (WIP)]",
	Short: "Prints out transaction information.",
	Long:  `See https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-transaction`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Add optional network flag to specify the network to use
		// Transaction network should be set from config file?

		// TODO: feeder.NewClient()

		// res, err := exec.Command("starknet", "get_transaction", "--hash",
		// 	args[0], "--network", viper.GetString("starknet_network")).CombinedOutput()
		// if err != nil {
		// 	log.Default.Error(err)
		// }
		// fmt.Println(string(res))
	},
}

func init() {
	rootCmd.AddCommand(getTransactionCmd)
}
