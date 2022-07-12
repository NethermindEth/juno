package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/NethermindEth/juno/internal/config"
	. "github.com/NethermindEth/juno/internal/log"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// updateConfig updates the configuration values from an instance of the Config struct.
func updateConfig(*config.Config) {
}

// createDynamicConfigCmd represents the createDynamicConfig command
var createDynamicConfigCmd = &cobra.Command{
	Use:   "configure",
	Short: "Update your juno configuration file",
	Long: `Available fields:
		- nodeURL: The URL of the node to connect to.
		- network: The network to connect to (mainnet or testnet).
		- rpc: Whether or not to enable the RPC server and which port to use.
		- metrics: Whether or not to enable the metrics server and which port to use.

	.`,
	Run: func(cmd *cobra.Command, args []string) {
		main()
	},
}

func main() {
	app := tview.NewApplication()
	runtime := *config.Runtime
	newConfig := &runtime

	// TODO: Preselect logger and network from current values
	networkList := []string{"mainnet", "goerli"}
	// currentNetwork := newConfig.Starknet.Network
	verbosityList := []string{"NO CHANGE", "DEBUG", "INFO", "WARN", "ERROR", "DPANIC", "PANIC", "FATAL"}

	// Create two forms which are shown in the same flex window.
	formLeft := tview.NewForm()
	formLeft.SetBorder(true).SetTitle("Juno Config").SetTitleColor(tcell.ColorPurple)

	// Add fields to the form.
	formLeft.
		AddInputField("Node URL", runtime.Ethereum.Node, 20, nil, func(nodeURL string) {
			newConfig.Ethereum.Node = nodeURL
		}).
		AddDropDown("Network", networkList, 0, func(option string, index int) {
			// Set both the network and feeder gateway URL
			switch option {
			case "mainnet":
				newConfig.Starknet.FeederGateway = "https://alpha-mainnet.starknet.io"
				newConfig.Starknet.Network = "mainnet"
			case "goerli":
				newConfig.Starknet.FeederGateway = "https://alpha4.starknet.io"
				newConfig.Starknet.Network = "goerli"
			}
		}).
		AddCheckbox("RPC Enabled", runtime.RPC.Enabled, func(checked bool) {
			newConfig.RPC.Enabled = checked
		}).
		AddInputField("RPC Port", fmt.Sprintf("%d", runtime.RPC.Port), 5, nil, func(rpcPort string) {
			port, err := strconv.Atoi(rpcPort)
			// If the port is not a number, set it to the previous value.
			if err != nil {
				newConfig.RPC.Port = port
			}
			newConfig.RPC.Port = port
		})

	// Create new form and set title to Juno Config, draw box edges.
	formRight := tview.NewForm()
	formRight.SetBorder(true).SetTitle(viper.ConfigFileUsed()).SetTitleAlign(tview.AlignCenter)

	// Add checkboxes, input fields, and buttons.
	formRight.
		AddCheckbox("Metrics Enabled", runtime.Metrics.Enabled, func(checked bool) {
			newConfig.Metrics.Enabled = checked
		}).
		AddInputField("Metrics Port", fmt.Sprintf("%d", runtime.Metrics.Port), 5, nil, func(metricsPort string) {
			port, err := strconv.Atoi(metricsPort)
			// If the port is not a number, set it to the previous value.
			if err != nil {
				newConfig.Metrics.Port = port
			}
			newConfig.Metrics.Port = port
		}).
		AddCheckbox("REST Enabled", runtime.REST.Enabled, func(checked bool) {
			newConfig.REST.Enabled = checked
		}).
		AddInputField("REST Port", fmt.Sprintf("%d", runtime.REST.Port), 5, nil, func(restPort string) {
			port, err := strconv.Atoi(restPort)
			// If the port is not a number, set it to the previous value.
			if err != nil {
				newConfig.REST.Port = port
			}
			newConfig.REST.Port = port
		}).
		AddDropDown("Logger Verbosity", verbosityList, 0, func(option string, index int) {
			if option == "KEEP" {
				newConfig.Logger.VerbosityLevel = runtime.Logger.VerbosityLevel
			}
		}).
		AddCheckbox("Logger JSON output", runtime.Logger.EnableJsonOutput, func(checked bool) {
			newConfig.Logger.EnableJsonOutput = checked
		}).
		AddButton("Save (CTRL + S)", func() {
			// Update the config file with the new values
			updateConfig(newConfig)
			// Close the application
			app.Stop()

			printConfigInfo(*newConfig)
		}).
		AddButton("Cancel (esc)", func() {
			// Close the application
			app.Stop()
			fmt.Println("Cancelled Config Update.")
		})

	// Create flex layout with forms.
	flex := tview.NewFlex().
		AddItem(formLeft, 0, 1, true).
		AddItem(formRight, 0, 1, true).
		AddItem(tview.NewTextView(), 0, 3, false)

	// Allow q to quit the application.
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// If user presses Ctrl+q, quit the application.
		if event.Key() == tcell.KeyEscape {
			app.Stop()
		} else if event.Key() == tcell.KeyCtrlS {
			updateConfig(newConfig)
			app.Stop()

			printConfigInfo(*newConfig)

		}
		return event
	})
	// Open the application last
	if err := app.SetRoot(flex, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}

func printConfigInfo(newConfig config.Config) {
	// Marshal DEBUG
	empJSON, _ := json.MarshalIndent(newConfig, "", "  ")
	Logger.Info("Updating Config... in %s")
	fmt.Printf("Updated Config Values: \n %s \n", string(empJSON))
}

func init() {
	rootCmd.AddCommand(createDynamicConfigCmd)
}
