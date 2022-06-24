package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// postReqManagerCmd represents the postReqManager command
var postReqManagerCmd = &cobra.Command{
	Use:   "cx",
	Short: "Routing for post requests.",
	Long:  `Routing command for estimate fee and call contract.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("postReqManager called")
	},
}

func init() {
	// Add calldata flag
	postReqManagerCmd.PersistentFlags().StringP("calldata", "i", "0", "Transaction calldata (function inputs).")

	// Add signature flag
	postReqManagerCmd.PersistentFlags().StringP("signature", "s", "0", "Account signature.")

	// Add command
	rootCmd.AddCommand(postReqManagerCmd)
}
