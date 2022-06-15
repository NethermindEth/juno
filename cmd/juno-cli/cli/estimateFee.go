package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// estimateFeeCmd represents the estimateFee command
var estimateFeeCmd = &cobra.Command{
	Use:   "estimate_fee [CONTRACT_ADDRESS] [ENTRY_POINT_SELECTOR] [CALLDATA] [SIGNATURE]",
	Short: "Estimate the fee of a given transaction before invoking it.",
	Long:  `See https://www.cairo-lang.org/docs/hello_starknet/cli.html#estimate-fee`,
	Args:  cobra.MinimumNArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("estimateFee called")
	},
}

func estimateFee(contractAddress, entryPointSelector, callData, signature string) {
	fmt.Println("WIP: awaiting implementation of entry point (contract function name) hashing")
}

func init() {
	rootCmd.AddCommand(estimateFeeCmd)
}
