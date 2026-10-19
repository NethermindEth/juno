package main_test

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	check "github.com/NethermindEth/juno/scripts/release/check"
	"github.com/stretchr/testify/require"
)

type testRepo struct {
	t     *testing.T
	dir   string
	clock int
}

func newTestRepo(t *testing.T) *testRepo {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	r := &testRepo{t: t, dir: t.TempDir()}
	r.run("init", "--quiet", "--initial-branch=main")
	r.run("config", "user.name", "test")
	r.run("config", "user.email", "test@example.com")
	r.commit("init", map[string]string{"README.md": "juno\n"})
	return r
}

func (r *testRepo) run(args ...string) string {
	r.t.Helper()
	var stderr bytes.Buffer
	// A cherry-pick onto the parent of its commit in the same second would make the same commit.
	r.clock++
	cmd := exec.CommandContext(r.t.Context(), "git", args...)
	cmd.Dir = r.dir
	cmd.Env = append(os.Environ(), fmt.Sprintf("GIT_COMMITTER_DATE=@%d +0000", 1_700_000_000+r.clock))
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	require.NoError(r.t, err, "git %s: %s", strings.Join(args, " "), stderr.String())
	return strings.TrimSpace(string(out))
}

// commit commits files with msg as the whole message, without git's message cleanup.
func (r *testRepo) commit(msg string, files map[string]string) string {
	r.t.Helper()
	for name, content := range files {
		path := filepath.Join(r.dir, name)
		require.NoError(r.t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(r.t, os.WriteFile(path, []byte(content), 0o600))
	}
	msgFile := filepath.Join(r.t.TempDir(), "msg")
	require.NoError(r.t, os.WriteFile(msgFile, []byte(msg), 0o600))
	r.run("add", "--all")
	r.run("commit", "--quiet", "--allow-empty", "--allow-empty-message", "--cleanup=verbatim",
		"-F", msgFile)
	return r.head()
}

func (r *testRepo) head() string {
	r.t.Helper()
	return r.run("rev-parse", "HEAD")
}

// push points the remote-tracking branch origin/branch at sha, as a fetch would.
func (r *testRepo) push(branch, sha string) {
	r.t.Helper()
	r.run("update-ref", "refs/remotes/origin/"+branch, sha)
}

// check runs the tool on ref and returns the step outputs and the summary that it wrote.
func (r *testRepo) check(ref, version string) (output, summary string, err error) {
	r.t.Helper()
	dir := r.t.TempDir()
	o := &check.Options{
		Ref: ref, Version: version, Repo: "o/r", Dir: r.dir,
		Summary: filepath.Join(dir, "summary"), Output: filepath.Join(dir, "output"),
	}
	err = check.Run(r.t.Context(), o)
	return r.read(o.Output), r.read(o.Summary), err
}

func (r *testRepo) read(path string) string {
	r.t.Helper()
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return ""
	}
	require.NoError(r.t, err)
	return string(b)
}

// checkCommit commits files on a release branch from base and returns the result of that
// commit in the summary.
func (r *testRepo) checkCommit(base string, files map[string]string) string {
	r.t.Helper()
	r.run("checkout", "--quiet", "-B", "release", base)
	tip := r.commit("release", files)
	return r.checkTip(tip)
}

// checkTip releases tip from release/v1.0.0 and returns the result of tip in the summary.
func (r *testRepo) checkTip(tip string) string {
	r.t.Helper()
	r.push("release/v1.0.0", tip)
	_, summary, err := r.check("release/v1.0.0", "v1.0.0")
	require.NoError(r.t, err)
	for _, row := range rows(summary) {
		if row.sha == tip {
			return row.result
		}
	}
	require.FailNow(r.t, "commit is not in the summary", "%s\n%s", tip, summary)
	return ""
}

// row is one row of the commit table in the summary.
type row struct {
	sha, result, subject string
}

var rowPattern = regexp.MustCompile(
	"(?m)^\\| \\[`[0-9a-f]{12}`\\]\\(https://github\\.com/o/r/commit/([0-9a-f]{40})\\) \\| (.+?) \\| (.*) \\|$",
)

func rows(summary string) []row {
	var got []row
	for _, m := range rowPattern.FindAllStringSubmatch(summary, -1) {
		got = append(got, row{sha: m[1], result: m[2], subject: m[3]})
	}
	return got
}

func TestRunFromMain(t *testing.T) {
	r := newTestRepo(t)
	reviewed := r.head()
	r.push("main", reviewed)
	// A tag named like the remote-tracking branch must not change the commit to release.
	evil := r.commit("evil", map[string]string{"evil.go": "package evil\n"})
	r.run("tag", "origin/main", evil)

	output, summary, err := r.check("main", "v1.0.0-rc.1")
	require.NoError(t, err)
	require.Equal(t, "sha="+reviewed+"\nprerelease=true\nreview=\n", output)
	require.Equal(t, "## Release v1.0.0-rc.1 from main\n\nCommit: `"+reviewed+"`\n\n"+
		"All commits are on main.\n", summary)
}

func TestRunOutputsCommitsToReview(t *testing.T) {
	r := newTestRepo(t)
	r.push("main", r.head())
	first := r.commit("release fix", map[string]string{"a.go": "package a\n"})
	second := r.commit("another fix", map[string]string{"b.go": "package b\n"})
	r.push("release/v1.0.0", second)

	output, _, err := r.check("release/v1.0.0", "v1.0.0")
	require.NoError(t, err)
	require.Equal(t, "sha="+second+"\nprerelease=false\nreview="+first+" "+second+"\n", output)
}

func TestRunFailsWhenTagExists(t *testing.T) {
	r := newTestRepo(t)
	r.push("main", r.head())
	r.run("tag", "v1.0.0")

	_, _, err := r.check("main", "v1.0.0")
	require.ErrorContains(t, err, "tag v1.0.0 exists")
}

func TestRunFailsWithoutBranch(t *testing.T) {
	r := newTestRepo(t)
	r.push("main", r.head())

	_, _, err := r.check("release/v1.0.0", "v1.0.0")
	require.ErrorContains(t, err, "running git rev-parse")
}
