package cli

// notest
import (
	_ "embed"
	"fmt"
	"github.com/NethermindEth/juno/internal/process"
	"github.com/NethermindEth/juno/pkg/rpc"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/NethermindEth/juno/internal/errpkg"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/internal/ospkg"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// rpcConfig represents the juno RPC configuration.
type rpcConfig struct {
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`
	Port    int  `yaml:"port" mapstructure:"port"`
}

// Config represents the juno configuration.
type Config struct {
	Rpc    rpcConfig `yaml:"rpc" mapstructure:"rpc"`
	DbPath string    `yaml:"db_path" mapstructure:"db_path"`
}

// Cobra configuration.
var (
	// General configuration.
	cfg *Config
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
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(longMsg)

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

			// Subscribe the RPC client to the main loop if it is enable in
			// the Config.
			if cfg.Rpc.Enabled {
				s := rpc.NewServer(":8080")
				handler.Add("RPC", s.ListenAndServe, s.Close)
			}

			// endless running process
			log.Default.Info("Starting all processes...")
			handler.Run()
			handler.Close()
			log.Default.Info("App closing...Bye!!!")
		},
	}
)

// init defines flags and handles configuration.
func init() {
	// Set the functions to be run when rootCmd.Execute() is called.
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "Config", "", fmt.Sprintf(
		"Config file (default is %s)",
		filepath.Join(ospkg.ConfigDir, "juno", "juno.yaml")))
}

// initConfig reads in Config file or environment variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use Config file specified by the flag.
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
		log.Default.With("File", viper.ConfigFileUsed()).Info("Using Config file:")
	} else {
		log.Default.Info("Config file not found.")

		GenerateConfig()

		err = viper.ReadInConfig()
		errpkg.CheckFatal(err, "Failed to read in Config after generation.")
	}

	// Log configuration values.
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
