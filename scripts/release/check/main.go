// Command check verifies that a release ref can be released: the version and ref match, the tag
// does not exist, and every commit that is not on main is a clean cherry-pick, docs only, or listed
// for the release approver to review.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type options struct {
	ref, version, repo, dir, summary, output string
}

func main() {
	var o options
	flag.StringVar(&o.ref, "ref", "", "branch to release: main or release/vX.Y.Z")
	flag.StringVar(&o.version, "version", "", "release tag, for example v0.16.9 or v0.16.9-rc.1")
	flag.StringVar(&o.repo, "repo", "", "GitHub repository as owner/name, for the commit links")
	flag.StringVar(&o.dir, "dir", ".", "git repository with all branches and tags of origin fetched")
	flag.StringVar(&o.summary, "summary", "", "file to append the Markdown report to")
	flag.StringVar(&o.output, "output", "", "file to append the step outputs to")
	flag.Parse()

	if err := run(context.Background(), &o); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, o *options) error {
	v, err := parseVersion(o.version)
	if err != nil {
		return err
	}
	if err := checkRef(o.ref, v); err != nil {
		return err
	}
	g := &gitRepo{dir: o.dir}
	exists, err := g.tagExists(ctx, v.tag)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("tag %s exists", v.tag)
	}
	sha, err := g.revParse(ctx, "refs/remotes/origin/"+o.ref)
	if err != nil {
		return err
	}
	mainTip, err := g.revParse(ctx, "refs/remotes/origin/main")
	if err != nil {
		return err
	}
	base, err := g.mergeBase(ctx, mainTip, sha)
	if err != nil {
		return err
	}

	c := &checker{git: g}
	verdicts, err := c.check(ctx, base, mainTip, sha)
	if err != nil {
		return err
	}
	report := func(w io.Writer) { writeReport(w, o.repo, v, o.ref, sha, verdicts) }
	if err := appendTo(o.summary, report); err != nil {
		return err
	}
	report(os.Stdout)
	if n := len(withResult(verdicts, rejected)); n > 0 {
		return fmt.Errorf("%d commits on %s are rejected", n, o.ref)
	}
	return appendTo(o.output, func(w io.Writer) {
		fmt.Fprintf(w, "sha=%s\nprerelease=%s\nreview=%s\n", sha, strconv.FormatBool(v.prerelease),
			strings.Join(withResult(verdicts, needsReview), " "))
	})
}

// appendTo appends what write writes to the file at path. An empty path writes nothing.
func appendTo(path string, write func(io.Writer)) (err error) {
	if path == "" {
		return nil
	}
	const ownerReadWrite = 0o600
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, ownerReadWrite)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	write(f)
	return nil
}
