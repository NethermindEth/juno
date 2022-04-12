// Package config provides constants and functions related to the
// application's configuration.
package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"

	"github.com/NethermindEth/juno/internal/errpkg"
	"github.com/NethermindEth/juno/internal/log"
	"gopkg.in/yaml.v2"
)

// rpcConfig represents the juno RPC configuration.
type rpcConfig struct {
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`
	Port    int  `yaml:"port" mapstructure:"port"`
}

// ethereumConfig represents the juno Ethereum configuration.
type ethereumConfig struct {
	Enabled bool   `yaml:"enabled" mapstructure:"enabled"`
	Node    string `yaml:"node" mapstructure:"node"`
}

type contractAbiConfig struct {
	StarknetAbiPath    string `yaml:"starknet"  mapstructure:"starknet"`
	GpsVerifierAbiPath string `yaml:"gps_verifier" mapstructure:"gps_verifier"`
	MemoryPageAbiPath  string `yaml:"memory_page" mapstructure:"memory_page"`
}

// starknetConfig represents the juno StarkNet configuration.
type starknetConfig struct {
	Enabled                        bool              `yaml:"enabled" mapstructure:"enabled"`
	FeederGateway                  string            `yaml:"feeder_gateway" mapstructure:"feeder_gateway"`
	ContractAbiPathConfig          contractAbiConfig `yaml:"contract_abi_path" mapstructure:"contract_abi_path"`
	MemoryPageFactRegistryContract string            `yaml:"memory_contract" mapstructure:"memory_contract"`
}

// Config represents the juno configuration.
type Config struct {
	DbPath   string         `yaml:"db_path" mapstructure:"db_path"`
	Ethereum ethereumConfig `yaml:"ethereum" mapstructure:"ethereum"`
	Starknet starknetConfig `yaml:"starknet" mapstructure:"starknet"`
	RPC      rpcConfig      `yaml:"rpc" mapstructure:"rpc"`
}

var (
	// Dir is the default root directory for user-specific
	// configuration data.
	//
	// On Darwin this is $HOME/Library/Application Support/juno/, on other
	// Unix systems $XDG_CONFIG_HOME/juno/, and on Windows,
	// %APPDATA%/juno.
	Dir,
	// DataDir is the is the default root directory for user-specific
	// application data.
	//
	// On Unix this is $XDG_DATA_HOME/juno/ and on Windows,
	// %APPDATA%/juno/.
	DataDir string
)

// Runtime is the runtime configuration of the application.
var Runtime *Config

const goerliStarknetGateway = "http://alpha4.starknet.io"

const goerli = "http://alpha4.starknet.io"

func init() {
	// Set user config directory.
	d, err := os.UserConfigDir()
	errpkg.CheckFatal(err, "Unable to get the user config directory.")
	Dir = filepath.Join(d, "juno")

	// Set user data directory.
	DataDir, err = func() (string, error) {
		// notest
		switch runtime.GOOS {
		case "windows":
			// On Windows ConfigDir and DataDir share the same path. See:
			// https://stackoverflow.com/questions/43853548/xdg-basedir-directories-for-windows.
			return Dir, nil
		case "darwin", "dragonfly", "freebsd", "illumos", "ios", "linux", "netbsd",
			"openbsd", "solaris":
			return "/usr/local/share/juno/", nil
		default: // js/wasm, plan9
			return "", errors.New("user data directory not found")
		}
	}()
	errpkg.CheckFatal(err, "Unable to get user data directory.")
}

// New creates a new configuration file with default values.
func New() {
	f := filepath.Join(Dir, "juno.yaml")
	log.Default.With("Path", f).Info("Creating default config.")
	// Create the juno configuration directory if it does not exist.
	if _, err := os.Stat(Dir); os.IsNotExist(err) {
		// notest
		err := os.MkdirAll(Dir, 0755)
		errpkg.CheckFatal(err, "Failed to create Config directory.")
	}
	data, err := yaml.Marshal(&Config{
		Ethereum: ethereumConfig{Enabled: false},
		Starknet: starknetConfig{Enabled: false},
		RPC:      rpcConfig{Enabled: false, Port: 8080},
		DbPath:   Dir,
	})
	errpkg.CheckFatal(err, "Failed to marshal Config instance to byte data.")
	err = os.WriteFile(f, data, 0644)
	errpkg.CheckFatal(err, "Failed to write config file.")
}
