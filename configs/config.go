package configs

import (
	"github.com/NethermindEth/juno/internal/log"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
)

const (
	ConfigFileName                 = "config.yaml"
	ConfigFileNameWithoutExtension = "config"

	ProjectFolderName = ".juno"
	DatabasePath      = "data"
)

// RpcConfiguration represent the RPC configuration
type RpcConfiguration struct {
	RpcEnabled bool `yaml:"enabled" mapstructure:"enabled"`
	RpcPort    int  `yaml:"port" mapstructure:"port"`
}

// Configuration represent project configuration
type Configuration struct {
	RpcConfiguration RpcConfiguration `yaml:"rpc" mapstructure:"rpc"`
	DatabasePath     string           `yaml:"db_path" mapstructure:"db_path"`
}

// NewConfiguration returns a valid configuration struct for project
func NewConfiguration(path string) *Configuration {
	dbPath := filepath.Join(path, ProjectFolderName, DatabasePath)
	projectFolder := filepath.Join(path, ProjectFolderName)
	// Checks that project folder exists, else, create a new one
	if _, err := os.Stat(projectFolder); os.IsNotExist(err) {
		err := os.MkdirAll(projectFolder, 0755)
		if err != nil {
			return nil
		}
	}
	return &Configuration{
		RpcConfiguration: RpcConfiguration{
			RpcEnabled: false,
			RpcPort:    8080,
		},
		DatabasePath: dbPath,
	}
}

var logger = log.GetLogger()

// Generate This function generate the default configuration file.
func Generate(path string) (err error) {
	finalPath := filepath.Join(path, ProjectFolderName, ConfigFileName)
	logger.With("Path", finalPath).Info("Generating configuration")

	// Generate configuration file from scratch
	config := NewConfiguration(path)
	logger.With("Path", finalPath).Info("Config file Generated")

	yamlData, err := yaml.Marshal(config)
	// Save in yaml format config file
	err = ioutil.WriteFile(finalPath, yamlData, 0644)
	if err != nil {
		logger.With("Error", err).Panic("Unable to write data into the file")
	}
	return nil
}
