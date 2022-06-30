package cli

import (
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/spf13/cobra"
)

// getStateUpdateCmd represents the getStateUpdate command
var getStateUpdateCmd = &cobra.Command{
	Use:   "get_state_update [BLOCK_HASH or BLOCK_NUMBER] [flags]",
	Short: "Get state changes made by a specific block.",
	Long:  `See https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-state-update`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		res, _ := getStateUpdate(args[0])
		if pretty, _ := cmd.Flags().GetBool("pretty"); pretty {
			prettyPrint(res)
		} else {
			normalReturn(*res)
		}
	},
}

func getStateUpdate(input string) (*feeder.StateUpdateResponse, error) {
	blockHash, blockNumber := "", ""

	if isInteger(input) || input == "latest" {
		blockNumber = input
	} else {
		blockHash = input
	}

	client := initClient()

	// Get the state update of the block with the given number.
	res, _ := client.GetStateUpdate(blockHash, blockNumber)
	return res, nil
}

func init() {
	rootCmd.AddCommand(getStateUpdateCmd)
}
