package cli

// notest
import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"

	. "github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type Config struct {
	Network   string
	FeederUrl string
}

// Cobra configuration.
var (
	cfg = &Config{}
	// longMsg is the long message shown in the "juno --help" output.
	//go:embed long.txt
	longMsg string

	// rootCmd is the root command of the application.
	rootCmd = &cobra.Command{
		Use:   "juno-cli [command] [flags]",
		Short: "Starknet client implementation in Go.",
		Long:  longMsg,
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			if network, _ := cmd.Flags().GetString("network"); network != "" {
				handleNetwork(network)
			}
		},
	}
)

// Define flags and load config.
func init() {
	// Pretty print flag.
	rootCmd.PersistentFlags().BoolP("pretty", "p", false, "Pretty print the response.")

	// Network flag.
	rootCmd.PersistentFlags().StringVarP(&cfg.Network, "network", "n", "", "the StarkNet network. Options: 'mainnet', 'goerli'.")
	rootCmd.PersistentFlags().StringVarP(&cfg.FeederUrl, "feeder", "f", "", "the URL of the feeder gateway.")
}

// handle networks by changing active value during call only
func handleNetwork(network string) {
	if network == "mainnet" {
		viper.Set("starknet.feeder_gateway", "https://alpha-mainnet.starknet.io")
	}
	if network == "goerli" {
		viper.Set("starknet.feeder_gateway", "http://alpha4.starknet.io")
	}
}

// Pretty Prints response. Use interface to take any type.
func prettyPrint(res interface{}) {
	resJSON, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		Logger.With("Error", err).Error("Failed to marshal and indent response.")
	}
	fmt.Println(string(resJSON))
}

// What to do in normal situations, when no pretty print flag is set.
func normalReturn(res interface{}) {
	resJSON, err := json.Marshal(res)
	if err != nil {
		Logger.With("Error", err).Error("Failed to marshal response.")
	}
	fmt.Println(string(resJSON))
}

// Check if string is integer or hash
func isInteger(input string) bool {
	_, err := strconv.ParseInt(input, 10, 64)
	return err == nil
}

func initClient() *feeder.Client {
	return feeder.NewClient(cfg.FeederUrl, "/feeder_gateway", nil)
}

// Execute handle flags for Cobra execution.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		Logger.Fatal("Failed to execute CLI.")
	}
}
