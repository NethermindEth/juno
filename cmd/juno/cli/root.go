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
	"github.com/NethermindEth/juno/internal/log"
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
				log.Default.Info("Trying to close...")
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

			// This variable keeps a count of the number of flags
			// If it is zero and the user asks for updating the config file,
			// then the file is not updated and the user is informed
			flagcount := 0
			flag, _ := cmd.PersistentFlags().GetString("feedergateway")
			if flag != "" {
				err := updateFeederGateway(flag)
				if err != nil {
					errpkg.CheckFatal(err, "Failed to update the feeder gateway")
				}
				flagcount++
			}
			flag, _ = cmd.PersistentFlags().GetString("rpcenable")
			if flag != "" {
				err := updateRPCEnable(flag)
				if err != nil {
					errpkg.CheckFatal(err, "Failed to update the RPC enable")
				}
				flagcount++
			}
			flag, _ = cmd.PersistentFlags().GetString("rpcport")
			if flag != "" {
				err := updateRPCPort(flag)
				if err != nil {
					errpkg.CheckFatal(err, "Failed to update the RPC port")
				}
				flagcount++
			}
			flag, _ = cmd.PersistentFlags().GetString("metricsenable")
			if flag != "" {
				err := updateMetricsEnable(flag)
				if err != nil {
					errpkg.CheckFatal(err, "Failed to update the Metrics enable")
				}
				flagcount++
			}
			flag, _ = cmd.PersistentFlags().GetString("metricsport")
			if flag != "" {
				err := updateMetricsPort(flag)
				if err != nil {
					errpkg.CheckFatal(err, "Failed to update the Metrics port")
				}
				flagcount++
			}
			flag, _ = cmd.PersistentFlags().GetString("dbpath")
			if flag != "" {
				err := updateDbPath(flag)
				if err != nil {
					errpkg.CheckFatal(err, "Failed to update the DB Path")
				}
				flagcount++
			}
			flag, _ = cmd.PersistentFlags().GetString("starknetenable")
			if flag != "" {
				err := updateStarknetEnable(flag)
				if err != nil {
					errpkg.CheckFatal(err, "Failed to update the Starknet Enable")
				}
				flagcount++
			}
			flag, _ = cmd.PersistentFlags().GetString("apisync")
			if flag != "" {
				err := updateAPISync(flag)
				if err != nil {
					errpkg.CheckFatal(err, "Failed to update the API sync")
				}
				flagcount++
			}
			flag, _ = cmd.PersistentFlags().GetString("ethereumnode")
			if flag != "" {
				err := updateEthereumNode(flag)
				if err != nil {
					errpkg.CheckFatal(err, "Failed to update the Ethereum node")
				}
				flagcount++
			}
			flag, _ = cmd.PersistentFlags().GetString("restenable")
			if flag != "" {
				err := updateRESTEnable(flag)
				if err != nil {
					errpkg.CheckFatal(err, "Failed to update the REST enable")
				}
				flagcount++
			}
			flag, _ = cmd.PersistentFlags().GetString("restport")
			if flag != "" {
				err := updateRESTPort(flag)
				if err != nil {
					errpkg.CheckFatal(err, "Failed to update the REST port")
				}
				flagcount++
			}
			flag, _ = cmd.PersistentFlags().GetString("restprefix")
			if flag != "" {
				err := updateRESTPrefix(flag)
				if err != nil {
					errpkg.CheckFatal(err, "Failed to update the REST prefix")
				}
				flagcount++
			}
			flag, _ = cmd.PersistentFlags().GetString("updateconfigfile")
			if flagcount > 0 {
				log.Default.Info("No flags provided for updating the config file")
				flag = ""
			}
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
				processHandler.Add("RPC", s.ListenAndServe, s.Close)
			}

			if config.Runtime.Metrics.Enabled {
				s := metric.SetupMetric(":" + strconv.Itoa(config.Runtime.Metrics.Port))
				// Initialize the Metrics Service.
				processHandler.Add("Metrics", s.ListenAndServe, s.Close)
			}

			if err := db.InitializeMDBXEnv(config.Runtime.DbPath, 100, 0); err != nil {
				log.Default.With("Error", err).Fatal("Error starting the database environment")
			}

			// Initialize ABI Service
			processHandler.Add("ABI Service", services.AbiService.Run, services.AbiService.Close)

			// Initialize State storage service
			processHandler.Add("State Storage Service", services.StateService.Run, services.StateService.Close)

			// Initialize Transactions Storage Service
			processHandler.Add("Transactions Storage Service", services.TransactionService.Run, services.TransactionService.Close)

			// Initialize Block Storage Service
			processHandler.Add("Block Storage Service", services.BlockService.Run, services.BlockService.Close)

			// Initialize Contract Hash storage service
			processHandler.Add("Contract Hash Storage Service", services.ContractHashService.Run, services.ContractHashService.Close)

			// Subscribe the Starknet Synchronizer to the main loop if it is enabled in
			// the config.
			if config.Runtime.Starknet.Enabled {
				var ethereumClient *ethclient.Client
				if !config.Runtime.Starknet.ApiSync {
					// check if the ethereum node has been changed
					var err error
					ethereumClient, err = ethclient.Dial(config.Runtime.Ethereum.Node)
					if err != nil {
						log.Default.With("Error", err).Fatal("Unable to connect to Ethereum Client")
					}
				}
				// Synchronizer for Starknet State
				env, err := db.GetMDBXEnv()
				if err != nil {
					log.Default.Fatal(err)
				}
				synchronizerDb, err := db.NewMDBXDatabase(env, "SYNCHRONIZER")
				if err != nil {
					log.Default.With("Error", err).Fatal("Error starting the SYNCHRONIZER database")
				}
				stateSynchronizer := starknet.NewSynchronizer(synchronizerDb, ethereumClient, feederGatewayClient)
				// Initialize the Starknet Synchronizer Service.
				processHandler.Add("Starknet Synchronizer", stateSynchronizer.UpdateState,
					stateSynchronizer.Close)
			}

			// Subscribe the REST API client to the main loop if it is enabled in
			// the config.
			if config.Runtime.REST.Enabled {
				s := rest.NewServer(":"+strconv.Itoa(config.Runtime.REST.Port), config.Runtime.Starknet.FeederGateway, config.Runtime.REST.Prefix)
				// Initialize the REST Service.
				processHandler.Add("REST", s.ListenAndServe, s.Close)
			}
			// Print with the updated values
			log.Default.With(
				"Database Path", config.Runtime.DbPath,
				"Rpc Port", config.Runtime.RPC.Port,
				"Rpc Enabled", config.Runtime.RPC.Enabled,
				"Rest Port", config.Runtime.REST.Port,
				"Rest Enabled", config.Runtime.REST.Enabled,
				"Rest Prefix", config.Runtime.REST.Prefix,
			).Info("Config values.")
			// endless running process
			log.Default.Info("Starting all processes...")
			processHandler.Run()
			cleanup()
		},
	}
)

