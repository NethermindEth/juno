package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/NethermindEth/juno/internal/config"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
)

var (
	// f    = filepath.Join(config.Dir, "juno.yaml")
	pages      = tview.NewPages()
	app        = tview.NewApplication()
	mainScreen = tview.NewTextView().SetTextColor(tcell.ColorPurple).SetText("(c) to change config \n(q) to quit")
	form       = tview.NewForm()
)

// addConfigForm is in charge of creating a tview form with the correct input fields,
// including RPC, Metrics, REST, and Node configuration.
func addConfigForm() *tview.Form {
	// TODO: Add tview flex layout to the form.
	// TODO: Validate node input somehow? Or have default info?

	// Read the runtime config values.
	runtime := *config.Runtime

	// Create different struct to temporarily hold the config values.
	// This allows us to handle errors when the user enters invalid values.
	newConfig := &runtime

	// Input node URL
	form.AddInputField("Node URL", runtime.Ethereum.Node, 20, nil, func(nodeURL string) {
		newConfig.Ethereum.Node = nodeURL
	})
	form.AddDropDown("Network", []string{"mainnet", "goerli"}, 0, func(option string, index int) {
		// Set both the network and feeder gateway
		switch option {
		case "mainnet":
			newConfig.Starknet.FeederGateway = "https://alpha-mainnet.starknet.io"
			newConfig.Starknet.Network = "mainnet"
		case "goerli":
			newConfig.Starknet.FeederGateway = "https://alpha4.starknet.io"
			newConfig.Starknet.Network = "goerli"
		}
	})

	// Checkbox for RPC service on or off
	form.AddCheckbox("RPC", runtime.RPC.Enabled, func(checked bool) {
		newConfig.RPC.Enabled = checked
	})

	// Form input for RPC port
	form.AddInputField("RPC Port", fmt.Sprintf("%d", runtime.RPC.Port), 5, nil, func(rpcPort string) {
		port, err := strconv.Atoi(rpcPort)
		// If the port is not a number, set it to the previous value.
		if err != nil {
			newConfig.RPC.Port = port
		}
		newConfig.RPC.Port = port
	})

	// Checkbox for Metrics service on or off
	form.AddCheckbox("Metrics", runtime.Metrics.Enabled, func(checked bool) {
		newConfig.Metrics.Enabled = checked
	})

	// Form input for Metrics port
	form.AddInputField("Metrics Port", fmt.Sprintf("%d", runtime.Metrics.Port), 5, nil, func(metricsPort string) {
		port, err := strconv.Atoi(metricsPort)
		// If the port is not a number, set it to the previous value.
		if err != nil {
			newConfig.Metrics.Port = runtime.Metrics.Port
		}
		newConfig.Metrics.Port = port
	})

	// Checkbox for REST service on or off
	form.AddCheckbox("REST", runtime.REST.Enabled, func(checked bool) {
		runtime.REST.Enabled = checked
	})

	// Form input for REST port
	form.AddInputField("REST Port", fmt.Sprintf("%d", runtime.REST.Port), 5, nil, func(restPort string) {
		port, err := strconv.Atoi(restPort)
		// If the port is not a number, set it to the previous value.
		if err != nil {
			newConfig.REST.Port = runtime.REST.Port
		}
		runtime.REST.Port = port
	})

	form.AddButton("Save", func() {
		// Update the config file with the new values
		updateConfig(&runtime)
		// Close the application
		app.Stop()

		// Marshal
		empJSON, _ := json.MarshalIndent(newConfig, "", "  ")
		fmt.Printf("Updated Config Values: \n %s \n", string(empJSON))
	})

	form.AddButton("Cancel", func() {
		// Close the application
		app.Stop()

		fmt.Println("Cancelled Config Update.")
	})

	return form
}

// updateConfig updates the configuration values from an instance of the Config struct.
func updateConfig(*config.Config) {
}

// createDynamicConfigCmd represents the createDynamicConfig command
var createDynamicConfigCmd = &cobra.Command{
	Use:   "configure",
	Short: "Create or update juno configuration file",
	Long: `Update you juno configuration file easily. Available fields:
	...
	.`,
	Run: func(cmd *cobra.Command, args []string) {
		main()
	},
}

func main() {
	// Allow q to quit the application.
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'q' {
			app.Stop()
		} else if event.Rune() == 'c' {
			form.Clear(true)
			addConfigForm()
			pages.SwitchToPage("Update Juno Config")
		}
		return event
	})

	pages.AddPage("Menu", mainScreen, true, true)
	pages.AddPage("Update Juno Config", form, true, false)

	// Open the application last
	if err := app.SetRoot(pages, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}

func init() {
	rootCmd.AddCommand(createDynamicConfigCmd)
}
