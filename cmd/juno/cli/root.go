package cli

// notest
import (
	_ "embed"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/errpkg"
	. "github.com/NethermindEth/juno/internal/log"
	metric "github.com/NethermindEth/juno/internal/metrics/prometheus"
	"github.com/NethermindEth/juno/internal/process"
	"github.com/NethermindEth/juno/internal/services"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/NethermindEth/juno/pkg/rest"
	"github.com/NethermindEth/juno/pkg/rpc"
	"github.com/NethermindEth/juno/pkg/starknet"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
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

	processHandler *process.Handler
	// rootCmd is the root command of the application.
	rootCmd = &cobra.Command{
		Use:   "juno [options]",
		Short: "Starknet client implementation in Go.",
		Run: func(cmd *cobra.Command, _ []string) {
			processHandler = process.NewHandler()

			// Handle signal interrupts and exits.
			sig := make(chan os.Signal, 1)
			signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
			go func(sig chan os.Signal) {
				<-sig
				Logger.Info("Trying to close...")
				cleanup()
				os.Exit(0)
			}(sig)

			// Check if the user has provided any value for configurations
			// The order of preference
			// 1. Values provided with flags
			// 2. Values provided using environment variables
			// 3. Values from config file (the default)
			// Note that the config file isn't modified by these flags.
			// The values provided to the flags or environment variables are
			// used for that particular run only
			// To update the config file, the user needs to make the
			// -U flag true

			flag, _ := cmd.PersistentFlags().GetString("rpcenable")
			handleConfig("rpc.enabled", flag, "RPCENABLE", 0)
			flag, _ = cmd.PersistentFlags().GetString("rpcport")
			handleConfig("rpc.port", flag, "RPCPORT", 1)

			flag, _ = cmd.PersistentFlags().GetString("metricsenable")
			handleConfig("metrics.enabled", flag, "METRICSENABLE", 0)
			flag, _ = cmd.PersistentFlags().GetString("metricsport")
			handleConfig("metrics.port", flag, "METRICSPORT", 1)

			flag, _ = cmd.PersistentFlags().GetString("dbpath")
			handleConfig("db_path", flag, "DBPATH", 2)

			flag, _ = cmd.PersistentFlags().GetString("starknetenable")
			handleConfig("starknet.enabled", flag, "STARKNETENABLE", 0)
			flag, _ = cmd.PersistentFlags().GetString("apisync")
			handleConfig("starknet.api_sync", flag, "APISYNC", 0)
			flag, _ = cmd.PersistentFlags().GetString("feedergateway")
			handleConfig("starknet.feeder_gateway", flag, "FEEDERGATEWAY", 2)

			flag, _ = cmd.PersistentFlags().GetString("ethereumnode")
			handleConfig("ethereum.node", flag, "ETHEREUMNODE", 2)

			flag, _ = cmd.PersistentFlags().GetString("restenable")
			handleConfig("rest.enabled", flag, "RESTENABLE", 0)
			flag, _ = cmd.PersistentFlags().GetString("restport")
			handleConfig("rest.port", flag, "RESTPORT", 1)
			flag, _ = cmd.PersistentFlags().GetString("restprefix")
			handleConfig("rest.prefix", flag, "RESTPREFIX", 2)

			erru := viper.Unmarshal(&config.Runtime)
			errpkg.CheckFatal(erru, "Unable to unmarshal runtime config instance.")

			flag, _ = cmd.PersistentFlags().GetString("updateconfigfile")

			if flag != "" {
				updateConfigFile(cfgFile)
			}
			// Running the app
			feederGatewayClient := feeder.NewClient(config.Runtime.Starknet.FeederGateway, "/feeder_gateway", nil)
			// Subscribe the RPC client to the main loop if it is enabled in
			// the config.
			if config.Runtime.RPC.Enabled {
				s := rpc.NewServer(":"+strconv.Itoa(config.Runtime.RPC.Port), feederGatewayClient)
				// Initialize the RPC Service.
				processHandler.Add("RPC", true, s.ListenAndServe, s.Close)
			}

			if config.Runtime.Metrics.Enabled {
				s := metric.SetupMetric(":" + strconv.Itoa(config.Runtime.Metrics.Port))
				// Initialize the Metrics Service.
				processHandler.Add("Metrics", false, s.ListenAndServe, s.Close)
			}

			if err := db.InitializeMDBXEnv(config.Runtime.DbPath, 100, 0); err != nil {
				Logger.With("Error", err).Fatal("Error starting the database environment")
			}

			// Initialize ABI Service
			processHandler.Add("ABI Service", false, services.AbiService.Run, services.AbiService.Close)

			// Initialize State storage service
			processHandler.Add("State Storage Service", false, services.StateService.Run, services.StateService.Close)

			// Initialize Transactions Storage Service
			processHandler.Add("Transactions Storage Service", false, services.TransactionService.Run, services.TransactionService.Close)

			// Initialize Block Storage Service
			processHandler.Add("Block Storage Service", false, services.BlockService.Run, services.BlockService.Close)

			// Initialize Contract Hash storage service
			processHandler.Add("Contract Hash Storage Service", false, services.ContractHashService.Run, services.ContractHashService.Close)

			// Subscribe the Starknet Synchronizer to the main loop if it is enabled in
			// the config.
			if config.Runtime.Starknet.Enabled {
				var ethereumClient *ethclient.Client
				if !config.Runtime.Starknet.ApiSync {
					// check if the ethereum node has been changed
					var err error
					ethereumClient, err = ethclient.Dial(config.Runtime.Ethereum.Node)
					if err != nil {
						Logger.With("Error", err).Fatal("Unable to connect to Ethereum Client")
					}
				}
				// Synchronizer for Starknet State
				env, err := db.GetMDBXEnv()
				if err != nil {
					Logger.Fatal(err)
				}
				synchronizerDb, err := db.NewMDBXDatabase(env, "SYNCHRONIZER")
				if err != nil {
					Logger.With("Error", err).Fatal("Error starting the SYNCHRONIZER database")
				}
				stateSynchronizer := starknet.NewSynchronizer(synchronizerDb, ethereumClient, feederGatewayClient)
				// Initialize the Starknet Synchronizer Service.
				processHandler.Add("Starknet Synchronizer", true, stateSynchronizer.UpdateState,
					stateSynchronizer.Close)
			}

			// Subscribe the REST API client to the main loop if it is enabled in
			// the config.
			if config.Runtime.REST.Enabled {
				s := rest.NewServer(":"+strconv.Itoa(config.Runtime.REST.Port), config.Runtime.Starknet.FeederGateway, config.Runtime.REST.Prefix)
				// Initialize the REST Service.
				processHandler.Add("REST", true, s.ListenAndServe, s.Close)
			}
			// Print with the updated values
			Logger.With(
				"Database Path", config.Runtime.DbPath,
				"Rpc Port", config.Runtime.RPC.Port,
				"Rpc Enabled", config.Runtime.RPC.Enabled,
				"Rest Port", config.Runtime.REST.Port,
				"Rest Enabled", config.Runtime.REST.Enabled,
				"Rest Prefix", config.Runtime.REST.Prefix,
			).Info("Config values.")

			primaryServiceCheck := processHandler.PrimaryServiceChecker()

			if primaryServiceCheck > 0 {
				// endless running process
				Logger.Info("Starting all processes...")
				processHandler.Run()
				cleanup()
			} else {
				cleanup()
			}
		},
	}
)

