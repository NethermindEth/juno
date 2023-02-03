package utils_test

import (
	"testing"

	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

func TestDataDir(t *testing.T) {
	tests := []struct {
		os          string
		userDataDir string
		userHomeDir string
		expectedDir string
	}{
		{"", "noos/dataDir", "noos/homeDir", ""},
		{"", "noos/dataDir", "", ""},
		{"", "", "noos/homeDir", ""},
		{"", "", "", ""},

		{"unknown", "unknown/dataDir", "unknown/homeDir", "unknown/dataDir/juno"},
		{"unknown", "unknown/dataDir", "", "unknown/dataDir/juno"},
		{"unknown", "", "unknown/homeDir", "unknown/homeDir/.local/share/juno"},
		{"unknown", "", "", ""},

		{"windows", "windows/dataDir", "windows/homeDir", "windows/dataDir/juno"},
		{"windows", "windows/dataDir", "", "windows/dataDir/juno"},
		{"windows", "", "windows/homeDir", ""},
		{"windows", "", "", ""},

		{"dragonfly", "dragonfly/dataDir", "dragonfly/homeDir", "dragonfly/dataDir/juno"},
		{"dragonfly", "dragonfly/dataDir", "", "dragonfly/dataDir/juno"},
		{"dragonfly", "", "dragonfly/homeDir", "dragonfly/homeDir/.local/share/juno"},
		{"dragonfly", "", "", ""},

		{"freebsd", "freebsd/dataDir", "freebsd/homeDir", "freebsd/dataDir/juno"},
		{"freebsd", "freebsd/dataDir", "", "freebsd/dataDir/juno"},
		{"freebsd", "", "freebsd/homeDir", "freebsd/homeDir/.local/share/juno"},
		{"freebsd", "", "", ""},

		{"illumos", "illumos/dataDir", "illumos/homeDir", "illumos/dataDir/juno"},
		{"illumos", "illumos/dataDir", "", "illumos/dataDir/juno"},
		{"illumos", "", "illumos/homeDir", "illumos/homeDir/.local/share/juno"},
		{"illumos", "", "", ""},

		{"ios", "ios/dataDir", "ios/homeDir", "ios/dataDir/juno"},
		{"ios", "ios/dataDir", "", "ios/dataDir/juno"},
		{"ios", "", "ios/homeDir", "ios/homeDir/.local/share/juno"},
		{"ios", "", "", ""},

		{"linux", "linux/dataDir", "linux/homeDir", "linux/dataDir/juno"},
		{"linux", "linux/dataDir", "", "linux/dataDir/juno"},
		{"linux", "", "linux/homeDir", "linux/homeDir/.local/share/juno"},
		{"linux", "", "", ""},

		{"netbsd", "netbsd/dataDir", "netbsd/homeDir", "netbsd/dataDir/juno"},
		{"netbsd", "netbsd/dataDir", "", "netbsd/dataDir/juno"},
		{"netbsd", "", "netbsd/homeDir", "netbsd/homeDir/.local/share/juno"},
		{"netbsd", "", "", ""},

		{"openbsd", "openbsd/dataDir", "openbsd/homeDir", "openbsd/dataDir/juno"},
		{"openbsd", "openbsd/dataDir", "", "openbsd/dataDir/juno"},
		{"openbsd", "", "openbsd/homeDir", "openbsd/homeDir/.local/share/juno"},
		{"openbsd", "", "", ""},

		{"solaris", "solaris/dataDir", "solaris/homeDir", "solaris/dataDir/juno"},
		{"solaris", "solaris/dataDir", "", "solaris/dataDir/juno"},
		{"solaris", "", "solaris/homeDir", "solaris/homeDir/.local/share/juno"},
		{"solaris", "", "", ""},

		{"plan9", "plan9/dataDir", "plan9/homeDir", "plan9/dataDir/juno"},
		{"plan9", "plan9/dataDir", "", "plan9/dataDir/juno"},
		{"plan9", "", "plan9/homeDir", "plan9/homeDir/.local/share/juno"},
		{"plan9", "", "", ""},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.expectedDir, utils.DataDir(tc.os, tc.userDataDir, tc.userHomeDir))
	}
}
