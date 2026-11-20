package main_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunWritesSummary(t *testing.T) {
	r := newTestRepo(t)
	base := r.head()
	r.push("main", base)
	docs := r.commit("fix | pipe", map[string]string{"docs/a.md": "a\n"})
	r.run("checkout", "--quiet", "-b", "side")
	fix := r.commit("release fix <!--", map[string]string{"a.go": "package a\n"})
	r.run("checkout", "--quiet", "main")
	r.run("merge", "--quiet", "--no-ff", "-m", "bad", "side")
	merge := r.head()
	r.push("release/v1.0.0", merge)

	_, summary, err := r.check("release/v1.0.0", "v1.0.0")
	require.ErrorContains(t, err, "1 commits on release/v1.0.0 are rejected")
	link := func(sha string) string {
		return "[`" + sha[:12] + "`](https://github.com/o/r/commit/" + sha + ")"
	}
	require.Equal(t, "## Release v1.0.0 from release/v1.0.0\n\n"+
		"Commit: `"+merge+"`\n\n"+
		"**1 commits are not on main.** Read them before you approve. "+
		"Do not approve if you are the author or committer of one of them.\n\n"+
		"- "+link(fix)+"\n\n"+
		"| Commit | Result | Subject |\n|---|---|---|\n"+
		"| "+link(docs)+" | accepted: docs only | ` fix \\| pipe ` |\n"+
		"| "+link(fix)+" | "+notOnMain+" | ` release fix <!-- ` |\n"+
		"| "+link(merge)+" | **rejected**: merge commit | ` bad ` |\n", summary)
}

func TestRunShowsSubjectsAsCode(t *testing.T) {
	r := newTestRepo(t)
	base := r.head()
	r.push("main", base)
	want := make(map[string]string)
	for subject, cell := range map[string]string{
		"":                        "`  `",
		"**accepted**":            "` **accepted** `",
		"<!-- hide":               "` <!-- hide `",
		"a | b":                   "` a \\| b `",
		"use `x` and ``y``":       "``` use `x` and ``y`` ```",
		"`start":                  "`` `start ``",
		"end`":                    "`` end` ``",
		"line\rbreak\x1b[0m":      "` line�break�[0m `",
		"commit " + base + " end": "` commit " + base + " end `",
	} {
		want[r.commit(subject, nil)] = cell
	}
	r.push("release/v1.0.0", r.head())

	_, summary, err := r.check("release/v1.0.0", "v1.0.0")
	require.NoError(t, err)
	got := make(map[string]string)
	for _, row := range rows(summary) {
		got[row.sha] = row.subject
	}
	require.Equal(t, want, got)
}
