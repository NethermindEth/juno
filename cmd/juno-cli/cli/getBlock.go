package cli

import (
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/spf13/cobra"
)

// getBlockCmd represents the getBlock command
var getBlockCmd = &cobra.Command{
	Use:   "get_block [BLOCK_HASH or BLOCK_NUMBER] [flags]",
	Short: "Prints out block information.",
	Long:  `https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-block`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		res, _ := getBlockInfo(args[0])
		if pretty, _ := cmd.Flags().GetBool("pretty"); pretty {
			prettyPrint(res)
		} else {
			normalReturn(res)
		}
	},
}

func getBlockInfo(input string) (*feeder.StarknetBlock, error) {
	blockHash, blockNumber := "", ""

	if isInteger(input) {
		blockNumber = input
	} else {
		blockHash = input
	}

	client := initClient()

	// Call to get block info - hash and number possible inputs.
	res, _ := client.GetBlock(blockHash, blockNumber)
	return res, nil
}

func init() {
	rootCmd.AddCommand(getBlockCmd)
}