// Function for updating the parameters of the config
// configParam - The argument for corresponding to the viper variable
// flag - The value of the flag (if provided by the user)
// envVarName - The corresponding environment variable name
// t - The datatype of the config parameter (can be bool, int, string)
func handleConfig(configParam, flag, envVarName string, t int) error {
	val := ""
	if flag != "" {
		val = flag
	} else {
		envVar := os.Getenv(envVarName)
		val = envVar
	}

	if val != "" {
		if t == 0 {
			enabled, err := strconv.ParseBool(val)
			if err != nil {
				return err
			}
			viper.Set(configParam, enabled)
		} else {
			if t == 1 {
				num, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				viper.Set(configParam, num)
			} else {
				viper.Set(configParam, val)
			}
		}
	}
	return nil
}

func cleanup() {
	processHandler.Close()
	Logger.Info("App closing...Bye!!!")
}

func updateConfigFile(cfgFile string) {
	Logger.Info("Updating the config file with the flags/environment variables")
	var f string
	if cfgFile != "" {
		// Use Config file specified by the flag.
		f = cfgFile
	} else {
		// Use the default path for user configuration.
		f = filepath.Join(config.Dir, "juno.yaml")
	}
	data, err := yaml.Marshal(&config.Runtime)
	if err != nil {
		Logger.With("Error", err).Fatal("Error starting the SYNCHRONIZER database")
	}
	err = os.WriteFile(f, data, 0o644)
	if err != nil {
		Logger.With("Error", err).Fatal("Failed to write config file.")
	}
}

