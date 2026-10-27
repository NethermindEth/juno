package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteReport(t *testing.T) {
	v := releaseVersion{tag: "v1.0.0", core: "v1.0.0"}
	sha := strings.Repeat("a", 40)
	b, c, d := strings.Repeat("b", 40), strings.Repeat("c", 40), strings.Repeat("d", 40)
	verdicts := []verdict{
		{commit: commit{sha: b, subject: "fix | pipe"}, result: accepted, reason: "docs only"},
		{
			commit: commit{sha: c, subject: "release fix <!--"}, result: needsReview,
			reason: "not on main and not docs only",
		},
		{commit: commit{sha: d, subject: "bad"}, result: rejected, reason: "merge commit"},
	}

	var got strings.Builder
	writeReport(&got, "o/r", v, "release/v1.0.0", sha, verdicts)
	require.Equal(t, "## Release v1.0.0 from release/v1.0.0\n\n"+
		"Commit: `"+sha+"`\n\n"+
		"**1 commits are not on main.** Read them before you approve. "+
		"Do not approve if you are the author or committer of one of them.\n\n"+
		"- [`cccccccccccc`](https://github.com/o/r/commit/"+c+")\n\n"+
		"| Commit | Result | Subject |\n|---|---|---|\n"+
		"| [`bbbbbbbbbbbb`](https://github.com/o/r/commit/"+b+") | accepted: docs only | ` fix \\| pipe ` |\n"+
		"| [`cccccccccccc`](https://github.com/o/r/commit/"+c+") | **review**: not on main and not docs only | ` release fix <!-- ` |\n"+
		"| [`dddddddddddd`](https://github.com/o/r/commit/"+d+") | **rejected**: merge commit | ` bad ` |\n", got.String())

	got.Reset()
	writeReport(&got, "o/r", v, "main", sha, nil)
	require.Equal(t, "## Release v1.0.0 from main\n\nCommit: `"+sha+"`\n\n"+
		"All commits are on main.\n", got.String())
}

func TestCodeSpan(t *testing.T) {
	for subject, want := range map[string]string{
		"":                       "`  `",
		"fix":                    "` fix `",
		"**accepted**":           "` **accepted** `",
		"<!-- hide":              "` <!-- hide `",
		"a | b":                  "` a \\| b `",
		"use `x` and ``y``":      "``` use `x` and ``y`` ```",
		"`start":                 "`` `start ``",
		"end`":                   "`` end` ``",
		"line\rbreak\x00\x1b[0m": "` line�break��[0m `",
	} {
		require.Equal(t, want, codeSpan(subject), "subject %q", subject)
	}
}

func TestWithResult(t *testing.T) {
	verdicts := []verdict{
		{commit: commit{sha: "a"}, result: accepted},
		{commit: commit{sha: "b"}, result: needsReview},
		{commit: commit{sha: "c"}, result: needsReview},
	}
	require.Equal(t, []string{"b", "c"}, withResult(verdicts, needsReview))
	require.Empty(t, withResult(verdicts, rejected))
}
