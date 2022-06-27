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

			flag, _ := cmd.Flags().GetString("feedergateway")
			if flag != "" {
				err := updateFeederGateway(flag)
				if err != nil {
					errpkg.CheckFatal(err, "Failed to update the feeder gateway")
				}
			}
			feederGatewayClient := feeder.NewClient(config.Runtime.Starknet.FeederGateway, "/feeder_gateway", nil)
			// Subscribe the RPC client to the main loop if it is enabled in
			// the config.
			// Check if the user has enabled the rpc
			flag, _ = cmd.Flags().GetString("rpcenable")
			if flag != "" {
				err := updateRPCEnable(flag)
				if err != nil {
					errpkg.CheckFatal(err, "Failed to update the RPC enable")
				}
			}
			if config.Runtime.RPC.Enabled {
				// Check if the user has defined the RPC port externally
				flag, _ = cmd.Flags().GetString("rpcport")
				if flag != "" {
					err := updateRPCPort(flag)
					if err != nil {
						errpkg.CheckFatal(err, "Failed to update the RPC port")
					}
				}
				s := rpc.NewServer(":"+strconv.Itoa(config.Runtime.RPC.Port), feederGatewayClient)
				processHandler.Add("RPC", s.ListenAndServe, s.Close)
			}

			flag, _ = cmd.Flags().GetString("metricsenable")
			if flag != "" {
				err := updateMetricsEnable(flag)
				if err != nil {
					errpkg.CheckFatal(err, "Failed to update the Metrics enable")
				}
			}
			if config.Runtime.Metrics.Enabled {
				flag, _ = cmd.Flags().GetString("metricsport")
				if flag != "" {
					err := updateMetricsPort(flag)
					if err != nil {
						errpkg.CheckFatal(err, "Failed to update the Metrics port")
					}
				}
				s := metric.SetupMetric(":" + strconv.Itoa(config.Runtime.Metrics.Port))
				processHandler.Add("Metrics", s.ListenAndServe, s.Close)
			}

			flag, _ = cmd.Flags().GetString("dbpath")
			if flag != "" {
				err := updateDbPath(flag)
				if err != nil {
					errpkg.CheckFatal(err, "Failed to update the DB Path")
				}
			}
			if err := db.InitializeDatabaseEnv(config.Runtime.DbPath, 10, 0); err != nil {
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
			flag, _ = cmd.Flags().GetString("starknetenable")
			if flag != "" {
				err := updateStarknetEnable(flag)
				if err != nil {
					errpkg.CheckFatal(err, "Failed to update the Starknet Enable")
				}
			}
			if config.Runtime.Starknet.Enabled {
				var ethereumClient *ethclient.Client
				// Check if the api sync has been enabled or disabled
				flag, _ = cmd.Flags().GetString("apisync")
				if flag != "" {
					err := updateAPISync(flag)
					if err != nil {
						errpkg.CheckFatal(err, "Failed to update the API sync")
					}
				}
				if !config.Runtime.Starknet.ApiSync {
					// check if the ethereum node has been changed
					flag, _ = cmd.Flags().GetString("ethereumnode")
					if flag != "" {
						err := updateEthereumNode(flag)
						if err != nil {
							errpkg.CheckFatal(err, "Failed to update the Ethereum node")
						}
					}
					var err error
					ethereumClient, err = ethclient.Dial(config.Runtime.Ethereum.Node)
					if err != nil {
						log.Default.With("Error", err).Fatal("Unable to connect to Ethereum Client")
					}
				}
				// Synchronizer for Starknet State
				synchronizerDb, err := db.GetDatabase("SYNCHRONIZER")
				if err != nil {
					log.Default.With("Error", err).Fatal("Error starting the SYNCHRONIZER database")
				}
				stateSynchronizer := starknet.NewSynchronizer(synchronizerDb, ethereumClient, feederGatewayClient)
				processHandler.Add("Starknet Synchronizer", stateSynchronizer.UpdateState,
					stateSynchronizer.Close)
			}

			// Subscribe the REST API client to the main loop if it is enabled in
			// the config.
			flag, _ = cmd.Flags().GetString("restenable")
			if flag != "" {
				err := updateRESTEnable(flag)
				if err != nil {
					errpkg.CheckFatal(err, "Failed to update the REST enable")
				}
			}
			if config.Runtime.REST.Enabled {
				flag, _ = cmd.Flags().GetString("restport")
				if flag != "" {
					err := updateRESTPort(flag)
					if err != nil {
						errpkg.CheckFatal(err, "Failed to update the REST port")
					}
				}
				flag, _ = cmd.Flags().GetString("restprefix")
				if flag != "" {
					err := updateRESTPrefix(flag)
					if err != nil {
						errpkg.CheckFatal(err, "Failed to update the REST prefix")
					}
				}
				s := rest.NewServer(":"+strconv.Itoa(config.Runtime.REST.Port), config.Runtime.Starknet.FeederGateway, config.Runtime.REST.Prefix)
				processHandler.Add("REST", s.ListenAndServe, s.Close)
			}

			// endless running process
			log.Default.Info("Starting all processes...")
			processHandler.Run()
			cleanup()
		},
	}
)

func cleanup() {
	processHandler.Close()
	log.Default.Info("App closing...Bye!!!")
}

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

// init defines flags and handles configuration.
func init() {
	fmt.Println(longMsg)
	// Set the functions to be run when rootCmd.Execute() is called.
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", fmt.Sprintf(
		"config file (default is %s)", filepath.Join(config.Dir, "juno.yaml")))
	rootCmd.PersistentFlags().StringVar(&dataDir, "dataDir", "", fmt.Sprintf(
		"data path (default is %s)", config.DataDir))

	rootCmd.Flags().StringP("rpcport", "p", viper.GetString("RPCPORT"), "Set the RPC Port")
	rootCmd.Flags().StringP("rpcenable", "P", viper.GetString("RPCENABLE"), "Set if you would like to enable the RPC")
	// Rest
	rootCmd.Flags().StringP("restport", "r", viper.GetString("RESTPORT"), "Set the REST Port")
	rootCmd.Flags().StringP("restenable", "R", viper.GetString("RESTENABLE"), "Set if you would like to enable the REST")
	rootCmd.Flags().StringP("restprefix", "x", viper.GetString("RESTPREFIX"), "Set the REST prefix")
	//Metrics
	rootCmd.Flags().StringP("metricsport", "m", viper.GetString("METRICSPORT"), "Set the port where you would like to see the metrics")
	rootCmd.Flags().StringP("metricsenable", "M", viper.GetString("METRICSENABLE"), "Set if you would like to enable metrics")
	// Starknet
	rootCmd.Flags().StringP("feedergateway", "s", viper.GetString("FEEDERGATEWAY"), "Set the link to the feeder gateway")
	rootCmd.Flags().StringP("network", "n", viper.GetString("NETWORK"), "Set the network")
	rootCmd.Flags().StringP("starknetenable", "S", viper.GetString("STARKNETENABLE"), "Set if you would like to enable calls from feeder gateway")
	rootCmd.Flags().StringP("apisync", "A", viper.GetString("APISYNC"), "Set if you would like to enable api sync")
	// Ethereum
	rootCmd.Flags().StringP("ethereumnode", "e", viper.GetString("ETHEREUMNODE"), "Set the ethereum node")
	//DBPath
	rootCmd.Flags().StringP("dbpath", "d", viper.GetString("DBPATH"), "Set the DB Path")
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
	log.Default.With(
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
		log.Default.With("Error", err).Error("Failed to execute CLI.")
	}
}
