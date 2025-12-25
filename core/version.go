package core

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
)

var (
	Ver0_13_2 = semver.MustParse("0.13.2")
	Ver0_13_4 = semver.MustParse("0.13.4")
	Ver0_14_0 = semver.MustParse("0.14.0")
	Ver0_14_1 = semver.MustParse("0.14.1")
	LatestVer = Ver0_14_1
)

// ParseBlockVersion computes the block version, defaulting to "0.0.0" for empty strings
func ParseBlockVersion(protocolVersion string) (*semver.Version, error) {
	if protocolVersion == "" {
		return semver.New(0, 0, 0, "", ""), nil
	}

	const sep = "."
	parts := strings.Split(protocolVersion, sep)

	var versionVals [3]uint64 // [major, minor, patch]
	var err error

	for i := range min(len(versionVals), len(parts)) {
		versionVals[i], err = strconv.ParseUint(parts[i], 10, 64) // #nosec G602 // False positive.
		if err != nil {
			return nil,
				fmt.Errorf("cannot parse starknet protocol version \"%s\": %s", protocolVersion, err)
		}
	}

	return semver.New(versionVals[0], versionVals[1], versionVals[2], "", ""), nil // #nosec G602
}

// CheckBlockVersion checks if the block protocol version is supported by Juno
func CheckBlockVersion(protocolVersion string) error {
	blockVer, err := ParseBlockVersion(protocolVersion)
	if err != nil {
		return err
	}

	latestSupportedVersion := LatestVer

	supported := blockVer.Major() < latestSupportedVersion.Major() ||
		(blockVer.Major() == latestSupportedVersion.Major() &&
			blockVer.Minor() <= latestSupportedVersion.Minor())
	if supported {
		return nil
	}

	return fmt.Errorf(
		"unsupported block version %s. Maximum supported is: %s",
		protocolVersion,
		latestSupportedVersion.String(),
	)
}
