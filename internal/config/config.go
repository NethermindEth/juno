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

// Log represents the logger configuration
type Log struct {
	Level   string `yaml:"level" mapstructure:"level"`
	Json    bool   `yaml:"json" mapstructure:"json"`
	NoColor bool   `yaml:"nocolor" mapstructure:"nocolor"`
}

// Rpc represents the juno RPC configuration.
type Rpc struct {
	Enable bool `yaml:"enable" mapstructure:"enable"`
	Port   uint `yaml:"port" mapstructure:"port"`
}

// Metrics represents the Prometheus Metrics configuration.
type Metrics struct {
	Enable bool `yaml:"enable" mapstructure:"enable"`
	Port   uint `yaml:"port" mapstructure:"port"`
}

// Rest represents the juno REST configuration.
type Rest struct {
	Enable bool   `yaml:"enable" mapstructure:"enable"`
	Port   uint   `yaml:"port" mapstructure:"port"`
	Prefix string `yaml:"prefix" mapstructure:"prefix"`
}

// Database represents the juno database configuration.
type Database struct {
	Path string `yaml:"path" mapstructure:"path"`
}

// Sync represents the juno StarkNet configuration.
type Sync struct {
	Enable    bool   `yaml:"enable" mapstructure:"enable"`
	Sequencer string `yaml:"sequencer" mapstructure:"sequencer"`
	Network   string `yaml:"network" mapstructure:"network"`
	Trusted   bool   `yaml:"trusted" mapstructure:"trusted"`
	EthNode   string `yaml:"ethnode" mapstructure:"ethnode"`
}

// Juno is the top-level juno configuration.
type Juno struct {
	Log      Log      `yaml:"log" mapstructure:"log"`
	Rpc      Rpc      `yaml:"rpc" mapstructure:"rpc"`
	Metrics  Metrics  `yaml:"metrics" mapstructure:"metrics"`
	Rest     Rest     `yaml:"rest" mapstructure:"rest"`
	Database Database `yaml:"database" mapstructure:"database"`
	Sync     Sync     `yaml:"sync" mapstructure:"sync"`
}

// UserDataDir finds the user's default data directory, returning the
// empty string and an error otherwise.
//
// It is analagous to os.UserConfigDir.
func UserDataDir() (string, error) {
	// notest
	const junoDir = "juno"

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
				if err := os.Mkdir(result, os.ModePerm); err != nil {
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
