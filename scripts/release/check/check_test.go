package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheck(t *testing.T) {
	r := newTestRepo(t)
	r.commit("docs", map[string]string{"docs/guide.md": "guide\n"})
	base := r.head()
	fix := r.commit("fix", map[string]string{"a.go": "package a\n\nfunc A() {}\n"})
	r.commit("feature", map[string]string{"b.go": "package b\n"})
	mainTip := r.head()

	r.run("checkout", "--quiet", "-b", "release", base)
	r.run("cherry-pick", fix)
	pick := r.head()
	docs := r.commit("docs: version", map[string]string{"docs/versioned/guide.md": "guide\n"})
	releaseOnly := r.commit("release fix", map[string]string{"c.go": "package c\n"})
	r.run("mv", "docs/guide.md", "guide.md")
	moved := r.commit("move docs", nil)
	r.run("checkout", "--quiet", "-b", "side")
	side := r.commit("docs: side", map[string]string{"docs/side.md": "side\n"})
	r.run("checkout", "--quiet", "release")
	r.run("merge", "--quiet", "--no-ff", "-m", "merge side", "side")
	merge := r.head()

	c := &checker{git: r.git}
	verdicts, err := c.check(t.Context(), base, mainTip, merge)
	require.NoError(t, err)

	type outcome struct {
		result result
		reason string
	}
	got := make(map[string]outcome, len(verdicts))
	for _, v := range verdicts {
		got[v.sha] = outcome{v.result, v.reason}
	}
	notOnMain := "not on main and not docs only"
	require.Equal(t, map[string]outcome{
		pick:        {accepted, "cherry-pick of " + fix[:shortSHA]},
		docs:        {accepted, "docs only"},
		releaseOnly: {needsReview, notOnMain},
		moved:       {needsReview, notOnMain},
		side:        {accepted, "docs only"},
		merge:       {rejected, "merge commit"},
	}, got)
}

func TestCheckEmptyRange(t *testing.T) {
	r := newTestRepo(t)
	head := r.head()
	c := &checker{git: r.git}

	verdicts, err := c.check(t.Context(), head, head, head)
	require.NoError(t, err)
	require.Empty(t, verdicts)
}

func TestDocsOnly(t *testing.T) {
	require.True(t, docsOnly([]string{"docs/a.md", "docs/b/c.md"}))
	require.False(t, docsOnly([]string{"docs/a.md", "Makefile"}))
	require.False(t, docsOnly([]string{"docsx/a.md"}))
	require.False(t, docsOnly(nil))
}

// TestCheckRejectsCherryPickWithModeChange pins that the change key sees mode changes, also on a
// file whose content does not change.
func TestCheckRejectsCherryPickWithModeChange(t *testing.T) {
	r := newTestRepo(t)
	base := r.commit("scripts", map[string]string{"run.sh": "echo a\n", "other.sh": "echo o\n"})
	fix := r.commit("fix", map[string]string{"run.sh": "echo b\n"})

	r.run("checkout", "--quiet", "-b", "release", base)
	r.run("cherry-pick", fix)
	pick := r.head()
	makeExecutable := func(name string) string {
		r.run("reset", "--quiet", "--hard", base)
		require.NoError(t, os.Chmod(filepath.Join(r.git.dir, name), 0o700))
		return r.commit("fix and make "+name+" executable", map[string]string{"run.sh": "echo b\n"})
	}
	sameFile, otherFile := makeExecutable("run.sh"), makeExecutable("other.sh")

	c := &checker{git: r.git}
	for sha, want := range map[string]bool{pick: true, sameFile: false, otherFile: false} {
		verdicts, err := c.check(t.Context(), base, fix, sha)
		require.NoError(t, err)
		require.Len(t, verdicts, 1)
		require.Equal(t, want, verdicts[0].result == accepted, verdicts[0].reason)
	}
}

// TestCheckReviewsCherryPickOfRevertedCommit pins that a cherry-pick of a main commit that main
// reverted later goes to review, because main no longer ships that change.
func TestCheckReviewsCherryPickOfRevertedCommit(t *testing.T) {
	r := newTestRepo(t)
	base := r.commit("a", map[string]string{"a.go": "package a\n"})
	edit := r.commit("edit", map[string]string{"a.go": "package a\n\nfunc A() {}\n"})
	add := r.commit("add", map[string]string{"b.go": "package b\n"})
	binary := r.commit("binary", map[string]string{"c.bin": "\x00\x01\x02"})
	require.NoError(t, os.Chmod(filepath.Join(r.git.dir, "a.go"), 0o700))
	mode := r.commit("mode", nil)
	reverts := make(map[string]string)
	for _, sha := range []string{edit, add, binary, mode} {
		r.run("revert", "--no-edit", sha)
		reverts[sha] = r.head()
	}
	mainTip := r.head()

	c := &checker{git: r.git}
	for picked, revert := range reverts {
		r.run("checkout", "--quiet", "-B", "release", base)
		r.run("cherry-pick", picked)

		verdicts, err := c.check(t.Context(), base, mainTip, r.head())
		require.NoError(t, err)
		require.Len(t, verdicts, 1)
		require.Equal(t, needsReview, verdicts[0].result)
		require.Equal(t, "cherry-pick of "+picked[:shortSHA]+", reverted on main by "+revert[:shortSHA],
			verdicts[0].reason)
	}
}

// TestCheckReviewsChangeHiddenAfterBinaryFile pins that a change in a file after a binary file
// counts. In the binary patch of testdata/hex-line.bin, one line starts with 40 hex characters,
// and git patch-id read that line as the start of a new commit, so it ignored the rest.
func TestCheckReviewsChangeHiddenAfterBinaryFile(t *testing.T) {
	bin, err := os.ReadFile(filepath.Join("testdata", "hex-line.bin"))
	require.NoError(t, err)
	r := newTestRepo(t)
	base := r.commit("auth", map[string]string{"rpc/auth.go": "package rpc\n\nconst check = true\n"})
	fixture := r.commit("add fixture", map[string]string{"core/x.bin": string(bin)})

	r.run("checkout", "--quiet", "-b", "release", base)
	r.commit("add fixture", map[string]string{
		"core/x.bin":  string(bin),
		"rpc/auth.go": "package rpc\n\nconst check = false\n",
	})

	c := &checker{git: r.git}
	verdicts, err := c.check(t.Context(), base, fixture, r.head())
	require.NoError(t, err)
	require.Len(t, verdicts, 1)
	require.Equal(t, needsReview, verdicts[0].result, verdicts[0].reason)
}

// TestCheckReviewsChangeHiddenAfterLateNUL pins that a change in a file after a text file with a
// NUL counts. git shows a file as text when its first 8000 bytes hold no NUL, so a later NUL is
// in the diff.
func TestCheckReviewsChangeHiddenAfterLateNUL(t *testing.T) {
	fixture := strings.Repeat("a\n", 4500) + "\x00x\n"
	r := newTestRepo(t)
	base := r.commit("auth", map[string]string{"rpc/auth.go": "package rpc\n\nconst check = true\n"})
	added := r.commit("add fixture", map[string]string{"fixture.txt": fixture})

	r.run("checkout", "--quiet", "-b", "release", base)
	r.commit("add fixture", map[string]string{
		"fixture.txt": fixture,
		"rpc/auth.go": "package rpc\n\nconst check = false\n",
	})

	c := &checker{git: r.git}
	verdicts, err := c.check(t.Context(), base, added, r.head())
	require.NoError(t, err)
	require.Len(t, verdicts, 1)
	require.Equal(t, needsReview, verdicts[0].result, verdicts[0].reason)
}
