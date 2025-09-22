package core_test

import (
	"strconv"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils"
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

func TestSupportedBlockVersion(t *testing.T) {
	testVersions := []struct {
		block  semver.Version
		latest semver.Version
	}{
		{
			// Block and latest are the same version (e.g., 0.14.0 == 0.14.0)
			block:  *core.LatestVer,
			latest: *core.LatestVer,
		},
		{
			// Block is newer patch than latest (e.g., 0.14.1 > 0.14.0)
			block:  core.LatestVer.IncPatch(),
			latest: *core.LatestVer,
		},
		{
			// Block is older patch than latest (e.g., 0.14.0 < 0.14.1)
			block:  *core.LatestVer,
			latest: core.LatestVer.IncPatch(),
		},
		{
			// Block is older minor than latest (e.g., 0.14.0 < 0.15.0)
			block:  *core.LatestVer,
			latest: core.LatestVer.IncMinor(),
		},
		{
			// Block is older major than latest (e.g., 0.14.0 < 1.0.0)
			block:  *core.LatestVer,
			latest: core.LatestVer.IncMajor(),
		},
	}

	latestVarTemp := core.LatestVer

	for i, test := range testVersions {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			core.LatestVer = &test.latest
			err := core.CheckBlockVersion(utils.HeapPtr(test.block))
			assert.NoError(t, err)
		})
	}

	core.LatestVer = latestVarTemp
}

func TestUnsupportedBlockVersion(t *testing.T) {
	testVersions := []struct {
		block  semver.Version
		latest semver.Version
	}{
		{
			// Block is newer major than latest (e.g., 1.0.0 > 0.14.0)
			block:  core.LatestVer.IncMajor(),
			latest: *core.LatestVer,
		},
		{
			// Block is newer minor than latest (e.g., 0.15.0 > 0.14.0)
			block:  core.LatestVer.IncMinor(),
			latest: *core.LatestVer,
		},
	}

	for i, test := range testVersions {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			err := core.CheckBlockVersion(utils.HeapPtr(test.block))
			assert.ErrorContains(t, err, "unsupported block version")
		})
	}
}
