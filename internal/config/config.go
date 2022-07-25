// Package config provides constants and functions related to the
// application's configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// logConfig represents the logger configuration
type logConfig struct {
	Level string `yaml:"level" mapstructure:"level"`
	Json  bool   `yaml:"json" mapstructure:"json"`
}

// rpcConfig represents the juno RPC configuration.
type rpcConfig struct {
	Enable bool `yaml:"enable" mapstructure:"enable"`
	Port   uint `yaml:"port" mapstructure:"port"`
}

// metricsConfig represents the Prometheus Metrics configuration.
type metricsConfig struct {
	Enable bool `yaml:"enable" mapstructure:"enable"`
	Port   uint `yaml:"port" mapstructure:"port"`
}

// ethConfig represents the juno Ethereum configuration.
type ethereumConfig struct {
	Node string `yaml:"node" mapstructure:"node"`
}

// restConfig represents the juno REST configuration.
type restConfig struct {
	Enable bool   `yaml:"enable" mapstructure:"enable"`
	Port   uint   `yaml:"port" mapstructure:"port"`
	Prefix string `yaml:"prefix" mapstructure:"prefix"`
}

// databaseConfig represents the juno database configuration.
type databaseConfig struct {
	Name string `yaml:"name" mapstructure:"name"`
	Path string `yaml:"path" mapstructure:"path"`
}

// starknetConfig represents the juno StarkNet configuration.
type starknetConfig struct {
	Enable    bool   `yaml:"enable" mapstructure:"enable"`
	Sequencer string `yaml:"sequencer" mapstructure:"sequencer"`
	Network   string `yaml:"network" mapstructure:"network"`
	ApiSync   bool   `yaml:"api_sync" mapstructure:"api_sync"`
}

// Config is the top-level juno configuration.
type Config struct {
	Log      logConfig      `yaml:"log" mapstructure:"log"`
	Ethereum ethereumConfig `yaml:"ethereum" mapstructure:"ethereum"`
	RPC      rpcConfig      `yaml:"rpc" mapstructure:"rpc"`
	Metrics  metricsConfig  `yaml:"metrics" mapstructure:"metrics"`
	REST     restConfig     `yaml:"rest" mapstructure:"rest"`
	Database databaseConfig `yaml:"database" mapstructure:"database"`
	Starknet starknetConfig `yaml:"starknet" mapstructure:"starknet"`
}

// UserDataDir finds the user's default data directory, returning the
// empty string and error otherwise.
// It is analagous to os.UserConfigDir.
func UserDataDir() (string, error) {
	// notest
	junoDir := "juno"

	// notest
	switch runtime.GOOS {
	case "windows":
		// On Windows ConfigDir and DataDir share the same path. See:
		// https://stackoverflow.com/questions/43853548/xdg-basedir-directories-for-windows.
		configDir, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("user data directory not found: %w", err)
		}
		return filepath.Join(configDir, junoDir), nil
	case "darwin", "dragonfly", "freebsd", "illumos", "ios", "linux", "netbsd",
		"openbsd", "solaris":
		// Use XDG_DATA_HOME. If it is not configured, try $HOME/.local/share
		// See https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html
		dataHome := os.Getenv("XDG_DATA_HOME")
		if dataHome == "" {
			home := os.Getenv("HOME")
			if home == "" {
				return "", errors.New("could not find database directory: home directory not found")
			}
			result := filepath.Join(home, ".local", "share", junoDir)
			// Create Juno data directory if it does not exist
			if _, err := os.Stat(result); err != nil {
				if err := os.Mkdir(result, os.ModeDir); err != nil {
					return "", fmt.Errorf("could not create data directory %s: %w", result, err)
				}
			}
			return result, nil
		}
		return filepath.Join(dataHome, junoDir), nil
	default: // js/wasm, plan9
		return "", errors.New("user data directory not found")
	}
}
