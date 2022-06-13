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
	Node string `yaml:"node" mapstructure:"node"`
}

// restConfig represents the juno REST configuration.
type restConfig struct {
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`
	Port    int  `yaml:"port" mapstructure:"port"`
}

// starknetConfig represents the juno StarkNet configuration.
type starknetConfig struct {
	Enabled       bool   `yaml:"enabled" mapstructure:"enabled"`
	FeederGateway string `yaml:"feeder_gateway" mapstructure:"feeder_gateway"`
	ApiSync       bool   `yaml:"api_sync" mapstructure:"api_sync"`
}

// Config represents the juno configuration.
type Config struct {
	Ethereum ethereumConfig `yaml:"ethereum" mapstructure:"ethereum"`
	RPC      rpcConfig      `yaml:"rpc" mapstructure:"rpc"`
	REST     restConfig     `yaml:"rest" mapstructure:"rest"`
	DbPath   string         `yaml:"db_path" mapstructure:"db_path"`
	Network  string         `yaml:"starknet_network" mapstructure:"starknet_network"`
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
					err = os.Mkdir(result, 0o744)
					errpkg.CheckFatal(err, "Unable to create user data directory.")
				}
				return result, nil
			}
			return filepath.Join(dataHome, "juno"), nil
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
		err := os.MkdirAll(Dir, 0o755)
		errpkg.CheckFatal(err, "Failed to create Config directory.")
	}
	data, err := yaml.Marshal(&Config{
		Ethereum: ethereumConfig{Node: "your_node_here"},
		RPC:      rpcConfig{Enabled: false, Port: 8080},
		REST:     restConfig{Enabled: false, Port: 8100},
		DbPath:   DataDir,
		Network:  goerli,
		Starknet: starknetConfig{Enabled: true, ApiSync: true, FeederGateway: "https://alpha-mainnet.starknet.io"},
	})
	errpkg.CheckFatal(err, "Failed to marshal Config instance to byte data.")
	// Create default Juno configuration file if it does not exist
	if _, err := os.Stat(f); errors.Is(err, os.ErrNotExist) {
		err = os.WriteFile(f, data, 0o644)
		errpkg.CheckFatal(err, "Failed to write config file.")
	}
}

// Exists checks if the default configuration file already exists
func Exists() bool {
	f := filepath.Join(Dir, "juno.yaml")
	_, err := os.Stat(f)
	return err == nil
}
