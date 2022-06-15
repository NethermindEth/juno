package cli

import (
	"github.com/spf13/cobra"
)

// getBlockIdbyHashCmd represents the getBlockIdbyHash command
var getBlockIdbyHashCmd = &cobra.Command{
	Use:   "get_block_id_by_hash [BLOCK_HASH] [flags]",
	Short: "Get block number (ID) by block hash.",
	Long:  `See https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-block-id-by-hash`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		res, _ := getBlockID(args[0])
		if pretty, _ := cmd.Flags().GetBool("pretty"); pretty {
			prettyPrint(res)
		} else {
			normalReturn(*res)
		}
	},
}

func getBlockID(blockHash string) (*int, error) {
	client := initClient()

	// Get the block ID of the block with the given hash.
	res, _ := client.GetBlockIDByHash(blockHash)
	return res, nil
}

func init() {
	rootCmd.AddCommand(getBlockIdbyHashCmd)
}
