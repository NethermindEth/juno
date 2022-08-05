package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/NethermindEth/juno/internal/config"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

func newConfigureCmd(cfg *config.Juno, configFile *string) *cobra.Command {
	configureCmd := &cobra.Command{
		Use:   "configure",
		Short: "Interactively build your juno configuration file",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return loadConfig(cmd, configFile)
		},
		Run: func(_ *cobra.Command, _ []string) {
			configure(cfg, *configFile)
		},
	}

	return configureCmd
}

func configure(cfg *config.Juno, configFile string) {
	app := tview.NewApplication()

	networkList := []string{"mainnet", "goerli"}
	network := cfg.Sync.Network
	// Find index of selected network inside the networkList
	selectedNwIndex := -1
	for i, nw := range networkList {
		if strings.EqualFold(nw, network) {
			selectedNwIndex = i
			break
		}
	}
	// Find index of selected verbosity level inside the verbosityList
	currentLogLevelIndex := -1
	currentLogLevel := cfg.Log.Level
	verbosityList := []string{"DEBUG", "INFO", "WARN", "ERROR", "DPANIC", "PANIC", "FATAL"}
	for i, verb := range verbosityList {
		if strings.EqualFold(verb, currentLogLevel) {
			currentLogLevelIndex = i
			break
		}
	}

	// Create two forms which are shown in the same flex window.
	formLeft := tview.NewForm()
	formLeft.SetBorder(true).SetTitle(fmt.Sprintf("Juno Config (%s)", configFile)).SetTitleColor(tcell.ColorPurple)

	// Add fields to the form.
	formLeft.
		AddCheckbox("RPC Enabled", cfg.Rpc.Enable, func(checked bool) {
			cfg.Rpc.Enable = checked
		}).
		AddCheckbox("Metrics Enabled", cfg.Metrics.Enable, func(checked bool) {
			cfg.Metrics.Enable = checked
		}).
		AddCheckbox("REST API Enabled", cfg.Rest.Enable, func(checked bool) {
			cfg.Rest.Enable = checked
		}).
		AddInputField("Ethereum Node URL", cfg.Sync.EthNode, 20, nil, func(nodeURL string) {
			cfg.Sync.EthNode = nodeURL
		}).
		AddDropDown("Network", networkList, selectedNwIndex, func(option string, _ int) {
			// Set both the network and feeder gateway URL
			switch option {
			case "mainnet":
				cfg.Sync.Sequencer = "https://alpha-mainnet.starknet.io"
				cfg.Sync.Network = "mainnet"
			case "goerli":
				cfg.Sync.Sequencer = "https://alpha4.starknet.io"
				cfg.Sync.Network = "goerli"
			}
		}).
		AddCheckbox("API Sync Enabled", cfg.Sync.Trusted, func(checked bool) {
			cfg.Sync.Trusted = checked
		}).
		AddButton("Save (CTRL + S)", func() {
			// Close the application
			app.Stop()
			// Update the config file with the new values
			updateConfig(cfg, configFile)
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
		AddInputField("RPC Port", formatPortNumber(cfg.Rpc.Port), 5, isPortNumber, func(rpcPort string) {
			cfg.Rpc.Port, _ = parsePortNumber(rpcPort)
		}).
		AddInputField("Metrics Port", formatPortNumber(cfg.Metrics.Port), 5, isPortNumber, func(metricsPort string) {
			cfg.Metrics.Port, _ = parsePortNumber(metricsPort)
		}).
		AddInputField("REST Port", formatPortNumber(cfg.Metrics.Port), 5, isPortNumber, func(restPort string) {
			cfg.Rest.Port, _ = parsePortNumber(restPort)
		}).
		AddInputField("Database Path", cfg.Database.Path, 50, nil, func(dbPath string) {
			cfg.Database.Path = dbPath
		}).
		AddDropDown("Logger Verbosity", verbosityList, currentLogLevelIndex, func(option string, _ int) {
			cfg.Log.Level = option
		}).
		AddCheckbox("Logger JSON output", cfg.Log.Json, func(checked bool) {
			cfg.Log.Json = checked
		})

	// Create a layout where half of the available space is given to the left form and the other half to the right form.
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
			updateConfig(cfg, configFile)
		}
		return event
	})

	// Open the application
	if err := app.SetRoot(flex, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}

func formatPortNumber(portNumber uint) string {
	return strconv.FormatUint(uint64(portNumber), 10)
}

func isPortNumber(input string, _ rune) bool {
	_, err := parsePortNumber(input)
	return err == nil
}

func parsePortNumber(input string) (uint, error) {
	val, err := strconv.ParseUint(input, 10, 16) // Maximum port number is 2^16 - 1
	return uint(val), err
}

// updateConfig updates the configuration values from an instance of the config struct.
func updateConfig(cfg *config.Juno, configFile string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(configFile, data, os.ModePerm)
}
