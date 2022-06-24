/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"io/ioutil"
	"log"
	"path/filepath"
	"strconv"

	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/errpkg"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

var configfilename string

// updateConfigCmd represents the updateConfig command
var updateConfigCmd = &cobra.Command{
	Use:   "update_config",
	Short: "Update the fields of the config file",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		yamlFile, err := ioutil.ReadFile(configfilename)
		if err != nil {
			log.Printf("Can't get the file %v cos of %v", configfilename, err)
		}
		var c config.Config
		err = yaml.Unmarshal(yamlFile, &c)
		if err != nil {
			errpkg.CheckFatal(err, "Failed to read the config file")
		}
		flag, _ := cmd.Flags().GetString("rpcport")
		if flag {
			c, err = c.updateRPCPort(flag)
			if err != nil {
				errpkg.CheckFatal(err, "Failed to update the RPC port")
			}
		}
		flag, _ = cmd.Flags().GetString("rpcenable")
		if flag {
			c, err = c.updateRPCEnable(flag)
			if err != nil {
				errpkg.CheckFatal(err, "Failed to update the RPC enable")
			}
		}
		flag, _ = cmd.Flags().GetString("restport")
		if flag {
			c, err = c.updateRESTPort(flag)
			if err != nil {
				errpkg.CheckFatal(err, "Failed to update the REST port")
			}
		}
		flag, _ = cmd.Flags().GetString("restenable")
		if flag {
			c, err = c.updateRESTEnable(flag)
			if err != nil {
				errpkg.CheckFatal(err, "Failed to update the REST enable")
			}
		}
		flag, _ = cmd.Flags().GetString("restprefix")
		if flag {
			c, err = c.updateRESTPrefix(flag)
			if err != nil {
				errpkg.CheckFatal(err, "Failed to update the REST prefix")
			}
		}
		flag, _ = cmd.Flags().GetString("metricsport")
		if flag {
			c, err = c.updateMetricsPort(flag)
			if err != nil {
				errpkg.CheckFatal(err, "Failed to update the metrics port")
			}
		}
		flag, _ = cmd.Flags().GetString("metricsenable")
		if flag {
			c, err = c.updateMetricsEnable(flag)
			if err != nil {
				errpkg.CheckFatal(err, "Failed to update the metrics enable")
			}
		}
		flag, _ = cmd.Flags().GetString("feedergateway")
		if flag {
			c, err = c.updateFeederGateway(flag)
			if err != nil {
				errpkg.CheckFatal(err, "Failed to update the feeder gateway")
			}
		}
		flag, _ = cmd.Flags().GetString("network")
		if flag {
			c, err = c.updateNetwork(flag)
			if err != nil {
				errpkg.CheckFatal(err, "Failed to update the network")
			}
		}
		flag, _ = cmd.Flags().GetString("starknetenable")
		if flag {
			c, err = c.updateStarknetEnable(flag)
			if err != nil {
				errpkg.CheckFatal(err, "Failed to update the Starknet enable")
			}
		}
		flag, _ = cmd.Flags().GetString("apisync")
		if flag {
			c, err = c.updateAPISync(flag)
			if err != nil {
				errpkg.CheckFatal(err, "Failed to update the API sync")
			}
		}
		flag, _ = cmd.Flags().GetString("ethereumnode")
		if flag {
			c, err = c.updateEthereumNode(flag)
			if err != nil {
				errpkg.CheckFatal(err, "Failed to update the ethereum node")
			}
		}
		data, err := yaml.Marshal(&c)
		if err != nil {
			errpkg.CheckFatal(err, "Failed to update the yaml")
		}
		err2 := ioutil.WriteFile(configfilename, data, 0o644)
		if err2 != nil {
			errpkg.CheckFatal(err, "Failed to write to the yaml file")
		}
	},
}

func (c config.Config) updateRPCPort(args string) (config.Config, error) {
	port, err := strconv.Atoi(args)
	if err != nil {
		log.Println("%v", err)
	}
	c.rpcConfig.Port = port
	return c, nil
}

func (c config.Config) updateRPCEnable(args string) (config.Config, error) {
	enabled, err := strconv.ParseBool(args)
	if err != nil {
		log.Println("%v", err)
	}
	c.rpcConfig.Enabled = enabled
	return c, nil
}

