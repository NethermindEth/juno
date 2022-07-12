package cli

import (
	"fmt"
	"os"
	"strconv"

	"github.com/NethermindEth/juno/internal/config"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

// updateConfig updates the configuration values from an instance of the Config struct.
func updateConfig(newConfig config.Config) error {
	file := viper.ConfigFileUsed()
	data, err := yaml.Marshal(&newConfig)
	if err != nil {
		return err
	}
	err = os.WriteFile(file, data, 0o644)
	if err != nil {
		return err
	}
	return nil
}

// createDynamicConfigCmd represents the createDynamicConfig command
var createDynamicConfigCmd = &cobra.Command{
	Use:   "configure",
	Short: "Update your juno configuration file",
	Long: `Available fields:
		- Enable/Disable:
			- RPC Server 
			- Node Metrics Server
			- Node REST API
			- API Synchronization Mode

		- Ethereum Node URL
		- Network ID

		Advanced:
		- Port Settings
		- Logger Verbosity Settings
	.`,
	Run: func(cmd *cobra.Command, args []string) {
		main()
	},
}

func main() {
	app := tview.NewApplication()
	runtime := *config.Runtime
	newConfig := &runtime

	networkList := []string{"NO CHANGE", "mainnet", "goerli"}
	verbosityList := []string{"NO CHANGE", "DEBUG", "INFO", "WARN", "ERROR", "DPANIC", "PANIC", "FATAL"}

	// Create two forms which are shown in the same flex window.
	formLeft := tview.NewForm()
	formLeft.SetBorder(true).SetTitle(fmt.Sprintf("Juno Config (%s)", viper.ConfigFileUsed())).SetTitleColor(tcell.ColorPurple)

	// Add fields to the form.
	formLeft.
		AddCheckbox("RPC Enabled", runtime.RPC.Enabled, func(checked bool) {
			newConfig.RPC.Enabled = checked
		}).
		AddCheckbox("Metrics Enabled", runtime.Metrics.Enabled, func(checked bool) {
			newConfig.Metrics.Enabled = checked
		}).
		AddCheckbox("REST API Enabled", runtime.REST.Enabled, func(checked bool) {
			newConfig.REST.Enabled = checked
		}).
		AddInputField("Ethereum Node URL", runtime.Ethereum.Node, 20, nil, func(nodeURL string) {
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
			case "NO CHANGE":
				newConfig.Starknet.FeederGateway = runtime.Starknet.FeederGateway
				newConfig.Starknet.Network = runtime.Starknet.Network
			}
		}).
		AddCheckbox("API Sync Enabled", runtime.Starknet.ApiSync, func(checked bool) {
			newConfig.Starknet.ApiSync = checked
		}).
		AddButton("Save (CTRL + S)", func() {
			// Close the application
			app.Stop()
			// Update the config file with the new values
			updateConfig(*newConfig)
		}).
		AddButton("Cancel (esc)", func() {
			// Close the application
			app.Stop()
			fmt.Println("Cancelled Config Update.")
		})

	// Create new form and set title to Juno Config, draw box edges.
	formRight := tview.NewForm()
	formRight.SetBorder(true).SetTitle("Advanced Settings").SetTitleAlign(tview.AlignCenter)

	// Add checkboxes, input fields, and buttons.
	formRight.
		AddInputField("RPC Port", fmt.Sprintf("%d", runtime.RPC.Port), 5, nil, func(rpcPort string) {
			port, err := strconv.Atoi(rpcPort)
			// If the port is not a number, set it to the previous value.
			if err != nil {
				newConfig.RPC.Port = port
			}
			newConfig.RPC.Port = port
		}).
		AddInputField("Metrics Port", fmt.Sprintf("%d", runtime.Metrics.Port), 5, nil, func(metricsPort string) {
			port, err := strconv.Atoi(metricsPort)
			// If the port is not a number, set it to the previous value.
			if err != nil {
				newConfig.Metrics.Port = port
			}
			newConfig.Metrics.Port = port
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
			if option == "NO CHANGE" {
				newConfig.Logger.VerbosityLevel = runtime.Logger.VerbosityLevel
			} else {
				newConfig.Logger.VerbosityLevel = option
			}
		}).
		AddCheckbox("Logger JSON output", runtime.Logger.EnableJsonOutput, func(checked bool) {
			newConfig.Logger.EnableJsonOutput = checked
		})

	// Create flex layout with forms.
	flex := tview.NewFlex().
		AddItem(formLeft, 0, 1, true).
		AddItem(formRight, 0, 1, true)

	// Allow q to quit the application.
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// If user presses Ctrl+q, quit the application.
		if event.Key() == tcell.KeyEscape {
			app.Stop()
		} else if event.Key() == tcell.KeyCtrlS {
			app.Stop()
			updateConfig(*newConfig)
		}
		return event
	})
	// Open the application last
	if err := app.SetRoot(flex, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}

func init() {
	rootCmd.AddCommand(createDynamicConfigCmd)
}
