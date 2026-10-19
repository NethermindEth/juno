package main

import (
	"fmt"
	"regexp"
)

var versionPattern = regexp.MustCompile(
	`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-(rc|beta|alpha)\.(0|[1-9][0-9]*))?$`,
)

// releaseVersion is a release tag such as v0.16.9 or v0.16.9-rc.1.
type releaseVersion struct {
	tag        string
	core       string // tag without the pre-release suffix
	prerelease bool
}

// parseVersion parses a release tag in the form vX.Y.Z or vX.Y.Z-(rc|beta|alpha).N.
func parseVersion(tag string) (releaseVersion, error) {
	m := versionPattern.FindStringSubmatch(tag)
	if m == nil {
		return releaseVersion{}, fmt.Errorf(
			"invalid version %q: want vX.Y.Z or vX.Y.Z-(rc|beta|alpha).N", tag)
	}
	return releaseVersion{
		tag:        tag,
		core:       fmt.Sprintf("v%s.%s.%s", m[1], m[2], m[3]),
		prerelease: m[4] != "",
	}, nil
}

// checkRef returns an error unless ref is main or the release branch of v.
func checkRef(ref string, v releaseVersion) error {
	if ref == "main" || ref == "release/"+v.core {
		return nil
	}
	return fmt.Errorf("ref %q cannot hold %s: want main or release/%s", ref, v.tag, v.core)
}
