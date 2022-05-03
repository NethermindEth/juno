package cli

// notest
import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
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
			log.Default.Info("App closing...Bye!!!")
		},
	}

	getTransaction = &cobra.Command{ // Get_Transaction CLI command
		Use:   "juno get_transaction --hash TRANSACTION_HASH",
		Short: "The result contains transaction information",
		Long:  `https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-transaction`,
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Print: " + strings.Join(args, " "))
		},
	}

	getTransactionReceipt = &cobra.Command{ // Get_Transaction Receipt CLI command
		Use:   "juno get_transaction_receipt --hash TRANSACTION_HASH",
		Short: "Transaction receipt contains execution information, such as L1<->L2 interaction and consumed resources, in addition to its block information, similar to get_transaction",
		Long:  `https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-transaction-receipt`,
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Print: " + strings.Join(args, " "))
		},
	}

	getTransactionTrace = &cobra.Command{ // Get_Transaction Trace CLI command
		Use: "juno get_transaction_trace --hash TRANSACTION_HASH",
		Short: `Transaction trace contains execution information in a nested structure of calls; every call, starting from the external transaction, contains a list of inner calls,
         ordered chronologically. For each such call, the trace holds the following: caller/callee addresses, selector, calldata along with execution information such as its return value, emitted events, and sent messages.`,
		Long: `https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-transaction-trace`,
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Print: " + strings.Join(args, " "))
		},
	}

	estimateFee = &cobra.Command{ // estimate_fee CLI command
		Use: `To estimate the given fee use: 
              starknet estimate_fee \
                 --address CONTRACT_ADDRESS \
                 --abi contract_abi.json \
                 --function increase_balance \
                 --inputs 1234`,
		Short: `You can estimate the fee of a given transaction before invoking it. The following command is similar to starknet call, but it returns the estimated fee associated with the transaction. `,
		Long:  `https://www.cairo-lang.org/docs/hello_starknet/cli.html#estimate-fee`,
		Args:  cobra.MinimumNArgs(1), //not sure if it should be changed to 5 since requiring 4 extra args for address, abi, function, input
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Print: " + strings.Join(args, " "))
		},
	}

	getCode = &cobra.Command{ // get_code CLI command
		Use: "juno get_code --contract_address CONTRACT_ADDRESS",
		Short: `Once the deploy transaction is accepted on-chain, you will be able to see the code of the contract you have just deployed.
                The output consists of a list of bytecodes, rather than the source code. This is because the StarkNet network gets the contract after compilation.`,
		Long: `https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-code`,
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Print: " + strings.Join(args, " "))
		},
	}

	getFullContract = &cobra.Command{ // get_full_contract CLI command
		Use:   "juno get_full_contract --contract_address CONTRACT_ADDRESS",
		Short: `To get the full contract definition of a contract at a specific address`,
		Long:  `https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-full-contract`,
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Print: " + strings.Join(args, " "))
		},
	}

	getBlock = &cobra.Command{ // get_block CLI command
		Use:   "juno get_block --number BLOCK_NUMBER",
		Short: `Instead of querying a specific contract or transaction, you may want to query an entire block and examine the transactions contained within it.`,
		Long:  `https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-block`,
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Print: " + strings.Join(args, " "))
		},
	}

	getStateUpdate = &cobra.Command{ // get_state_update CLI command
		Use:   "juno get_state_update --block_number [BLOCK_NUMBER]",
		Short: `You can use the following command to get the state changes in a specific block (for example, what storage cells have changed).`,
		Long:  `https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-state-update`,
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Print: " + strings.Join(args, " "))
		},
	}

	//getStorageAt function is more complicated, don't think this is right as it requires running a python code to get balance key first
	getStorageAt = &cobra.Command{ // get_storage_at CLI command
		Use: "juno get_storage_at",
		Short: `Other than querying the contract’s code, you may also want to query the contract’s storage at a specific key. 
                To do so, you first need to understand which key is of interest to you. As you saw before, StarkNet introduces a new primitive, which is storage variables. 
                Each storage variable is mapped to a storage key (a field element).`,
		Long: `https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-storage-at`,
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Print: " + strings.Join(args, " "))
		},
	}
)

// init defines flags and handles configuration.
func init() {
	// Set the functions to be run when rootCmd.Execute() is called.
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", fmt.Sprintf(
		"config file (default is %s)", filepath.Join(config.Dir, "juno.yaml")))
}

// initConfig reads in Config file or environment variables if set.
func initConfig() {
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
		config.New()
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
	).Info("Config values.")
}

// Execute handle flags for Cobra execution.
func Execute() {

	cmd := exec.Command("python3.7", "-m", "venv ~/cairo_venv")
	//	cmd := exec.Command("python3", "-u", "/home/abcoder/pytest.py")  //REPLACE FILE DIRECTORY & FILE WITH PROPER CAIRO ENV AS A PYTHON SCRIPT
	fmt.Println(cmd.Args)
	out1, err1 := cmd.CombinedOutput()
	if err1 != nil {
		fmt.Println(err1)
	}
	fmt.Println(string(out1)) //Stores the ouput from test file and prints

	cmd2 := exec.Command("source ~/cairo_venv/bin/activate")
	fmt.Println(cmd2.Args)
	out2, err2 := cmd2.CombinedOutput()
	if err2 != nil {
		fmt.Println(err2)
	}
	fmt.Println(string(out2)) //Stores the ouput from test file and prints

	if err := rootCmd.Execute(); err != nil {
		log.Default.With("Error", err).Error("Failed to execute CLI.")
	}
}
