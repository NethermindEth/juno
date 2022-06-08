package cmd

// notest
import (
	_ "embed"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/errpkg"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/internal/process"
	"github.com/NethermindEth/juno/pkg/rpc"
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

		Run: func(cmd *cobra.Command, args []string) {
			handler := process.NewHandler()

			// Handle signal interrupts and exits.
			sig := make(chan os.Signal)
			signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
			go func() {
				<-sig
				log.Default.Info("Trying to close...")
				handler.Close()
				log.Default.Info("App closing...Bye!!!")
				os.Exit(0)
			}()

			// Subscribe the RPC client to the main loop if it is enabled in
			// the config.
			if config.Runtime.RPC.Enabled {
				s := rpc.NewServer(":" + strconv.Itoa(config.Runtime.RPC.Port))
				handler.Add("RPC", s.ListenAndServe, s.Close)
			}

			// endless running process
			log.Default.Info("Starting all processes...")
			handler.Run()
			handler.Close()
			log.Default.Info("All processes closed.")
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

// handle other networks
// NOTE: That it might be persistent.
func handleNetwork(network string) {
	if network == "mainnet" {
		viper.Set("network", "https://alpha-mainnet.starknet.io")
	}
	if network == "goerli" {
		viper.Set("network", "http://alpha4.starknet.io")
	}
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

	// Print out all of the key value pairs available in viper for debugging purposes.
	for _, key := range viper.AllKeys() {
		log.Default.With("Key", key).With("Value", viper.Get(key)).Info("Config:")
	}

	// Unmarshal and log runtime config instance.
	err = viper.Unmarshal(&config.Runtime)
	errpkg.CheckFatal(err, "Unable to unmarshal runtime config instance.")

	// If config successfully loaded, return no error.
	return nil
}

// Execute handle flags for Cobra execution.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Default.With("Error", err).Error("Failed to execute CLI.")
	}
}
