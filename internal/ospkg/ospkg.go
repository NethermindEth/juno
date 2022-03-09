// Package ospkg provides constants relating to system key system
// directories across different operating systems.
package ospkg

import (
	"errors"
	"os"
	"runtime"

	"github.com/NethermindEth/juno/internal/errpkg"
)

var (
	// ConfigDir is the default root directory for user-specific
	// configuration data.
	//
	// On Darwin this is $HOME/Library/Application Support, on other Unix
	// systems it is $XDG_CONFIG_HOME, and on Windows, %APPDATA%.
	ConfigDir,
	// DataDir is the is the default root directory for user-specific
	// application data.
	//
	// On Unix this is $XDG_DATA_HOME and on Windows, %APPDATA%.
	DataDir,
	// HomeDir is the current user's home directory.
	//
	// On Unix this is $HOME and on Windows, %USERPROFILE%.
	HomeDir string
)

func init() {
	var err error = nil
	ConfigDir, err = os.UserConfigDir()
	errpkg.CheckFatal(err, "unable to get the user config directory")
	HomeDir, err = os.UserHomeDir()
	errpkg.CheckFatal(err, "unable to get the user home directory")
	DataDir, err = func() (string, error) {
		switch runtime.GOOS {
		case "windows":
			// On Windows ConfigDir and DataDir share the same path. See:
			// https://stackoverflow.com/questions/43853548/xdg-basedir-directories-for-windows
			return ConfigDir, nil
		case "darwin", "dragonfly", "freebsd", "illumos", "ios", "linux", "netbsd",
			"openbsd", "solaris":
			return "/usr/local/share/juno/", nil
		default: // js/wasm, plan9
			return "", errors.New("user data directory not found")
		}
	}()
	errpkg.CheckFatal(err, "unable to get the user data directory")
}
