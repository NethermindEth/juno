package cli

import (
	"github.com/spf13/cobra"
)

// getBlockHashByIdCmd represents the getBlockHashById command
var getBlockHashByIdCmd = &cobra.Command{
	Use:   "get_block_hash_by_id [BLOCK_NUMBER] [flags]",
	Short: "Get corresponding block hash by block number (ID).",
	Long:  `See https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-block-hash-by-id`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		res, _ := getBlockHash(args[0])
		if pretty, _ := cmd.Flags().GetBool("pretty"); pretty {
			prettyPrint(res)
		} else {
			normalReturn(*res)
		}
	},
}

func getBlockHash(blockNumber string) (*string, error) {
	client := initClient()

	// Get the block hash of the block with the given number.
	res, _ := client.GetBlockHashById(blockNumber)
	return res, nil
}

func init() {
	rootCmd.AddCommand(getBlockHashByIdCmd)
}
