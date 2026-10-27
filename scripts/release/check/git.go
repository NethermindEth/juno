package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// gitRepo runs git commands in a local repository.
type gitRepo struct {
	dir string
}

// commit is one commit in a release range.
type commit struct {
	sha     string
	subject string
	parents int
}

func (g *gitRepo) command(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // set by this tool
	cmd.Dir = g.dir
	return cmd
}

// run runs git with args and returns its trimmed standard output.
func (g *gitRepo) run(ctx context.Context, args ...string) (string, error) {
	out, err := g.output(ctx, args...)
	return strings.TrimSpace(out), err
}

// output runs git with args and returns its standard output.
func (g *gitRepo) output(ctx context.Context, args ...string) (string, error) {
	var stderr bytes.Buffer
	cmd := g.command(ctx, args...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("running git %s: %w: %s",
			strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

// revParse returns the commit SHA of rev.
func (g *gitRepo) revParse(ctx context.Context, rev string) (string, error) {
	return g.run(ctx, "rev-parse", "--verify", rev+"^{commit}")
}

// mergeBase returns the best common ancestor of a and b.
func (g *gitRepo) mergeBase(ctx context.Context, a, b string) (string, error) {
	return g.run(ctx, "merge-base", a, b)
}

// tagExists reports whether the tag exists in the repository.
func (g *gitRepo) tagExists(ctx context.Context, tag string) (bool, error) {
	err := g.command(ctx, "show-ref", "--verify", "--quiet", "refs/tags/"+tag).Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return true, nil
	case errors.As(err, &exitErr) && exitErr.ExitCode() == 1:
		return false, nil
	default:
		return false, fmt.Errorf("looking up tag %s: %w", tag, err)
	}
}

// commits returns the commits in base..tip, oldest first.
func (g *gitRepo) commits(ctx context.Context, base, tip string) ([]commit, error) {
	// Commit text can hold anything, so the list comes only from SHAs and the subjects only label it.
	out, err := g.run(ctx, "rev-list", "--reverse", "--topo-order", "--parents", base+".."+tip)
	if err != nil || out == "" {
		return nil, err
	}
	subjects, err := g.subjects(ctx, base, tip)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(out, "\n")
	commits := make([]commit, len(lines))
	for i, line := range lines {
		fields := strings.Fields(line)
		commits[i] = commit{sha: fields[0], subject: subjects[fields[0]], parents: len(fields) - 1}
	}
	return commits, nil
}

// subjects returns the subject line of each commit in base..tip, by commit SHA.
func (g *gitRepo) subjects(ctx context.Context, base, tip string) (map[string]string, error) {
	out, err := g.run(ctx, "log", "--format=%H%x00%s", base+".."+tip)
	if err != nil {
		return nil, err
	}
	subjects := make(map[string]string)
	for line := range strings.Lines(out) {
		sha, subject, _ := strings.Cut(strings.TrimSuffix(line, "\n"), "\x00")
		subjects[sha] = subject
	}
	return subjects, nil
}

// changeKeys returns a key for the change of each non-merge, non-empty commit in base..tip, by
// commit SHA. Two changes get the same key only if their diffs are the same apart from line
// numbers and blob IDs, so a clean cherry-pick gets the key of its commit.
// Whitespace and binary content count.
func (g *gitRepo) changeKeys(ctx context.Context, base, tip string) (map[string]string, error) {
	return g.changeKeysOf(ctx, base, tip, "--src-prefix=a/", "--dst-prefix=b/")
}

// reversedChangeKeys is changeKeys for the reversed change of each commit. The reversed key of a
// clean revert is the key of the commit that it reverts.
func (g *gitRepo) reversedChangeKeys(
	ctx context.Context, base, tip string,
) (map[string]string, error) {
	// -R also swaps the path prefixes.
	return g.changeKeysOf(ctx, base, tip, "-R", "--src-prefix=b/", "--dst-prefix=a/")
}

func (g *gitRepo) changeKeysOf(
	ctx context.Context, base, tip string, diffArgs ...string,
) (map[string]string, error) {
	out, err := g.run(ctx, "rev-list", "--no-merges", base+".."+tip)
	if err != nil {
		return nil, err
	}
	// git patch-id is not used: it adds up a hash per file, so extra files can cancel out a change,
	// and a binary patch line can look like a commit line to it, so it skips the files after it.
	// Each diff gets its own git process, because a diff can hold any byte and no separator is safe.
	keys := make(map[string]string)
	for _, sha := range strings.Fields(out) {
		diff, err := g.output(ctx, append([]string{
			"diff-tree", "-p", "--binary", "--no-renames", "--no-textconv", "--no-ext-diff",
			"--no-color", "--no-commit-id",
		}, append(diffArgs, sha)...)...)
		if err != nil {
			return nil, err
		}
		if diff != "" {
			keys[sha] = changeKey(diff)
		}
	}
	return keys, nil
}

// hunkHeader matches the line numbers at the start of a hunk header.
var hunkHeader = regexp.MustCompile(`^@@ -\d+(,\d+)? \+\d+(,\d+)? @@`)

// changeKey returns the SHA-256 of diff without its index lines and hunk headers. They hold the
// blob IDs and line numbers, which differ when other lines of the file differ.
func changeKey(diff string) string {
	h := sha256.New()
	for line := range strings.Lines(diff) {
		switch {
		case strings.HasPrefix(line, "index "):
			continue
		case hunkHeader.MatchString(line):
			line = "@@\n"
		}
		h.Write([]byte(line))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// changedFiles returns the paths that commit sha changes. A rename gives both paths.
func (g *gitRepo) changedFiles(ctx context.Context, sha string) ([]string, error) {
	out, err := g.run(ctx, "diff-tree", "--no-commit-id", "--name-only", "-r", "--no-renames",
		"--root", sha)
	if err != nil || out == "" {
		return nil, err
	}
	return strings.Split(out, "\n"), nil
}
