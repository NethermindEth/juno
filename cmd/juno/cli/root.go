package cli

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/NethermindEth/juno/internal/errpkg"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/internal/ospkg"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

// rpcConfig represents the juno RPC configuration.
type rpcConfig struct {
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`
	Port    int  `yaml:"port" mapstructure:"port"`
}

// config represents the juno configuration.
type config struct {
	Rpc    rpcConfig `yaml:"rpc" mapstructure:"rpc"`
	DbPath string    `yaml:"db_path" mapstructure:"db_path"`
}

// Cobra configuration.
var (
	// cfgFile is the path of the juno configuration file.
	cfgFile string
	// longMsg is the long message shown in the "juno --help" output.
	//go:embed long.txt
	longMsg string

	// rootCmd is the root command of the application.
	rootCmd = &cobra.Command{
		Use:   "juno [options]",
		Short: "Starknet client implementation in Go.",
		Long:  longMsg,
		Run:   func(cmd *cobra.Command, args []string) {},
	}
)

// init defines flags and handles configuration.
func init() {
	// Set the functions to be run when rootCmd.Execute() is called.
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", fmt.Sprintf(
		"config file (default is %s)",
		filepath.Join(ospkg.ConfigDir, "juno", "juno.yaml")))
}

// initConfig reads in config file or environment variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file specified by the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(filepath.Join(ospkg.ConfigDir, "juno"))
		viper.SetConfigType("yaml")
		viper.SetConfigName("juno")
	}

	// Check whether the environment variables match any of the existing
	// keys and loads them if they are found.
	viper.AutomaticEnv()

	err := viper.ReadInConfig()
	if err == nil {
		log.Default.With("File", viper.ConfigFileUsed()).Info("Using config file:")
	} else {
		log.Default.Info("Config file not found.")

		// Create default config.
		cfgPath := filepath.Join(ospkg.ConfigDir, "juno", "juno.yaml")
		log.Default.With("Path", cfgPath).Info("Creating default config.")

		// Create the juno configuration directory if it does not exist.
		cfgDirPath := filepath.Join(ospkg.ConfigDir, "juno")
		if _, err := os.Stat(cfgDirPath); os.IsNotExist(err) {
			err := os.MkdirAll(cfgDirPath, 0755)
			errpkg.CheckFatal(err, "Failed to create config directory.")
		}

		data, err := yaml.Marshal(&config{
			Rpc:    rpcConfig{Enabled: true, Port: 8080},
			DbPath: filepath.Join(ospkg.DataDir, "juno"),
		})
		errpkg.CheckFatal(err, "Failed to marshal Config instance to byte data.")
		err = os.WriteFile(cfgPath, data, 0644)
		errpkg.CheckFatal(err, "Failed to write config file.")

		err = viper.ReadInConfig()
		errpkg.CheckFatal(err, "Failed to read in config after generation.")
	}

	// Log configuration values.
	var cfg *config
	err = viper.Unmarshal(&cfg)
	if err != nil {
		log.Default.With("Error", err).Panic("Unable to unmarshal configuration.")
		return
	}
	log.Default.With(
		"Database Path", cfg.DbPath,
		"Rpc Port", cfg.Rpc.Port,
		"Rpc Enabled", cfg.Rpc.Enabled,
	).Info("Configuration values.")
}

// Execute handle flags for Cobra execution.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Default.With("Error", err).Error("Failed to execute CLI.")
	}
}
