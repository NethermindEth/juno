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

type Network uint8

const (
	GOERLI Network = iota
	MAINNET
)

func (n Network) String() string {
	switch n {
	case GOERLI:
		return "goerli"
	case MAINNET:
		return "mainnet"
	default:
		return ""
	}
}

// Juno is the top-level juno configuration.
type Juno struct {
	Verbosity    string  `mapstructure:"verbosity"`
	RpcPort      uint16  `mapstructure:"rpc-port"`
	MetricsPort  uint16  `mapstructure:"metrics-port"`
	DatabasePath string  `mapstructure:"db-path"`
	Network      Network `mapstructure:"network"`
	EthNode      string  `mapstructure:"eth-node"`
}

// UserDataDir finds the user's default data directory, returning the
// empty string an error otherwise.
//
// It is analogous to os.UserConfigDir.
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
