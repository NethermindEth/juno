package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const notOnMain = "**review**: not on main and not docs only"

func TestRunReportsEachCommit(t *testing.T) {
	r := newTestRepo(t)
	r.commit("docs", map[string]string{"docs/guide.md": "guide\n"})
	base := r.head()
	fix := r.commit("fix", map[string]string{"a.go": "package a\n\nfunc A() {}\n"})
	r.commit("feature", map[string]string{"b.go": "package b\n"})
	r.push("main", r.head())

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
	r.push("release/v1.0.0", merge)

	output, summary, err := r.check("release/v1.0.0", "v1.0.0")
	require.ErrorContains(t, err, "1 commits on release/v1.0.0 are rejected")
	require.Empty(t, output)
	require.Equal(t, []row{
		{pick, "accepted: cherry-pick of " + fix[:12], "` fix `"},
		{docs, "accepted: docs only", "` docs: version `"},
		{releaseOnly, notOnMain, "` release fix `"},
		{moved, notOnMain, "` move docs `"},
		{side, "accepted: docs only", "` docs: side `"},
		{merge, "**rejected**: merge commit", "` merge side `"},
	}, rows(summary))
}

func TestRunAcceptsDocsOnlyCommits(t *testing.T) {
	r := newTestRepo(t)
	base := r.head()
	r.push("main", base)

	for want, files := range map[string]map[string]string{
		"accepted: docs only": {"docs/a.md": "a\n", "docs/b/c.md": "c\n"},
		notOnMain:             {"docs/a.md": "a\n", "Makefile": "all:\n"},
	} {
		require.Equal(t, want, r.checkCommit(base, files))
	}
	require.Equal(t, notOnMain, r.checkCommit(base, map[string]string{"docsx/a.md": "a\n"}))
	require.Equal(t, notOnMain, r.checkCommit(base, nil))
}

func TestRunMatchesCherryPicksByChange(t *testing.T) {
	const body = "\n// 1\n// 2\n// 3\n\nfunc A() {"
	r := newTestRepo(t)
	base := r.commit("base", map[string]string{
		"a.go": "package a\n" + body + "}\n",
		"b.go": "package b\n",
	})
	fixed := map[string]string{"a.go": "package a\n" + body + " a() }\n", "blob.bin": "\x00\x01A"}
	fix := r.commit("fix", fixed)
	r.push("main", fix)
	accepted := "accepted: cherry-pick of " + fix[:12]

	require.Equal(t, accepted, r.checkCommit(base, fixed))

	// A line far above the change differs, so the line numbers and blob IDs of the change differ.
	r.run("checkout", "--quiet", "-B", "older", base)
	older := r.commit("older", map[string]string{"a.go": "// Package a.\npackage a\n" + body + "}\n"})
	require.Equal(t, accepted, r.checkCommit(older, map[string]string{
		"a.go": "// Package a.\npackage a\n" + body + " a() }\n", "blob.bin": "\x00\x01A",
	}))

	for name, files := range map[string]map[string]string{
		"whitespace": {"a.go": "package a\n" + body + "  a() }\n", "blob.bin": "\x00\x01A"},
		"binary":     {"a.go": "package a\n" + body + " a() }\n", "blob.bin": "\x00\x01B"},
		// The extra file sorts after the binary file.
		"extra file": {
			"a.go": "package a\n" + body + " a() }\n", "blob.bin": "\x00\x01A", "c.go": "package c\n",
		},
	} {
		require.Equal(t, notOnMain, r.checkCommit(base, files), name)
	}
}

func TestRunReviewsCherryPickWithModeChange(t *testing.T) {
	r := newTestRepo(t)
	base := r.commit("scripts", map[string]string{"run.sh": "echo a\n", "other.sh": "echo o\n"})
	fix := r.commit("fix", map[string]string{"run.sh": "echo b\n"})
	r.push("main", fix)

	r.run("checkout", "--quiet", "-B", "release", base)
	r.run("cherry-pick", fix)
	require.Equal(t, "accepted: cherry-pick of "+fix[:12], r.checkTip(r.head()))

	for _, name := range []string{"run.sh", "other.sh"} {
		r.run("checkout", "--quiet", "-B", "release", base)
		require.NoError(t, os.Chmod(filepath.Join(r.dir, name), 0o700))
		tip := r.commit("fix and make "+name+" executable", map[string]string{"run.sh": "echo b\n"})
		require.Equal(t, notOnMain, r.checkTip(tip), name)
	}
}

