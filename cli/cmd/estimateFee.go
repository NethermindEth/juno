package cmd

import (
	"fmt"
	"os/exec"

	"github.com/NethermindEth/juno/internal/log"
	"github.com/spf13/cobra"
	_ "github.com/spf13/viper"
)

/*
Example values
IN GOERLI TESTNET
network = alpha-goerli
https://goerli.voyager.online/contract/0x044a68c9052a5208a46aee5d0af6f6a3e30686ab9ce3e852c4b817d0a76f2f09#writeContract
contract_add = 0x044a68c9052a5208a46aee5d0af6f6a3e30686ab9ce3e852c4b817d0a76f2f09
contract_abi =
function =
inputs =

*/

// estimateFeeCmd represents the estimate_fee command
var estimateFeeCmd = &cobra.Command{
	Use: `estimate_fee \
	   --address CONTRACT_ADDRESS \
	   --abi contract_abi.json \
	   --function increase_balance \
	   --inputs 1234`,
	Short: `Estimate the fee of a transaction without invoking it.`,
	Long:  `https://www.cairo-lang.org/docs/hello_starknet/cli.html#estimate-fee`,
	Args:  cobra.ExactArgs(0), // Take zero args, all flags

	Run: func(cmd *cobra.Command, args []string) {
		// Read in values from flags and ensure they are not empty.
		address, _ := cmd.Flags().GetString("contract_address")
		abi, _ := cmd.Flags().GetString("contract_abi")
		function, _ := cmd.Flags().GetString("function")
		inputs, _ := cmd.Flags().GetString("inputs")

		if address == "" || abi == "" || function == "" || inputs == "" {
			log.Default.Error("Please provide all required flags")
			return
		}

		res, err := exec.Command("starknet", "estimate_fee", "--address",
			address, "--abi", abi, "--function", function, "--inputs", inputs).CombinedOutput()
		if err != nil {
			log.Default.Error(err)
		}
		fmt.Println(string(res))
	},
}

func init() {
	rootCmd.AddCommand(estimateFeeCmd)

	// Required flags for estimate_fee
	estimateFeeCmd.Flags().StringP("contract_address", "c", "", "Contract address.")
	estimateFeeCmd.MarkFlagRequired("contract_address")

	estimateFeeCmd.Flags().StringP("contract_abi", "a", "", "Contract ABI.")
	estimateFeeCmd.MarkFlagRequired("contract_abi")

	estimateFeeCmd.Flags().StringP("function", "f", "", "Function to call inside the contract.")
	estimateFeeCmd.MarkFlagRequired("function")

	estimateFeeCmd.Flags().StringP("inputs", "i", "", "Inputs to the function.")
	estimateFeeCmd.MarkFlagRequired("inputs")

}
