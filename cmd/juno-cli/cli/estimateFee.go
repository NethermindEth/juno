package cli

import (
	"fmt"

	"github.com/NethermindEth/juno/pkg/crypto/keccak"
	"github.com/NethermindEth/juno/pkg/feeder"

	"github.com/spf13/cobra"
)

// estimateFeeCmd represents the estimateFee command
var estimateFeeCmd = &cobra.Command{
	Use:   "estimate_fee CONTRACT_HASH FUNCTION_NAME INPUTS [SIGNATURE] [flags]",
	Short: "Calculate transaction fee for calling a function.",
	Long:  `See https://www.cairo-lang.org/docs/hello_starknet/cli.html#estimate-fee`,
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		selector_hash, _ := getSelectorFromName(args[1])
		res, _ := estimateFee(args[0], selector_hash, cmd.Flag("calldata").Value.String(), cmd.Flag("signature").Value.String())

		// Pretty print or not
		if cmd.Flag("pretty").Value.String() == "true" {
			prettyPrint(res)
		} else {
			normalReturn(res)
		}
	},
}

func getSelectorFromName(func_name string) (string, error) {
	// Convert function name to ASCII (bytes)
	// Then convert to keccak hash
	// Then convert to hex string
	bigIntHash := fmt.Sprintf("%x\n", keccak.Digest250([]byte(func_name)))

	return "0x" + bigIntHash, nil
}

func estimateFee(contractAddress, entryPointSelector, callData, signature string) (*feeder.EstimateFeeResponse, error) {
	client := initClient()

	// Call to get estimate transaction fee for given contract and params.
	res, _ := client.EstimateTransactionFee(contractAddress, entryPointSelector, callData, signature)
	return res, nil
}

func init() {
	rootCmd.AddCommand(estimateFeeCmd)

	// Add calldata flag
	estimateFeeCmd.Flags().StringP("calldata", "i", "0", "Transaction calldata.")

	// Add signature flag
	estimateFeeCmd.Flags().StringP("signature", "s", "0", "Account signature.")
}
