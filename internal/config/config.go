// Package config provides constants and functions related to the
// application's configuration.
package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"

	. "github.com/NethermindEth/juno/internal/log"
	"gopkg.in/yaml.v2"
)

// loggerConfig represents the logger configuration
type loggerConfig struct {
	VerbosityLevel   string `yaml:"verbosity_level" mapstructure:"verbosity_level"`
	EnableJsonOutput bool   `yaml:"enable_json_output" mapstructure:"enable_json_output"`
}

// rpcConfig represents the juno RPC configuration.
type rpcConfig struct {
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`
	Port    int  `yaml:"port" mapstructure:"port"`
}

// metricsConfig represents the Prometheus Metrics configuration.
type metricsConfig struct {
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`
	Port    int  `yaml:"port" mapstructure:"port"`
}

// ethereumConfig represents the juno Ethereum configuration.
type ethereumConfig struct {
	Node string `yaml:"node" mapstructure:"node"`
}

// restConfig represents the juno REST configuration.
type restConfig struct {
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`
	Port    int  `yaml:"port" mapstructure:"port"`
	// TODO: Does this need to be configured?
	// Prefix  string `yaml:"prefix" mapstructure:"prefix"`
}

// starknetConfig represents the juno StarkNet configuration.
type starknetConfig struct {
	Enabled       bool   `yaml:"enabled" mapstructure:"enabled"`
	FeederGateway string `yaml:"feeder_gateway" mapstructure:"feeder_gateway"`
	Network       string `yaml:"network" mapstructure:"network"`
	ApiSync       bool   `yaml:"api_sync" mapstructure:"api_sync"`
}

// Config represents the juno configuration.
type Config struct {
	Logger   loggerConfig   `yaml:"logger" mapstructure:"logger"`
	Ethereum ethereumConfig `yaml:"ethereum" mapstructure:"ethereum"`
	RPC      rpcConfig      `yaml:"rpc" mapstructure:"rpc"`
	Metrics  metricsConfig  `yaml:"metrics" mapstructure:"metrics"`
	REST     restConfig     `yaml:"rest" mapstructure:"rest"`
	DbPath   string         `yaml:"db_path" mapstructure:"db_path"`
	Starknet starknetConfig `yaml:"starknet" mapstructure:"starknet"`
}

var (
	// Dir is the default root directory for user-specific
	// configuration data.
	//
	// On Darwin this is $HOME/Library/Application Support/juno/, on other
	// Unix systems $XDG_CONFIG_HOME/juno/, and on Windows,
	// %APPDATA%/juno.
	Dir string
	// DataDir is the is the default root directory for user-specific
	// application data.
	//
	// On Unix this is $XDG_DATA_HOME/juno/ and on Windows,
	// %APPDATA%/juno/.
	DataDir string
)

// Runtime is the runtime configuration of the application.
var Runtime *Config

func init() {
	// Set user config directory.
	// notest
	d, err := os.UserConfigDir()
	if err != nil {
		Logger.Fatal("Could not get config directory.")
	}
	Dir = filepath.Join(d, "juno")

	// Set user data directory.
	// notest
	DataDir, err = func() (string, error) {
		// notest
		switch runtime.GOOS {
		case "windows":
			// On Windows ConfigDir and DataDir share the same path. See:
			// https://stackoverflow.com/questions/43853548/xdg-basedir-directories-for-windows.
			return Dir, nil
		case "darwin", "dragonfly", "freebsd", "illumos", "ios", "linux", "netbsd",
			"openbsd", "solaris":
			// Use XDG_DATA_HOME. If it is not configured, try $HOME/.local/share
			// See https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html
			dataHome := os.Getenv("XDG_DATA_HOME")
			if dataHome == "" {
				home := os.Getenv("HOME")
				if home == "" {
					return "", errors.New("user home directory not found")
				}
				result := filepath.Join(home, ".local", "share", "juno")
				// Create Juno data directory if it does not exist
				if _, err := os.Stat(result); errors.Is(err, os.ErrNotExist) {
					err = os.MkdirAll(result, 0o744)
					if err != nil {
						Logger.Fatal("Could not create user data directory.")
					}
				}
				return result, nil
			}
			return filepath.Join(dataHome, "juno"), nil
		default: // js/wasm, plan9
			return "", errors.New("user data directory not found")
		}
	}()
	// If there is an error
	if err != nil {
		Logger.Fatal("Could not get user data directory.")
	}
}

// New creates a new configuration file with default values.
func New() {
	f := filepath.Join(Dir, "juno.yaml")
	Logger.With("Path", f).Info("Creating default config.")
	// Create the juno configuration directory if it does not exist.
	if _, err := os.Stat(Dir); os.IsNotExist(err) {
		// notest
		err := os.MkdirAll(Dir, 0o755)
		if err != nil {
			Logger.Fatal("Could not create config directory.")
		}
	}
	// notest
	data, err := yaml.Marshal(&Config{
		Logger: loggerConfig{
			VerbosityLevel:   "debug",
			EnableJsonOutput: false,
		},
		Ethereum: ethereumConfig{Node: ""},
		RPC:      rpcConfig{Enabled: true, Port: 8080},
		Metrics:  metricsConfig{Enabled: true, Port: 2048},
		DbPath:   DataDir,
		REST:     restConfig{Enabled: true, Port: 8100 /*, Prefix: "/feeder_gateway" */},
		Starknet: starknetConfig{
			Enabled: true, ApiSync: true, FeederGateway: "https://alpha-mainnet.starknet.io",
			Network: "mainnet",
		},
	})
	if err != nil {
		Logger.Fatal("Could not marshal default config.")
	}
	// Create default Juno configuration file if it does not exist
	if _, err := os.Stat(f); errors.Is(err, os.ErrNotExist) {
		// notest
		err = os.WriteFile(f, data, 0o644)
		if err != nil {
			Logger.Fatal("Could not create default config file.")
		}
	}
}

// Exists checks if the default configuration file already exists
func Exists() bool {
	f := filepath.Join(Dir, "juno.yaml")
	_, err := os.Stat(f)
	return err == nil
}
