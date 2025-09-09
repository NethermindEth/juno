package core_test

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBlockVersion(t *testing.T) {
	versions := []struct {
		version  string
		expected *semver.Version
	}{
		{"0.13.1", semver.MustParse("0.13.1")},
		{"0.13.1.1", semver.MustParse("0.13.1")},
		{"0.14", semver.MustParse("0.14.0")},
		{"14", semver.MustParse("14.0.0")},
	}

	for _, test := range versions {
		t.Run("block version: "+test.version, func(t *testing.T) {
			version, err := core.ParseBlockVersion(test.version)
			require.Nil(t, err)
			assert.Equal(t, test.expected, version)
		})
	}
}

func TestCannotParseBlockVersion(t *testing.T) {
	versions := []string{
		"1.4.2-alpha.3+20250908.1",
	}

	for _, version := range versions {
		t.Run("block version: "+version, func(t *testing.T) {
			version, err := core.ParseBlockVersion(version)
			require.Nil(t, version)
			assert.ErrorContains(t, err, "cannot parse starknet protocol version")
		})
	}
}
