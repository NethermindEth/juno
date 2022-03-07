package cli

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"

	"github.com/NethermindEth/juno/internal/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// General configuration.
var cfg *Config

var (
	// Cobra configuration.
	cfgFile string
	//go:embed long.txt
	doc string

	rootCmd = &cobra.Command{
		Use:   "juno",
		Short: "Starknet client implementation in Go.",
		Long:  doc,
		Run:   func(cmd *cobra.Command, args []string) {},
	}
)

// init defines flags and handles configuration.
func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(
		&cfgFile, "config", "", "config file (default is $HOME/.juno/config.yaml)")
}

// initConfig reads in config file or environment variables if set.
func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Default.With("Error", err).Error("Unable to get User Home Directory")
		}

		// Search config in ~/.juno directory with name "config".
		viper.AddConfigPath(filepath.Join(home, ProjectDir))
		viper.SetConfigType("yaml")
		viper.SetConfigName(strings.TrimSuffix(CfgFileName, filepath.Ext(CfgFileName)))
	}

	// Check whether the environment variables match any of the existing
	// keys and load them if they are found.
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		log.Default.With("File", viper.ConfigFileUsed()).Info("Using config file:")
		unmarshalConfig()
	} else {
		log.Default.With("Error", err).Info("Generate file not found on the path provided nor in the home directory")

		// Generate config file.
		home, err := os.UserHomeDir()
		if err != nil {
			log.Default.With("Error", err).Error("Unable to get User Home Directory")
		}
		err = DefaultConfig(home)
		if err != nil {
			log.Default.With("Error", err).Error("Unable to generate config")
		}

		err = viper.ReadInConfig()
		if err != nil {
			log.Default.With("Error", err).Errorf("Unable of read config after generation")
		} else {
			unmarshalConfig()
		}
	}
}

// unmarshalConfig unmarshals the configuration into cli.cfg.
func unmarshalConfig() {
	err := viper.Unmarshal(&cfg)
	if err != nil {
		log.Default.With("Error", err).Panic("Unable to unmarshal project configuration")
		return
	}
	log.Default.With(
		"Database Path",
		cfg.DbPath,
		"Rpc Port",
		cfg.Rpc.Port,
		"Rpc Enabled",
		cfg.Rpc.Enabled,
	).Info("Configuration values")
}

// Execute handle flags for Cobra execution.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Default.With("Error", err).Error("Error executing CLI")
	}
}
