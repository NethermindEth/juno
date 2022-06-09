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

// getCode = &cobra.Command{ // get_code CLI command
// 	Use: "juno get_code --contract_address CONTRACT_ADDRESS",
// 	Short: `Once the deploy transaction is accepted on-chain, you will be able to see the code of the contract you have just deployed.
// 			The output consists of a list of bytecodes, rather than the source code. This is because the StarkNet network gets the contract after compilation.`,
// 	Long: `https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-code`,
// 	Args: cobra.MinimumNArgs(1),
// 	Run: func(cmd *cobra.Command, args []string) {
// 		fmt.Println("Print: " + strings.Join(args, " "))
// 	},
// }

// getFullContract = &cobra.Command{ // get_full_contract CLI command
// 	Use:   "juno get_full_contract --contract_address CONTRACT_ADDRESS",
// 	Short: `To get the full contract definition of a contract at a specific address`,
// 	Long:  `https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-full-contract`,
// 	Args:  cobra.MinimumNArgs(1),
// 	Run: func(cmd *cobra.Command, args []string) {
// 		fmt.Println("Print: " + strings.Join(args, " "))
// 	},
// }

// getBlock = &cobra.Command{ // get_block CLI command
// 	Use:   "juno get_block --number BLOCK_NUMBER",
// 	Short: `Instead of querying a specific contract or transaction, you may want to query an entire block and examine the transactions contained within it.`,
// 	Long:  `https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-block`,
// 	Args:  cobra.MinimumNArgs(1),
// 	Run: func(cmd *cobra.Command, args []string) {
// 		fmt.Println("Print: " + strings.Join(args, " "))
// 	},
// }

// getStateUpdate = &cobra.Command{ // get_state_update CLI command
// 	Use:   "juno get_state_update --block_number [BLOCK_NUMBER]",
// 	Short: `You can use the following command to get the state changes in a specific block (for example, what storage cells have changed).`,
// 	Long:  `https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-state-update`,
// 	Args:  cobra.MinimumNArgs(1),
// 	Run: func(cmd *cobra.Command, args []string) {
// 		fmt.Println("Print: " + strings.Join(args, " "))
// 	},
// }

// //getStorageAt function is more complicated, don't think this is right as it requires running a python code to get balance key first
// getStorageAt = &cobra.Command{ // get_storage_at CLI command
// 	Use: "juno get_storage_at",
// 	Short: `Other than querying the contract’s code, you may also want to query the contract’s storage at a specific key.
// 			To do so, you first need to understand which key is of interest to you. As you saw before, StarkNet introduces a new primitive, which is storage variables.
// 			Each storage variable is mapped to a storage key (a field element).`,
// 	Long: `https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-storage-at`,
// 	Args: cobra.MinimumNArgs(1),
// 	Run: func(cmd *cobra.Command, args []string) {
// 		fmt.Println("Print: " + strings.Join(args, " "))
// 	},
// }
