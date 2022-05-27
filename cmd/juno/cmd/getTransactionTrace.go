package cmd

import (
	"fmt"
	"os/exec"

	"github.com/NethermindEth/juno/internal/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// getTransactionTraceCmd represents the get_transaction_trace command
var getTransactionTraceCmd = &cobra.Command{
	Use:   "get_transaction_trace --hash TRANSACTION_HASH [--network NETWORK (WIP)]",
	Short: `Print execution information with caller/callee addresses, selector, calldata along with execution information such as return value, emitted events, and sent messages.`,
	Long:  `https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-transaction-trace`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// network := viper.GetString("starknet_network")
		res, err := exec.Command("starknet", "get_transaction_trace", "--hash",
			args[0], "--network", viper.GetString("starknet_network")).CombinedOutput()
		if err != nil {
			log.Default.Error(err)
		}
		fmt.Println(string(res))
	},
}

func init() {
	rootCmd.AddCommand(getTransactionTraceCmd)
}
