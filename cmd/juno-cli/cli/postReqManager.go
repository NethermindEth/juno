package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// postReqManagerCmd represents the postReqManager command
var postReqManagerCmd = &cobra.Command{
	Use:   "cx",
	Short: "Router for `estimate_fee` and `call_contract`.",
	Long:  `Routing command for estimate fee and call contract.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Please use either `cx estimate_fee` or `cx call_contract`.")
	},
}

func init() {
	// Add calldata flag
	postReqManagerCmd.PersistentFlags().StringArrayP("calldata", "i", []string{}, "Transaction calldata (function inputs).")

	// Add signature flag
	postReqManagerCmd.PersistentFlags().StringArrayP("signature", "s", []string{}, "Account signature.")

	// Add command
	rootCmd.AddCommand(postReqManagerCmd)
}
