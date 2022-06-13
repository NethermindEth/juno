package cli

// notest
import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/errpkg"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Cobra configuration.
var (
	// cfgFile is the path of the juno configuration file.
	cfgFile string
	// dataDir is the path of the directory to read and save user-specific
	// application data
	dataDir string
	// longMsg is the long message shown in the "juno --help" output.
	//go:embed long.txt
	longMsg string
	// selectedNetwork is the network selected by the config or user.
	selectedNetwork string

	// rootCmd is the root command of the application.
	rootCmd = &cobra.Command{
		Use:   "juno-cli [command] [flags]",
		Short: "Starknet client implementation in Go.",
		Long:  longMsg,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if network, _ := cmd.Flags().GetString("network"); network != "" {
				handleNetwork(network)
			}
			return initConfig()
		},
	}
)

// Define flags and load config.
func init() {
	// Set flags shared accross commands as persistent flags.
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", fmt.Sprintf(
		"config file (default is %s).", filepath.Join(config.Dir, "juno.yaml")))

	// Pretty print flag.
	rootCmd.PersistentFlags().BoolP("pretty", "p", false, "Pretty print the response.")

	// Network flag.
	rootCmd.PersistentFlags().StringVarP(&selectedNetwork, "network", "n", "", "Use a network different to config. Available: 'mainnet', 'goerli'.")
}

// handle networks by changing active value
// FIXME: DO NOT hardcode here. Have in config.go
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
	errpkg.CheckFatal(err, "Failed to marshal response.")
	fmt.Println(string(resJSON))
}

// What to do in normal situations, when no pretty print flag is set.
func normalReturn(res interface{}) {
	fmt.Println(res)
}

// Check if string is integer or hash
func isInteger(input string) bool {
	_, err := strconv.ParseInt(input, 10, 64)
	return err == nil
}

// initConfig reads in Config file or environment variables if set.
func initConfig() error {
	if dataDir != "" {
		info, err := os.Stat(dataDir)
		if err != nil || !info.IsDir() {
			log.Default.Info("Invalid data directory. The default data directory will be used")
			dataDir = config.DataDir
		}
	}
	if cfgFile != "" {
		// If a specific config file is given, read it in.
		viper.SetConfigFile(cfgFile)
	} else {
		// Use the default path for user configuration.
		viper.AddConfigPath(config.Dir)
		viper.SetConfigName("juno")
		viper.SetConfigType("yaml")
	}

	// Fetch other configs from the environment.
	viper.AutomaticEnv()

	err := viper.ReadInConfig()
	if err != nil {
		log.Default.Info("Config file not found.")
		if !config.Exists() {
			config.New()
		}
		viper.SetConfigFile(filepath.Join(config.Dir, "juno.yaml"))
		err = viper.ReadInConfig()
		errpkg.CheckFatal(err, "Failed to read in Config after generation.")
	}

	// // Print out all of the key value pairs available in viper for debugging purposes.
	// for _, key := range viper.AllKeys() {
	// 	log.Default.With("Key", key).With("Value", viper.Get(key)).Info("Config:")
	// }

	// Unmarshal and log runtime config instance.
	err = viper.Unmarshal(&config.Runtime)
	errpkg.CheckFatal(err, "Unable to unmarshal runtime config instance.")

	// If config successfully loaded, return no error.
	return nil
}

func initClient() *feeder.Client {
	// TODO: Use Config instead of viper here.
	feederUrl := viper.GetString("starknet.feeder_gateway")
	client := feeder.NewClient(feederUrl, "/feeder_gateway", nil)
	return client
}

// Execute handle flags for Cobra execution.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Default.With("Error", err).Error("Failed to execute CLI.")
	}
}
