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

// Config represents the juno configuration.
type Config struct {
	Rpc    rpcConfig `yaml:"rpc" mapstructure:"rpc"`
	DbPath string    `yaml:"db_path" mapstructure:"db_path"`
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
			// https://stackoverflow.com/questions/43853548/xdg-basedir-directories-for-windows
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
		Rpc:    rpcConfig{Enabled: false, Port: 8080},
		DbPath: Dir,
	})
	errpkg.CheckFatal(err, "Failed to marshal Config instance to byte data.")
	err = os.WriteFile(f, data, 0644)
	errpkg.CheckFatal(err, "Failed to write config file.")
}
