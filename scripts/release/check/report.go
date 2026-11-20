package main

import (
	"fmt"
	"io"
	"strings"
	"unicode"
)

var resultWords = map[result]string{
	accepted:    "accepted",
	needsReview: "**review**",
	rejected:    "**rejected**",
}

// writeReport writes the Markdown summary that the release approver reads.
// Commit text comes last, so it cannot hide or change the list of commits to review.
func writeReport(w io.Writer, repo string, v releaseVersion, ref, sha string, verdicts []verdict) {
	fmt.Fprintf(w, "## Release %s from %s\n\nCommit: `%s`\n\n", v.tag, ref, sha)
	if len(verdicts) == 0 {
		fmt.Fprint(w, "All commits are on main.\n")
		return
	}
	link := func(sha string) string {
		return fmt.Sprintf("[%#q](https://github.com/%s/commit/%s)", sha[:shortSHA], repo, sha)
	}
	if review := withResult(verdicts, needsReview); len(review) > 0 {
		fmt.Fprintf(w, "**%d commits are not on main.** Read them before you approve. "+
			"Do not approve if you are the author or committer of one of them.\n\n", len(review))
		for _, sha := range review {
			fmt.Fprintf(w, "- %s\n", link(sha))
		}
		fmt.Fprintln(w)
	}
	fmt.Fprint(w, "| Commit | Result | Subject |\n|---|---|---|\n")
	for _, v := range verdicts {
		fmt.Fprintf(w, "| %s | %s: %s | %s |\n",
			link(v.sha), resultWords[v.result], v.reason, codeSpan(v.subject))
	}
}

// codeSpan returns s as a Markdown code span in a table cell, so the summary shows commit text
// as it is and does not render it.
func codeSpan(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return unicode.ReplacementChar
		}
		return r
	}, s)
	fence := "`"
	for strings.Contains(s, fence) {
		fence += "`"
	}
	return fence + " " + strings.ReplaceAll(s, "|", `\|`) + " " + fence
}

// withResult returns the SHAs of the commits with result r.
func withResult(verdicts []verdict, r result) []string {
	var shas []string
	for _, v := range verdicts {
		if v.result == r {
			shas = append(shas, v.sha)
		}
	}
	return shas
}
