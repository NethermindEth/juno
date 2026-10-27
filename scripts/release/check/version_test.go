package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		tag  string
		want releaseVersion
	}{
		{"v0.16.9", releaseVersion{tag: "v0.16.9", core: "v0.16.9"}},
		{"v0.16.9-rc.1", releaseVersion{tag: "v0.16.9-rc.1", core: "v0.16.9", prerelease: true}},
		{"v1.0.0-beta.0", releaseVersion{tag: "v1.0.0-beta.0", core: "v1.0.0", prerelease: true}},
		{"v1.10.0-alpha.12", releaseVersion{tag: "v1.10.0-alpha.12", core: "v1.10.0", prerelease: true}},
	}
	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			got, err := parseVersion(tt.tag)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestParseVersionRejects(t *testing.T) {
	for _, tag := range []string{
		"", "0.16.9", "v0.16", "v0.16.9-rc1", "v0.16.9-rc.01", "v01.16.9",
		"v0.16.9 ", "v0.16.9\n", "v0.16.9-rc.1-x", "v0.16.9-final.1",
	} {
		t.Run(tag, func(t *testing.T) {
			_, err := parseVersion(tag)
			require.ErrorContains(t, err, "invalid version")
		})
	}
}

func TestCheckRef(t *testing.T) {
	stable := releaseVersion{tag: "v0.16.9", core: "v0.16.9"}
	rc := releaseVersion{tag: "v0.16.9-rc.1", core: "v0.16.9", prerelease: true}

	for _, v := range []releaseVersion{stable, rc} {
		require.NoError(t, checkRef("main", v))
		require.NoError(t, checkRef("release/v0.16.9", v))
	}
	refs := []string{
		"release/v0.16.8", "release/v0.16", "release/v0.16.9/x", "feature/x", "refs/heads/main", "",
	}
	for _, ref := range refs {
		require.ErrorContains(t, checkRef(ref, stable), "cannot hold v0.16.9")
	}
}
