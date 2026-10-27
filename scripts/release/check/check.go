package main

import (
	"context"
	"strings"
)

const shortSHA = 12

// result is the decision about one commit of a release ref that is not on main.
type result int

const (
	accepted    result = iota
	needsReview        // the release approver reads it and must not have made it
	rejected
)

// verdict tells what happens to a commit and why.
type verdict struct {
	commit
	result result
	reason string
}

// checker decides about each commit of a release ref that is not on main.
type checker struct {
	git *gitRepo
}

// check returns a verdict for each commit in base..tip, oldest first.
func (c *checker) check(ctx context.Context, base, mainTip, tip string) ([]verdict, error) {
	commits, err := c.git.commits(ctx, base, tip)
	if err != nil || len(commits) == 0 {
		return nil, err
	}
	mainKeys, err := c.git.changeKeys(ctx, base, mainTip)
	if err != nil {
		return nil, err
	}
	onMain := make(map[string]string, len(mainKeys))
	for sha, key := range mainKeys {
		onMain[key] = sha
	}
	reversedKeys, err := c.git.reversedChangeKeys(ctx, base, mainTip)
	if err != nil {
		return nil, err
	}
	// A revert of a revert also lands here, so a commit that main applied again goes to review too.
	revertedBy := make(map[string]string, len(reversedKeys))
	for sha, key := range reversedKeys {
		revertedBy[key] = sha
	}
	keys, err := c.git.changeKeys(ctx, base, tip)
	if err != nil {
		return nil, err
	}

	verdicts := make([]verdict, len(commits))
	for i, cm := range commits {
		key := keys[cm.sha]
		if verdicts[i], err = c.checkCommit(ctx, cm, onMain[key], revertedBy[key]); err != nil {
			return nil, err
		}
	}
	return verdicts, nil
}

// checkCommit decides about one commit. mainSHA is the main commit with the same patch, and
// revertSHA is the main commit that reverts that patch. Each can be empty.
func (c *checker) checkCommit(
	ctx context.Context, cm commit, mainSHA, revertSHA string,
) (verdict, error) {
	if cm.parents > 1 {
		return verdict{commit: cm, result: rejected, reason: "merge commit"}, nil
	}
	if mainSHA != "" {
		reason := "cherry-pick of " + mainSHA[:shortSHA]
		if revertSHA != "" {
			// Main does not ship this change any more, so the release approver must read it.
			reason += ", reverted on main by " + revertSHA[:shortSHA]
			return verdict{commit: cm, result: needsReview, reason: reason}, nil
		}
		return verdict{commit: cm, result: accepted, reason: reason}, nil
	}
	files, err := c.git.changedFiles(ctx, cm.sha)
	if err != nil {
		return verdict{}, err
	}
	if docsOnly(files) {
		return verdict{commit: cm, result: accepted, reason: "docs only"}, nil
	}
	return verdict{commit: cm, result: needsReview, reason: "not on main and not docs only"}, nil
}

// docsOnly reports whether files is not empty and every file is under docs/.
// The release image and binaries do not contain docs, and the docs site deploys from main.
func docsOnly(files []string) bool {
	for _, f := range files {
		if !strings.HasPrefix(f, "docs/") {
			return false
		}
	}
	return len(files) > 0
}