func (c config.Config) updateRESTPort(args string) (config.Config, error) {
	port, err := strconv.Atoi(args)
	if err != nil {
		log.Println("%v", err)
	}
	c.restConfig.Port = port
	return c, nil
}

func (c config.Config) updateRESTEnable(args string) (config.Config, error) {
	enabled, err := strconv.ParseBool(args)
	if err != nil {
		log.Println("%v", err)
	}
	c.restConfig.Enabled = enabled
	return c, nil
}

func (c config.Config) updateRESTPrefix(args string) (config.Config, error) {
	c.restConfig.Prefix = args
	return c, nil
}

func (c config.Config) updateMetricsPort(args string) (config.Config, error) {
	port, err := strconv.Atoi(args)
	if err != nil {
		log.Println("%v", err)
	}
	c.metricsConfig.Port = port
	return c, nil
}

func (c config.Config) updateMetricsEnable(args string) (config.Config, error) {
	enabled, err := strconv.ParseBool(args)
	if err != nil {
		log.Println("%v", err)
	}
	c.metricsConfig.Enabled = enabled
	return c, nil
}

func (c config.Config) updateFeederGateway(args string) (config.Config, error) {
	c.starknetConfig.feeder_gateway = args
	return c, nil
}

func (c config.Config) updateNetwork(args string) (config.Config, error) {
	c.starknetConfig.network = args
	return c, nil
}

func (c config.Config) updateStarknetEnable(args string) (config.Config, error) {
	enabled, err := strconv.ParseBool(args)
	if err != nil {
		log.Println("%v", err)
	}
	c.starknetConfig.Enabled = enabled
	return c, nil
}

func (c config.Config) updateAPISync(args string) (config.Config, error) {
	enabled, err := strconv.ParseBool(args)
	if err != nil {
		log.Println("%v", err)
	}
	c.starknetConfig.api_sync = enabled
	return c, nil
}

func (c config.Config) updateEthereumNode(args string) (config.Config, error) {
	c.ethereumConfig.node = args
	return c, nil
}

func init() {
	rootCmd.AddCommand(updateConfigCmd)
	// RPC
	updateConfigCmd.Flags().StringP("rpcport", "p", viper.GetString("RPCPORT"), "Set the RPC Port")
	updateConfigCmd.Flags().StringP("rpcenable", "P", viper.GetString("RPCENABLE"), "Set if you would like to enable the RPC")
	// Rest
	updateConfigCmd.Flags().StringP("restport", "r", viper.GetString("RESTPORT"), "Set the REST Port")
	updateConfigCmd.Flags().StringP("restenable", "R", viper.GetString("RESTENABLE"), "Set if you would like to enable the REST")
	updateConfigCmd.Flags().StringP("restprefix", "x", viper.GetString("RESTPREFIX"), "Set the REST prefix")
	//Metrics
	updateConfigCmd.Flags().StringP("metricsport", "m", viper.GetString("METRICSPORT"), "Set the port where you would like to see the metrics")
	updateConfigCmd.Flags().StringP("metricsenable", "M", viper.GetString("METRICSENABLE"), "Set if you would like to enable metrics")
	// Starknet
	updateConfigCmd.Flags().StringP("feedergateway", "s", viper.GetString("FEEDERGATEWAY"), "Set the link to the feeder gateway")
	updateConfigCmd.Flags().StringP("network", "n", viper.GetString("NETWORK"), "Set the network")
	updateConfigCmd.Flags().StringP("starknetenable", "S", viper.GetString("STARKNETENABLE"), "Set if you would like to enable calls from feeder gateway")
	updateConfigCmd.Flags().StringP("apisync", "A", viper.GetString("APISYNCENABLE"), "Set if you would like to enable api sync")
	// Ethereum
	updateConfigCmd.Flags().StringP("ethereumnode", "e", viper.GetString("ETHEREUMNODE"), "Set the ethereum node")
	configfilename = filepath.Join(config.Dir, "juno.yaml")
	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// updateConfigCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// updateConfigCmd.Flags().StringP("toggle", "t", false, "Help message for toggle")
}
