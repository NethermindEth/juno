package cli

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/NethermindEth/juno/internal/log"
	"gopkg.in/yaml.v2"
)

// RpcConfig represents the RPC configuration.
type RpcConfig struct {
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`
	Port    int  `yaml:"port" mapstructure:"port"`
}

// Config represents the project configuration.
type Config struct {
	Rpc    RpcConfig `yaml:"rpc" mapstructure:"rpc"`
	DbPath string    `yaml:"db_path" mapstructure:"db_path"`
}

const (
	CfgFileName = "config.yaml"
	ProjectDir  = ".juno"
	DbPath      = "data"
)

// NewConfig returns a reference to a cli.Config instance.
func NewConfig(path string) *Config {
	dbPath := filepath.Join(path, ProjectDir, DbPath)
	projectFolder := filepath.Join(path, ProjectDir)
	// Checks that project folder exists, else, create a new one.
	if _, err := os.Stat(projectFolder); os.IsNotExist(err) {
		err := os.MkdirAll(projectFolder, 0755)
		if err != nil {
			return nil
		}
	}
	return &Config{Rpc: RpcConfig{Enabled: true, Port: 8080}, DbPath: dbPath}
}

// DefaultConfig generates a default configuration file.
func DefaultConfig(path string) (err error) {
	finalPath := filepath.Join(path, ProjectDir, CfgFileName)
	log.Default.With("Path", finalPath).Info("Generating configuration")

	// Generate configuration file from scratch
	config := NewConfig(path)
	log.Default.With("Path", finalPath).Info("Config file Generated")

	// TODO: handle this error. 
	yamlData, _ := yaml.Marshal(config)

	// Save in yaml format config file
	err = ioutil.WriteFile(finalPath, yamlData, 0644)
	if err != nil {
		log.Default.With("Error", err).Panic("Unable to write data into the file")
	}
	return nil
}
