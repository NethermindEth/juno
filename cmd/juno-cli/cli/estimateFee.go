package cli

import (
	"github.com/NethermindEth/juno/pkg/feeder"

	"github.com/spf13/cobra"
)

// estimateFeeCmd represents the estimateFee command
var estimateFeeCmd = &cobra.Command{
	Use:   "estimate_fee CONTRACT_HASH FUNCTION_NAME [--calldata] [--signature] [flags]",
	Short: "Calculate transaction fee for calling a function.",
	Long: `See https://www.cairo-lang.org/docs/hello_starknet/cli.html#estimate-fee.

Please note that to estimate fee for functions with multiple inputs, you should use multiple -i flags with values, 
in the order of the necessary inputs. The network response will tell you that the number of inputs is wrong otherwise.

Example with multiple inputs and expected response being returned:

Input:
  juno-cli cx estimate_fee 0x0003a4d1be6ae6cccc7b6cc3cc16bbfd092c2a724ffe6be86c7b5b5fe6ec11d0 increase_decrease --network goerli -p -i 1 -i 2

Response:
  {
    "amount": 2455450017189,
    "unit": "wei"
  } 


`,
	Args: cobra.MinimumNArgs(2),
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
	Example: "juno-cli cx estimate_fee '0x0003a4d1be6ae6cccc7b6cc3cc16bbfd092c2a724ffe6be86c7b5b5fe6ec11d0' increase_balance --network goerli -p -i 1",
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
