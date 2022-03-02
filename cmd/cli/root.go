package cmd

import (
	"fmt"
	"github.com/NethermindEth/juno/configs"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/internal/utils"
	"github.com/NethermindEth/juno/pkg/rpc"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"os/signal"
	"path/filepath"
)

// Logger for the app
var logger = log.GetLogger()

// Represent all the app configuration
var configuration *configs.Configuration
var cfgFile string

// Process handler
var processor utils.Processor

// Base cmd handler
var rootCmd = &cobra.Command{
	Use:   "juno",
	Short: "Juno, Starknet Client in Go",
	Long:  "Juno, StarkNet Client in Go",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(`      _                   
     | |                  
     | |_   _ _ __   ___  
 _   | | | | | '_ \ / _ \ 
| |__| | |_| | | | | (_) |
 \____/ \__,_|_| |_|\___/ 
                          
                          
`)
		fmt.Println(cmd.Short)

		processor = utils.NewProcessor(logger)

		// Handle Ctrl+C for close and close Juno
		sig := make(chan os.Signal)
		signal.Notify(sig, os.Interrupt, os.Kill)
		go func() {
			<-sig
			logger.Info("Trying to close...")
			cleanup()
			logger.Info("App closing...Bye!!!")
			os.Exit(1)
		}()

		// Subscribe RPC to main loop execution only if enable in configs
		if configuration.RpcConfiguration.RpcEnabled {
			s := rpc.NewServer(":8080")
			processor.Add(logger, "RPC", s.ListenAndServe, s.Close)
		}

		// endless running process
		logger.Info("Starting all processes...")
		processor.Run()
		cleanup()
		logger.Info("App closing...Bye!!!")
	},
}

// Clean up and close all running processes
func cleanup() {
	processor.Close()
}

// Initialize cobra and viper for logs and configuration respectively
func init() {
	cobra.OnInitialize(initConfig)
	// Flag to change config file
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"config file (default is $HOME/.juno/config.yaml)")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		if err != nil {
			logger.With("Error", err).Error("Unable to get User Home Directory")
		}

		// Search config in ~/.juno directory with name "config" (without extension).
		viper.AddConfigPath(filepath.Join(home, configs.ProjectFolderName))
		viper.SetConfigType("yaml")
		viper.SetConfigName(configs.ConfigFileNameWithoutExtension)
	}

	// read in environment variables that match
	viper.AutomaticEnv()

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		logger.With("File", viper.ConfigFileUsed()).Info("Using config file:")
		GetConfig()
	} else {
		logger.With("Error", err).Info("Generate file not found on the path provided nor in the home directory")

		// Generate config file
		home, err := os.UserHomeDir()
		if err != nil {
			logger.With("Error", err).Error("Unable to get User Home Directory")
		}
		err = configs.Generate(home)
		if err != nil {
			logger.With("Error", err).Error("Unable to generate config")
		}

		// Read from config file
		err = viper.ReadInConfig()
		if err != nil {
			logger.With("Error", err).Errorf("Unable of read config after generation")
		} else {
			GetConfig()
		}

	}
}

// GetConfig parse and log config to project struct
func GetConfig() {
	// Unmarshal Viper Config to configuration struct
	err := viper.Unmarshal(&configuration)
	if err != nil {
		logger.With("Error", err).Panic("Unable to unmarshal project configuration")
		return
	}
	logger.With("Database Path", configuration.DatabasePath,
		"Rpc Port", configuration.RpcConfiguration.RpcPort,
		"Rpc Enabled", configuration.RpcConfiguration.RpcEnabled).
		Info("Configuration values")

}

// Execute handle flags for Cobra execution
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logger.With("Error", err).Error("Error executing CLI")
		return
	}
}
