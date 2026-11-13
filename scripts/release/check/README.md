# check

`check` decides if a branch can be released. The verify job of `.github/workflows/release.yml` runs it.

A release needs two people for every change. Code on `main` gets its second person from the PR review. A release branch may also have commits that did not come through a PR on `main`, and GitHub has no rule that links them to a reviewed commit. `check` finds those commits, so the release approver knows what to read.

## Rules

`check` fails if:

- the version is not `vX.Y.Z` or `vX.Y.Z-(rc|beta|alpha).N`;
- the ref is not `main` or `release/vX.Y.Z` for the same `X.Y.Z`;
- the tag exists.

Then it takes `base = merge-base(origin/main, origin/<ref>)` and gives each commit in `base..origin/<ref>` a result:

| Result | Commit |
| --- | --- |
| accepted | the same change as a commit on the first-parent chain of `base..origin/main` (a clean cherry-pick), and `main` did not revert it later |
| accepted | changes only files under `docs/` |
| rejected | a merge commit; `check` fails |
| review | any other commit, such as a cherry-pick that needed conflict fixes; the approver reads it, and the publish job blocks an approver who committed it |

Two commits make the same change if their diffs are equal apart from line numbers and blob IDs. The function name in each hunk header counts, so the same lines changed in another function go to review.

The first-parent chain holds the commits that `main` took itself: squash merges, rebase merges and direct pushes. The commits of a branch that a merge commit brings in do not count, because a later commit of that branch can undo a change. A revert anywhere on `main` counts.

## Usage

```sh
git fetch origin '+refs/heads/*:refs/remotes/origin/*' --tags
go run ./scripts/release/check -ref release/v0.16.9 -version v0.16.9-rc.1 -repo NethermindEth/juno
```

It prints the Markdown report and appends it to the `-summary` file. It appends these lines to the `-output` file:

```
sha=<commit to release>
prerelease=<true or false>
review=<space-separated SHAs of the commits to review>
```
