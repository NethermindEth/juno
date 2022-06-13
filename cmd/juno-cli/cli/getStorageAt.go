package cli

import (
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/spf13/cobra"
)

// getStorageAtCmd represents the getStorageAt command
var getStorageAtCmd = &cobra.Command{
	Use:   "get_storage_at CONTRACT_ADDRESS KEY [flags]",
	Short: "Retrieve stored value for a key within a contract.",
	Long:  `See https://starknet.io/docs/hello_starknet/cli.html#get-storage-at`,
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		contractAddress := args[0]
		key := args[1]

		res, _ := GetStorage(contractAddress, key)
		if pretty, _ := cmd.Flags().GetBool("pretty"); pretty {
			prettyPrint(res)
		} else {
			normalReturn(res)
		}
	},
}

func GetStorage(contractAddress, key string) (*feeder.StorageInfo, error) {
	client := initClient()

	// Call to get storage at key
	res, _ := client.GetStorageAt(contractAddress, key, "", "")
	return res, nil
}

func init() {
	rootCmd.AddCommand(getStorageAtCmd)
}
