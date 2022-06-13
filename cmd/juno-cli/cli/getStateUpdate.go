package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// getStateUpdateCmd represents the getStateUpdate command
var getStateUpdateCmd = &cobra.Command{
	Use:   "getStateUpdate",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("getStateUpdate called")
	},
}

func init() {
	rootCmd.AddCommand(getStateUpdateCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// getStateUpdateCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// getStateUpdateCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