func TestRunReviewsCherryPickOfRevertedCommit(t *testing.T) {
	r := newTestRepo(t)
	base := r.commit("a", map[string]string{"a.go": "package a\n"})
	edit := r.commit("edit", map[string]string{"a.go": "package a\n\nfunc A() {}\n"})
	add := r.commit("add", map[string]string{"b.go": "package b\n"})
	binary := r.commit("binary", map[string]string{"c.bin": "\x00\x01\x02"})
	require.NoError(t, os.Chmod(filepath.Join(r.dir, "a.go"), 0o700))
	mode := r.commit("mode", nil)
	reverts := make(map[string]string)
	for _, sha := range []string{edit, add, binary, mode} {
		r.run("revert", "--no-edit", sha)
		reverts[sha] = r.head()
	}
	r.push("main", r.head())

	for picked, revert := range reverts {
		r.run("checkout", "--quiet", "-B", "release", base)
		r.run("cherry-pick", picked)
		require.Equal(t, "**review**: cherry-pick of "+picked[:12]+", reverted on main by "+revert[:12],
			r.checkTip(r.head()))
	}
}

// TestRunReviewsChangeAfterLateNUL pins that a change in a file after a text file with a NUL
// counts. git shows a file as text when its first 8000 bytes hold no NUL, so a later NUL is in the
// diff.
func TestRunReviewsChangeAfterLateNUL(t *testing.T) {
	fixture := strings.Repeat("a\n", 4500) + "\x00x\n"
	r := newTestRepo(t)
	base := r.commit("auth", map[string]string{"rpc/auth.go": "package rpc\n\nconst check = true\n"})
	r.push("main", r.commit("add fixture", map[string]string{"fixture.txt": fixture}))

	require.Equal(t, notOnMain, r.checkCommit(base, map[string]string{
		"fixture.txt": fixture,
		"rpc/auth.go": "package rpc\n\nconst check = false\n",
	}))
}

func TestRunReviewsChangeInAnotherFunction(t *testing.T) {
	const body = " {\n\ta := 1\n\tb := 2\n\tc := 3\n\td := 4\n\te := 5\n\tf := 6\n\tg := 7\n}\n"
	edited := strings.Replace(body, "d := 4", "d := 40", 1)
	file := func(a, b string) map[string]string {
		return map[string]string{"a.go": "package a\n\nfunc A()" + a + "\nfunc B()" + b}
	}
	r := newTestRepo(t)
	base := r.commit("base", file(body, body))
	fix := r.commit("fix A", file(edited, body))
	r.push("main", fix)

	require.Equal(t, "accepted: cherry-pick of "+fix[:12], r.checkCommit(base, file(edited, body)))
	require.Equal(t, notOnMain, r.checkCommit(base, file(body, edited)))
}

func TestRunReviewsCommitOfMergedBranch(t *testing.T) {
	r := newTestRepo(t)
	base := r.head()
	r.run("checkout", "--quiet", "-b", "pr")
	bad := r.commit("debug", map[string]string{"a.go": "package a\n\nconst debug = true\n"})
	r.commit("no debug", map[string]string{"a.go": "package a\n\nconst debug = false\n"})
	r.run("checkout", "--quiet", "main")
	r.run("merge", "--quiet", "--no-ff", "-m", "merge pr", "pr")
	r.push("main", r.head())

	r.run("checkout", "--quiet", "-B", "release", base)
	r.run("cherry-pick", bad)
	require.Equal(t, notOnMain, r.checkTip(r.head()))
}

func TestRunReviewsCherryPickRevertedInMergedBranch(t *testing.T) {
	r := newTestRepo(t)
	base := r.head()
	fix := r.commit("fix", map[string]string{"a.go": "package a\n"})
	r.run("checkout", "--quiet", "-b", "pr")
	r.run("revert", "--no-edit", fix)
	revert := r.head()
	r.run("checkout", "--quiet", "main")
	r.commit("other", map[string]string{"b.go": "package b\n"})
	r.run("merge", "--quiet", "--no-ff", "-m", "merge pr", "pr")
	r.push("main", r.head())

	r.run("checkout", "--quiet", "-B", "release", base)
	r.run("cherry-pick", fix)
	require.Equal(t, "**review**: cherry-pick of "+fix[:12]+", reverted on main by "+revert[:12],
		r.checkTip(r.head()))
}
