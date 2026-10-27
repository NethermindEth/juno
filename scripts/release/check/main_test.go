package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunIgnoresTagsNamedLikeBranches(t *testing.T) {
	r := newTestRepo(t)
	reviewed := r.head()
	r.run("update-ref", "refs/remotes/origin/main", reviewed)
	evil := r.commit("evil", map[string]string{"evil.go": "package evil\n"})
	r.run("tag", "origin/main", evil)

	output := filepath.Join(t.TempDir(), "output")
	o := &options{ref: "main", version: "v1.0.0-rc.1", repo: "o/r", dir: r.git.dir, output: output}
	require.NoError(t, run(t.Context(), o))

	got, err := os.ReadFile(output)
	require.NoError(t, err)
	require.Equal(t, "sha="+reviewed+"\nprerelease=true\nreview=\n", string(got))
}

func TestRunOutputsCommitsToReview(t *testing.T) {
	r := newTestRepo(t)
	base := r.head()
	r.run("update-ref", "refs/remotes/origin/main", base)
	r.run("checkout", "--quiet", "-b", "release/v1.0.0")
	first := r.commit("release fix", map[string]string{"a.go": "package a\n"})
	second := r.commit("another fix", map[string]string{"b.go": "package b\n"})
	r.run("update-ref", "refs/remotes/origin/release/v1.0.0", second)

	output := filepath.Join(t.TempDir(), "output")
	o := &options{
		ref: "release/v1.0.0", version: "v1.0.0", repo: "o/r", dir: r.git.dir, output: output,
	}
	require.NoError(t, run(t.Context(), o))

	got, err := os.ReadFile(output)
	require.NoError(t, err)
	require.Equal(t, "sha="+second+"\nprerelease=false\nreview="+first+" "+second+"\n", string(got))
}

func TestRunRejectsMergeCommits(t *testing.T) {
	r := newTestRepo(t)
	base := r.head()
	r.run("update-ref", "refs/remotes/origin/main", base)
	r.run("checkout", "--quiet", "-b", "side")
	r.commit("side", map[string]string{"a.go": "package a\n"})
	r.run("checkout", "--quiet", "-b", "release/v1.0.0", base)
	r.run("merge", "--quiet", "--no-ff", "-m", "merge side", "side")
	r.run("update-ref", "refs/remotes/origin/release/v1.0.0", r.head())

	o := &options{ref: "release/v1.0.0", version: "v1.0.0", repo: "o/r", dir: r.git.dir}
	require.ErrorContains(t, run(t.Context(), o), "1 commits on release/v1.0.0 are rejected")
}
