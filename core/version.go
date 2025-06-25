package core

import (
	"strings"

	"github.com/Masterminds/semver/v3"
)

var (
	Ver0_13_2 = semver.MustParse("0.13.2")
	Ver0_13_4 = semver.MustParse("0.13.4")
	Ver0_14_0 = semver.MustParse("0.14.0")
)

// ParseBlockVersion computes the block version, defaulting to "0.0.0" for empty strings
func ParseBlockVersion(protocolVersion string) (*semver.Version, error) {
	if protocolVersion == "" {
		return semver.NewVersion("0.0.0")
	}

	sep := "."
	digits := strings.Split(protocolVersion, sep)
	// pad with 3 zeros in case version has less than 3 digits
	digits = append(digits, []string{"0", "0", "0"}...)

	// get first 3 digits only
	return semver.NewVersion(strings.Join(digits[:3], sep))
}
