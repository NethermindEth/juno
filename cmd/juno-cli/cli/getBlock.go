package cli

import (
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

	// Initialise new client
	feederUrl := viper.GetString("network")
	client := feeder.NewClient(feederUrl, "/feeder_gateway", nil)

	// Call to get block info
	res, _ := client.GetBlock(blockHash, blockNumber)
	return res, nil
}

func init() {
	rootCmd.AddCommand(getBlockCmd)
}
