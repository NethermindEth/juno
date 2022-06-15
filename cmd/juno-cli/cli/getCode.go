package cli

import (
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/spf13/cobra"
)

// getCodeCmd represents the getCode command
var getCodeCmd = &cobra.Command{
	Use:   "get_code [CONTRACT_ADDRESS] [flags]",
	Short: "Get bytecode of a smart contract",
	Long:  `See https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-code`,
	Run: func(cmd *cobra.Command, args []string) {
		res, _ := getCode(args[0])
		if pretty, _ := cmd.Flags().GetBool("pretty"); pretty {
			prettyPrint(res)
		} else {
			normalReturn(res)
		}
	},
}

func getCode(contractAddress string) (*feeder.CodeInfo, error) {
	client := initClient()

	res, _ := client.GetCode(contractAddress, "", "")
	return res, nil
}

func init() {
	rootCmd.AddCommand(getCodeCmd)
}
