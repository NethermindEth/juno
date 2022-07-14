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

			flagb, _ := cmd.Flags().GetBool("rpcenabled")
			handleConfig("rpc.enabled", "RPCENABLED", flagb, true, 0)
			flagi, _ := cmd.Flags().GetInt("rpcport")
			handleConfig("rpc.port", "RPCPORT", flagi, 0, 1)

			flagb, _ = cmd.Flags().GetBool("metricsenabled")
			handleConfig("metrics.enabled", "METRICSENABLED", flagb, true, 0)
			flagi, _ = cmd.Flags().GetInt("metricsport")
			handleConfig("metrics.port", "METRICSPORT", flagi, 0, 1)

			flags, _ := cmd.Flags().GetString("dbpath")
			handleConfig("db_path", "DBPATH", flags, "", 2)

			flagb, _ = cmd.Flags().GetBool("starknetenabled")
			handleConfig("starknet.enabled", "STARKNETENABLED", flagb, true, 0)
			flagb, _ = cmd.Flags().GetBool("apisync")
			handleConfig("starknet.api_sync", "APISYNC", flagb, true, 0)
			flags, _ = cmd.Flags().GetString("feedergateway")
			handleConfig("starknet.feeder_gateway", "FEEDERGATEWAY", flags, "", 2)

			flags, _ = cmd.Flags().GetString("ethereumnode")
			handleConfig("ethereum.node", "ETHEREUMNODE", flags, "", 2)

			flagb, _ = cmd.Flags().GetBool("restenabled")
			handleConfig("rest.enabled", "RESTENABLED", flagb, true, 0)
			flagi, _ = cmd.Flags().GetInt("restport")
			handleConfig("rest.port", "RESTPORT", flagi, 0, 1)
			flags, _ = cmd.Flags().GetString("restprefix")
			handleConfig("rest.prefix", "RESTPREFIX", flags, "", 2)

			erru := viper.Unmarshal(&config.Runtime)
			errpkg.CheckFatal(erru, "Unable to unmarshal runtime config instance.")

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
			// Printing these here because printing them in the
			// initConfig only gives the value from the config file
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
// envVarName - The corresponding environment variable name
// flag - The value of the flag (if provided by the user)
// defaultVal - The default value of the flag
// t - The datatype of the config parameter (can be bool, int, string)
func handleConfig(configParam, envVarName string, flag, defaultVal interface{}, t int) error {
	// check if any flag has been provided
	// if no flag has been provided, check for the environment variable
	// if no flag or env variable has been provided, do nothing
	// do nothing -> juno runs with values loaded from config
	if flag != defaultVal {
		viper.Set(configParam, flag)
	} else {
		// os.Getenv() returns a string
		// therefore, we have to typecast
		envVar := os.Getenv(envVarName)
		if envVar != "" {
			if t == 0 {
				enabled, err := strconv.ParseBool(envVar)
				if err != nil {
					return err
				}
				viper.Set(configParam, enabled)
			} else {
				if t == 1 {
					num, err := strconv.Atoi(envVar)
					if err != nil {
						return err
					}
					viper.Set(configParam, num)
				} else {
					viper.Set(configParam, envVar)
				}
			}
		}
	}
	return nil
}

func cleanup() {
	processHandler.Close()
	Logger.Info("App closing...Bye!!!")
}

// init defines flags and handles configuration.
func init() {
	fmt.Println(longMsg)
	// Set the functions to be run when rootCmd.Execute() is called.
	cobra.OnInitialize(initConfig)

	rootCmd.Flags().StringVar(&cfgFile, "config", "", fmt.Sprintf(
		"config file (default is %s)", filepath.Join(config.Dir, "juno.yaml")))
	rootCmd.Flags().StringVar(&dataDir, "dataDir", "", fmt.Sprintf(
		"data path (default is %s)", config.DataDir))
	// RPC
	rootCmd.Flags().IntP("rpcport", "p", 0, "Set the RPC Port")
	rootCmd.Flags().BoolP("rpcenabled", "P", true, "Set if you would like to enable the RPC")
	// Rest
	rootCmd.Flags().IntP("restport", "r", 0, "Set the REST Port")
	rootCmd.Flags().BoolP("restenabled", "R", true, "Set if you would like to enable the REST")
	rootCmd.Flags().StringP("restprefix", "x", "", "Set the REST prefix")
	// Metrics
	rootCmd.Flags().IntP("metricsport", "m", 0, "Set the port where you would like to see the metrics")
	rootCmd.Flags().BoolP("metricsenabled", "M", true, "Set if you would like to enable metrics")
	// Starknet
	rootCmd.Flags().StringP("feedergateway", "s", "", "Set the link to the feeder gateway")
	rootCmd.Flags().StringP("network", "n", "", "Set the network")
	rootCmd.Flags().BoolP("starknetenabled", "S", true, "Set if you would like to enable calls from feeder gateway")
	rootCmd.Flags().BoolP("apisync", "A", true, "Set if you would like to enable api sync")
	// Ethereum
	rootCmd.Flags().StringP("ethereumnode", "e", "", "Set the ethereum node")
	// DBPath
	rootCmd.Flags().StringP("dbpath", "d", "", "Set the DB Path")
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
}

// Execute handle flags for Cobra execution.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		Logger.With("Error", err).Error("Failed to execute CLI.")
	}
}
