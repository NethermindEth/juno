package cli

import (
	"github.com/NethermindEth/juno/pkg/feeder"

	"github.com/spf13/cobra"
)

// estimateFeeCmd represents the estimateFee command
var estimateFeeCmd = &cobra.Command{
	Use:   "estimate_fee CONTRACT_HASH FUNCTION_NAME [--calldata] [--signature] [flags]",
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

func estimateFee(contractAddress string, entryPointSelector string, callData string, signature string) (*feeder.EstimateFeeResponse, error) {
	client := initClient()

	// Call to get estimate transaction fee for given contract and params.
	res, _ := client.EstimateTransactionFee(contractAddress, entryPointSelector, callData, signature)
	return res, nil
}

func init() {
	postReqManagerCmd.AddCommand(estimateFeeCmd)
}
