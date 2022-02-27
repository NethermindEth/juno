package cmd

import (
	"github.com/spf13/cobra"
)

var (
	contractAddress string
	contractAbi     string
	functionName    string
	arguments       []string
	blockHash       string
	blockNumber     string
)

var cmdCall = &cobra.Command{
	Use:   "call \n  --address <contract_address>\n  --abi <contract_abi>\n  --function <function_name>\n  --inputs <arguments>\n  --block_hash <block_hash>\n  --block_number <block_number>",
	Short: "Calls a StarkNet contract without affecting the state",
	Long:  "Calls a StarkNet contract without affecting the state",
	Args:  cobra.RangeArgs(4, 6),
	Run: func(cmd *cobra.Command, args []string) {
		logger.Info("Running `cli call` using Juno")
	},
}

func init() {
	cmdCall.PersistentFlags().StringVar(&contractAddress, "address", "", "address of the called contract")
	cmdCall.PersistentFlags().StringVar(&contractAbi, "abi", "", "path to a JSON file containing the called contract’s abi")
	cmdCall.PersistentFlags().StringVar(&functionName, "function", "", "name of the function which is called")
	cmdCall.PersistentFlags().StringArrayVar(&arguments, "inputs", []string{}, "inputs to the called function, represented by a list of space-delimited values")
	cmdCall.Flags().StringVar(&blockHash, "block_hash", "", "the hash of the block used as the context for the call operation. If this argument is omitted, the latest block is used")
	cmdCall.Flags().StringVar(&blockNumber, "block_number", "", "same as block_hash, but specifies the context block by number")
}

// StarkNetCall calls a StarkNet contract without affecting the state
func StarkNetCall() error {
	return nil
}
