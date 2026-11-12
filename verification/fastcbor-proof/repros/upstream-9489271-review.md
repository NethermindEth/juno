# Review of the upstream slice-index fix branch

The user authorized adopting the unmerged `mit-pdos/perennial`
`fix-index-slice-operands` branch. The harness pins the exact commit
`9489271c166c7567fb4561f3aed83da866cc921f`, replacing
`84d5380639a38f56cd517f1e81246fad7be5b21b`.
[Upstream PR #644](https://github.com/mit-pdos/perennial/pull/644)
was open when checked on 2026-11-12. Merge status is not a build dependency.
There are no local tracked modifications to Perennial.

## Trusted delta

Reviewed using the upstream `review-trusted` skill and its calibration cases.
The range changes 37 files, mostly regression proofs, specifications and generated
artifacts. Two implementation corrections matter:

- `Perennial/Golang/Defn/Slice.lean`: `index_slice` passes `(slice, index)` to
  `IndexRef`, matching the existing `index_ref_slice` definition. The existing
  signed bounds check, panic branch, element type and memory-load rule remain.
- `goose/goose.go`: after testing the first type-switch case type, iteration over
  `c.List[1:]` uses `getCond(i+1)`. Previously it repeated the first type and
  omitted the final type. The new generated context code and regression test
  reflect this correction.

These agree with the Go specification for
[index expressions](https://go.dev/ref/spec#Index_expressions) and
[type switches](https://go.dev/ref/spec#Type_switches).
The range introduces no trusted package model, trusted configuration change,
new semantic axiom or adequacy change. The slice fix corrects an operand order;
it does not weaken the bounds obligation. The array-value equality proposal
remains separate and unapplied.

## Validation

- Rebuilt Goose and proofgen with Go 1.27.0 from the exact new pin.
- Built `Perennial` plus every changed `Perennial/Proof` and `Perennial/Spec`
  module: 24 requested targets, successful. This includes byte-slice,
  named-byte-slice, generic-string indexing and final-type-switch-case
  regression proofs, and the changed context proofs. Printed the axiom dependencies
  of all 37 added upstream proof declarations: no admissions or native evaluation
  axioms; only Lean's standard axioms and existing opaque `sync.Map` /
  `sync.poolChainElt` type declarations occur.
- Ran `go test ./goose/...`. After fetching fixture dependencies, the remaining
  failure is `TestExternalPackage`: the unchanged fixture selects
  `github.com/tchajed/marshal@v0.6.2`, which references removed
  `primitive.UInt64Put`, `UInt32Put`, `UInt64Get` and `UInt32Get` symbols.
  This fixture's `go.mod` is identical at the old and new pins. The full suite
  is therefore **not** reported as passing. A follow-up
  `go test -skip '^TestExternalPackage$' ./goose/...` passed, including translator
  golden tests. The temporary fixture checksum file was removed afterward.
- Juno `check.sh` passed: 3152 build jobs and 4499 audited theorem results.
  The five new results prove the prefix helper, arbitrary-offset word contract,
  complete validator, and both general Header APIs without helper premises.
  Every prior theorem's axiom set and the combined axiom union are unchanged.
  The independent audit verified 1358 project hashes and 46 standard-library
  source hashes, exact dependency pin, frozen Go and preserved other worktrees.
- The root index audit rejects seven malformed proved-Header entries, including
  substituting a conditional theorem or retaining a helper premise. Existing
  theorem-signature parser tests pass.

Review conclusion: the implementation corrections match their intended Go
operations. The unrelated external-package fixture incompatibility limits the
upstream test evidence and is recorded rather than patched into the dependency.
Go source files and the archived performance candidate are unchanged.
