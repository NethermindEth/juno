package main_test

import (
	"testing"

	check "github.com/NethermindEth/juno/scripts/release/check"
	"github.com/stretchr/testify/require"
)

func TestRunPrerelease(t *testing.T) {
	r := newTestRepo(t)
	r.push("main", r.head())

	for version, want := range map[string]string{
		"v0.16.9":          "false",
		"v0.16.9-rc.1":     "true",
		"v1.0.0-beta.0":    "true",
		"v1.10.0-alpha.12": "true",
	} {
		output, _, err := r.check("main", version)
		require.NoError(t, err, version)
		require.Contains(t, output, "\nprerelease="+want+"\n", version)
	}
}

func TestRunAcceptsReleaseBranchOfCoreVersion(t *testing.T) {
	r := newTestRepo(t)
	r.push("main", r.head())
	r.push("release/v1.10.0", r.head())

	for _, version := range []string{"v1.10.0", "v1.10.0-alpha.12"} {
		_, _, err := r.check("release/v1.10.0", version)
		require.NoError(t, err, version)
	}
}

func TestRunRejectsInvalidVersions(t *testing.T) {
	for _, version := range []string{
		"", "0.16.9", "v0.16", "v0.16.9-rc1", "v0.16.9-rc.01", "v01.16.9",
		"v0.16.9 ", "v0.16.9\n", "v0.16.9-rc.1-x", "v0.16.9-final.1",
	} {
		err := check.Run(t.Context(), &check.Options{Ref: "main", Version: version})
		require.ErrorContains(t, err, "invalid version", version)
	}
}

func TestRunRejectsRefsThatCannotHoldVersion(t *testing.T) {
	refs := []string{
		"release/v0.16.8", "release/v0.16", "release/v0.16.9/x", "feature/x", "refs/heads/main", "",
	}
	for _, ref := range refs {
		for _, version := range []string{"v0.16.9", "v0.16.9-rc.1"} {
			err := check.Run(t.Context(), &check.Options{Ref: ref, Version: version})
			require.ErrorContains(t, err, "cannot hold "+version, ref)
		}
	}
}