// Functions for updating the configuration for the run
func updateRPCPort(args string) error {
	port, err := strconv.Atoi(args)
	if err != nil {
		return err
	}
	config.Runtime.RPC.Port = port
	return nil
}

func updateRPCEnable(args string) error {
	enabled, err := strconv.ParseBool(args)
	if err != nil {
		return err
	}
	config.Runtime.RPC.Enabled = enabled
	return nil
}

func updateRESTPort(args string) error {
	port, err := strconv.Atoi(args)
	if err != nil {
		return err
	}
	config.Runtime.REST.Port = port
	return nil
}

func updateRESTEnable(args string) error {
	enabled, err := strconv.ParseBool(args)
	if err != nil {
		return err
	}
	config.Runtime.REST.Enabled = enabled
	return nil
}

func updateRESTPrefix(args string) error {
	config.Runtime.REST.Prefix = args
	return nil
}

func updateMetricsPort(args string) error {
	port, err := strconv.Atoi(args)
	if err != nil {
		return err
	}
	config.Runtime.Metrics.Port = port
	return nil
}

func updateMetricsEnable(args string) error {
	enabled, err := strconv.ParseBool(args)
	if err != nil {
		return err
	}
	config.Runtime.Metrics.Enabled = enabled
	return nil
}

func updateFeederGateway(args string) error {
	config.Runtime.Starknet.FeederGateway = args
	return nil
}

func updateNetwork(args string) error {
	config.Runtime.Starknet.Network = args
	return nil
}

func updateStarknetEnable(args string) error {
	enabled, err := strconv.ParseBool(args)
	if err != nil {
		return err
	}
	config.Runtime.Starknet.Enabled = enabled
	return nil
}

func updateAPISync(args string) error {
	enabled, err := strconv.ParseBool(args)
	if err != nil {
		return err
	}
	config.Runtime.Starknet.ApiSync = enabled
	return nil
}

func updateEthereumNode(args string) error {
	config.Runtime.Ethereum.Node = args
	return nil
}

func updateDbPath(args string) error {
	config.Runtime.DbPath = args
	return nil
}

func cleanup() {
	processHandler.Close()
	log.Default.Info("App closing...Bye!!!")
}

func updateConfigFile(cfgFile string) {
	log.Default.Info("Updating the config file with the flags/environment variables")
	var f string
	if cfgFile != "" {
		// Use Config file specified by the flag.
		f = cfgFile
	} else {
		// Use the default path for user configuration.
		f = filepath.Join(config.Dir, "juno.yaml")
	}
	data, err := yaml.Marshal(&config.Runtime)
	errpkg.CheckFatal(err, "Failed to marshal Config instance to byte data.")
	err = os.WriteFile(f, data, 0o644)
	errpkg.CheckFatal(err, "Failed to write config file.")
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
			log.Default.Info("Invalid data directory. The default data directory will be used")
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
		log.Default.With("File", viper.ConfigFileUsed()).Info("Using config file:")
	} else {
		log.Default.Info("Config file not found.")
		if !config.Exists() {
			config.New()
		}
		viper.SetConfigFile(filepath.Join(config.Dir, "juno.yaml"))
		err = viper.ReadInConfig()
		errpkg.CheckFatal(err, "Failed to read in Config after generation.")
	}

	// Unmarshal and log runtime config instance.
	err = viper.Unmarshal(&config.Runtime)
	errpkg.CheckFatal(err, "Unable to unmarshal runtime config instance.")
}

// Execute handle flags for Cobra execution.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Default.With("Error", err).Error("Failed to execute CLI.")
	}
}
