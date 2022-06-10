package cli

import (
	"fmt"

	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// getTransactionIDByHashCmd represents the getTransactionIDByHash command
var getTransactionIDByHashCmd = &cobra.Command{
	Use:   "get_transaction_id_by_hash [TRANSACTION_HASH] [flags]",
	Short: "Get transaction ID by hash.",
	Long:  `See https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-transaction-id-by-hash`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		res, _ := getTxIDByHash(args[0])
		fmt.Println(res)
	},
}

func getTxIDByHash(txHash string) (string, error) {
	fmt.Println("Please note that Transaction IDs are no longer used in StarkNet. Use Hash instead.")
	// Initialise new client
	feeder_url := viper.GetString("network")
	client := feeder.NewClient(feeder_url, "/feeder_gateway", nil)

	// Call to get transaction info
	res, err := client.GetTransactionIDByHash(txHash)
	if err != nil {
		log.Default.With("err", "Hash not found").Error("Error getting transaction ID. Hash not found or invalid.")
	}
	return *res, nil
}

func init() {
	rootCmd.AddCommand(getTransactionIDByHashCmd)
}
