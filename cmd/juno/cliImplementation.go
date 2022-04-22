package main    //change if needed 

//various different imports, some are not used, but included just incase if needed to implement any /juno/internal files
// Else comment out unused imports to run code without needing to fight with the Go compiler

// For reference to CLI commands on Cairo, I used https://www.cairo-lang.org/docs/hello_starknet/cli.html

import (
	"fmt"
	"os/exec"
	"os/signal"
	"path/filepath"
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

//Many will need more further implementation on specific function outputs after Run: func(cmd *cobra.Command, args []string){} 
// CURRENTLY SETUP TO PRINT THE GIVEN ARGS, REPLACE WITH WHATEVER NEEDS TO BE USED 

func cliCmd() {

    var getTransaction = &cobra.Command{   // Get_Transaction CLI command
        Use: "juno get_transaction --hash [TRANSACTION_HASH]",
        Short: "The result contains transaction information",
        Long: `https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-transaction`,
        Args: cobra.MinimumNArgs(1),
        Run: func(cmd *cobra.Command, args []string) {
          fmt.Println("Print: " + strings.Join(args, " "))
        },
      }

    var getTransactionReceipt = &cobra.Command{   // Get_Transaction Receipt CLI command
        Use: "juno get_transaction_receipt --hash [TRANSACTION_HASH]",
        Short: "Transaction receipt contains execution information, such as L1<->L2 interaction and consumed resources, in addition to its block information, similar to get_transaction",
        Long: `https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-transaction-receipt`,
        Args: cobra.MinimumNArgs(1),
        Run: func(cmd *cobra.Command, args []string) {
          fmt.Println("Print: " + strings.Join(args, " "))
        },
      }

    var getTransactionTrace = &cobra.Command{   // Get_Transaction Trace CLI command
        Use: "juno get_transaction_trace --hash [TRANSACTION_HASH]",
        Short: `Transaction trace contains execution information in a nested structure of calls; every call, starting from the external transaction, contains a list of inner calls,ordered chronologically.
                For each such call, the trace holds the following: caller/callee addresses, selector, calldata along with execution information such as its return value, emitted events, and sent messages.`,
        Long: `https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-transaction-trace`,
        Args: cobra.MinimumNArgs(1),
        Run: func(cmd *cobra.Command, args []string) {
          fmt.Println("Print: " + strings.Join(args, " "))
        },
      }

    var estimateFee = &cobra.Command{   // estimate_fee CLI command
        Use: `To estimate the given fee use: 
              juno estimate_fee \
                 --address [CONTRACT_ADDRESS] \
                 --abi contract_abi.json \
                 --function increase_balance \
                 --inputs 1234`,
        Short: `You can estimate the fee of a given transaction before invoking it. The following command is similar to starknet call, but it returns the estimated fee associated with the transaction. `,
        Long: `https://www.cairo-lang.org/docs/hello_starknet/cli.html#estimate-fee`,
        Args: cobra.MinimumNArgs(1),       //not sure if it should be changed to 5 since requiring 4 extra args for address, abi, function, input 
        Run: func(cmd *cobra.Command, args []string) {
          fmt.Println("Print: " + strings.Join(args, " "))
        },
      }

    var getCode = &cobra.Command{   // get_code CLI command
        Use: "juno get_code --contract_address [CONTRACT_ADDRESS]",
        Short: `Once the deploy transaction is accepted on-chain, you will be able to see the code of the contract you have just deployed.
                The output consists of a list of bytecodes, rather than the source code. This is because the StarkNet network gets the contract after compilation.`,
        Long: `https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-code`,
        Args: cobra.MinimumNArgs(1),
        Run: func(cmd *cobra.Command, args []string) {
          fmt.Println("Print: " + strings.Join(args, " "))
        },
      }

    var getFullContract = &cobra.Command{   // get_full_contract CLI command
        Use: "juno get_full_contract --contract_address [CONTRACT_ADDRESS]",
        Short: `To get the full contract definition of a contract at a specific address`,
        Long: `https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-full-contract`,
        Args: cobra.MinimumNArgs(1),
        Run: func(cmd *cobra.Command, args []string) {
          fmt.Println("Print: " + strings.Join(args, " "))
        },
      }

    var getBlock = &cobra.Command{   // get_block CLI command
        Use: "juno get_block --number [BLOCK_NUMBER]",
        Short: `Instead of querying a specific contract or transaction, you may want to query an entire block and examine the transactions contained within it.`,
        Long: `https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-block`,
        Args: cobra.MinimumNArgs(1),
        Run: func(cmd *cobra.Command, args []string) {
          fmt.Println("Print: " + strings.Join(args, " "))
        },
      }

    var getStateUpdate = &cobra.Command{   // get_state_update CLI command
        Use: "juno get_state_update --block_number [BLOCK_NUMBER]",
        Short: `You can use the following command to get the state changes in a specific block (for example, what storage cells have changed).`,
        Long: `https://www.cairo-lang.org/docs/hello_starknet/cli.html#get-state-update`,
        Args: cobra.MinimumNArgs(1),
        Run: func(cmd *cobra.Command, args []string) {
          fmt.Println("Print: " + strings.Join(args, " "))
        },
      }

      //getStorageAt function is more complicated, don't think this is right as it requires running a python code to get balance key first 
    var getStorageAt = &cobra.Command{   // get_storage_at CLI command
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

//Add flags here if needed 

}
//executes handle flags for cobra 
func Execute() {
	if err := cliCmd.Execute(); err != nil {
		log.Default.With("Error", err).Error("Failed to execute CLI.")
	}
}

// executes python script
func main() {      //replace main with execute or other func if needed or add into just 1 whole function 
	cmd := exec.Command("python3.7", "-m", "venv ~/cairo_venv")
  //	cmd := exec.Command("python3", "-u", "/home/abcoder/pytest.py")  //REPLACE FILE DIRECTORY & FILE WITH PROPER CAIRO ENV AS A PYTHON SCRIPT
	  fmt.Println(cmd.Args)
	  out, err := cmd.CombinedOutput()
	  if err != nil {
		  fmt.Println(err)
	  }
	  fmt.Println(string(out))                //Stores the ouput from test file and prints 
  
	cmd2 := exec.Command("source ~/cairo_venv/bin/activate")
	   fmt.Println(cmd2.Args)
	   out, err := cmd2.CombinedOutput()
	   if err != nil {
		fmt.Println(err)
		  }
		fmt.Println(string(out))                //Stores the ouput from test file and prints 


//curently CLI commands not used, add/change code with json RPC, root file, gateway client, etc as such needed 
}
