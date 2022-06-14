package cli

import (
	"github.com/spf13/cobra"
)

// getFullContractCmd represents the getFullContract command
var getFullContractCmd = &cobra.Command{
	Use:   "get_full_contract [CONTRACT_ADDRESS] [BLOCK_HASH or BLOCK_NUMBER] [flags]",
	Short: "Prints out full contract class at a specific address.",
	Long:  `https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-full-contract`,
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		res, _ := getFullContract(args[0], args[1])
		if pretty, _ := cmd.Flags().GetBool("pretty"); pretty {
			prettyPrint(res)
		} else {
			normalReturn(res)
		}
	},
}

func getFullContract(contractAddress, blockAddressOrHash string) (map[string]interface{}, error) {
	blockHash, blockNumber := "", ""

	if isInteger(blockAddressOrHash) {
		blockNumber = blockAddressOrHash
	} else {
		blockHash = blockAddressOrHash
	}

	client := initClient()

	// For blocks, block hash and number are possible valid inputs.
	res, _ := client.GetFullContract(contractAddress, blockHash, blockNumber)
	return res, nil
}

func init() {
	rootCmd.AddCommand(getFullContractCmd)
}
