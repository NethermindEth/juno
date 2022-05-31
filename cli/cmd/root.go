package cmd

// notest
import (
	_ "embed"
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
		// TODO: What is the usual description of Root command?
		Use:   "juno [command] [flags]",
		Short: "Starknet client implementation in Go.",
		Long:  longMsg,
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
			log.Default.Info("App closing...Bye!!!")
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

// Define flags and load config.
func init() {
	fmt.Println(longMsg)
	cobra.OnInitialize(initConfig)

	// Set flags shared accross commands as persistent flags.
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", fmt.Sprintf(
		"config file (default is %s).", filepath.Join(config.Dir, "juno.yaml")))

	// Pretty print flag
	rootCmd.PersistentFlags().BoolP("pretty", "p", false, "Pretty print the response.")
}

// initConfig reads in Config file or environment variables if set.
func initConfig() {
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
		config.New()
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

}

// Execute handle flags for Cobra execution.
func Execute() {
	// pwdCli := os.Getenv("PWD") + "/cmd/juno/cli/tests/"
	// FIXME: Once app is compiled, change the path back to point from main app dir.

	// TODO: ASK: How to handle different networks? Their config file?

	// TODO: Remove test below once proper handling of Python env done

	// Small test to see that cairo-compile is installed and active. Deleting compiled test after.
	pwdCli := os.Getenv("PWD") + "/cmd/tests/"
	err := exec.Command("cairo-compile", pwdCli+"test.cairo", "--output", pwdCli+"test_compiled.json").Run()
	if err != nil {
		fmt.Println(err)
	}
	err = os.Remove(pwdCli + "test_compiled.json")
	if err != nil {
		fmt.Println(err)
	}
	log.Default.Debug("Cairo Test compilation Successful.")

	if err := rootCmd.Execute(); err != nil {
		log.Default.With("Error", err).Error("Failed to execute CLI.")
	}
}