// init defines flags and handles configuration.
func init() {
	fmt.Println(longMsg)
	// Set the functions to be run when rootCmd.Execute() is called.
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", fmt.Sprintf(
		"config file (default is %s)", filepath.Join(config.Dir, "juno.yaml")))
	rootCmd.PersistentFlags().StringVar(&dataDir, "dataDir", "", fmt.Sprintf(
		"data path (default is %s)", config.DataDir))
	// RPC
	rootCmd.PersistentFlags().StringP("rpcport", "p", os.Getenv("RPCPORT"), "Set the RPC Port")
	rootCmd.PersistentFlags().StringP("rpcenable", "P", os.Getenv("RPCENABLE"), "Set if you would like to enable the RPC")
	// Rest
	rootCmd.PersistentFlags().StringP("restport", "r", os.Getenv("RESTPORT"), "Set the REST Port")
	rootCmd.PersistentFlags().StringP("restenable", "R", os.Getenv("RESTENABLE"), "Set if you would like to enable the REST")
	rootCmd.PersistentFlags().StringP("restprefix", "x", os.Getenv("RESTPREFIX"), "Set the REST prefix")
	// Metrics
	rootCmd.PersistentFlags().StringP("metricsport", "m", os.Getenv("METRICSPORT"), "Set the port where you would like to see the metrics")
	rootCmd.PersistentFlags().StringP("metricsenable", "M", os.Getenv("METRICSENABLE"), "Set if you would like to enable metrics")
	// Starknet
	rootCmd.PersistentFlags().StringP("feedergateway", "s", os.Getenv("FEEDERGATEWAY"), "Set the link to the feeder gateway")
	rootCmd.PersistentFlags().StringP("network", "n", os.Getenv("NETWORK"), "Set the network")
	rootCmd.PersistentFlags().StringP("starknetenable", "S", os.Getenv("STARKNETENABLE"), "Set if you would like to enable calls from feeder gateway")
	rootCmd.PersistentFlags().StringP("apisync", "A", os.Getenv("APISYNC"), "Set if you would like to enable api sync")
	// Ethereum
	rootCmd.PersistentFlags().StringP("ethereumnode", "e", os.Getenv("ETHEREUMNODE"), "Set the ethereum node")
	// DBPath
	rootCmd.PersistentFlags().StringP("dbpath", "d", os.Getenv("DBPATH"), "Set the DB Path")
	// Option for updating the config file
	rootCmd.PersistentFlags().StringP("updateconfigfile", "U", os.Getenv("UPDATECONFIGFILE"), "Set it to true if you would like to update the config file")
}

// initConfig reads in Config file or environment variables if set.
func initConfig() {
	if dataDir != "" {
		info, err := os.Stat(dataDir)
		if err != nil || !info.IsDir() {
			Logger.Info("Invalid data directory. The default data directory will be used")
			dataDir = config.DataDir
		}
	}
	if cfgFile != "" {
		// Use Config file specified by the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Use the default path for user configuration.
		viper.AddConfigPath(config.Dir)
		viper.SetConfigType("yaml")
		viper.SetConfigName("juno")
	}

	// Check whether the environment variables match any of the existing
	// keys and loads them if they are found.
	viper.AutomaticEnv()

	err := viper.ReadInConfig()
	if err == nil {
		Logger.With("File", viper.ConfigFileUsed()).Info("Using config file:")
	} else {
		Logger.Info("Config file not found.")
		if !config.Exists() {
			config.New()
		}
		viper.SetConfigFile(filepath.Join(config.Dir, "juno.yaml"))
		err = viper.ReadInConfig()
		if err != nil {
			Logger.With("Error", err).Fatal("Failed to read in Config after generation.")
		}
	}

	// Unmarshal and log runtime config instance.
	err = viper.Unmarshal(&config.Runtime)
	errpkg.CheckFatal(err, "Unable to unmarshal runtime config instance.")

	// Configure logger - we want the logger to be created right after the config has been set
	enableJsonOutput := config.Runtime.Logger.EnableJsonOutput
	verbosityLevel := config.Runtime.Logger.VerbosityLevel
	err = ReplaceGlobalLogger(enableJsonOutput, verbosityLevel)
	errpkg.CheckFatal(err, "Failed to initialise logger.")
	Logger.With(
		"Verbosity Level", verbosityLevel,
		"Json Output", enableJsonOutput,
	).Info("Logger values")

	Logger.With(
		"Database Path", config.Runtime.DbPath,
		"Rpc Port", config.Runtime.RPC.Port,
		"Rpc Enabled", config.Runtime.RPC.Enabled,
		"Rest Port", config.Runtime.REST.Port,
		"Rest Enabled", config.Runtime.REST.Enabled,
		"Rest Prefix", config.Runtime.REST.Prefix,
	).Info("Config values.")
}

// Execute handle flags for Cobra execution.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		Logger.With("Error", err).Error("Failed to execute CLI.")
	}
}
