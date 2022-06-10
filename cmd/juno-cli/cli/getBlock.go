package cli

import (
	"github.com/NethermindEth/juno/cmd/juno-cli"
	"github.com/NethermindEth/juno/internal/errpkg"
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
			main.prettyPrint(res)
		} else {
			main.normalReturn(res)
		}
	},
}

func getBlockInfo(input string) (*feeder.StarknetBlock, error) {
	blockHash := ""
	blockNumber := ""

	if main.isInteger(input) {
		blockNumber = input
	} else {
		blockHash = input
	}

	// Initialise new client
	feeder_url := viper.GetString("network")
	client := feeder.NewClient(feeder_url, "/feeder_gateway", nil)

	// Call to get block info
	res, err := client.GetBlock(blockHash, blockNumber)
	errpkg.CheckFatal(err, "Error getting block info")
	return res, nil
}

func init() {
	main.rootCmd.AddCommand(getBlockCmd)
}
