# Typed fastcbor proof pilot

This directory is an **incomplete first stage** of the typed-decoder verification plan.
The feasibility gate has not passed. There is no whole-library correctness theorem
and no candidate performance result yet.

## Source and scope

- Baseline: `a4a67242cc67f3855915a97bf4e79a97d217fae4`
  (`origin/danielntmd/cbor-fastcbor` when the task started).
- Worktree: `cbor-fastcbor-verified`, branch `danielntmd/cbor-fastcbor-verified`,
  based on the earlier assessment commit `2f33ecf2f`.
- Perennial/Goose: the commit in `perennial.rev`; Lean: `lean-toolchain`;
  Go: 1.27.0, linux/amd64. Upstream semantics are unchanged.

`utils/cbor/cmd/cborgen` reads the actual Go types and emits the Header and GasPrice
readers in `core/cbor_readers_generated.go`. Unknown field shapes fail generation.
The generated entries use typed stores, allocation, and zero checks instead of
runtime field offsets and runtime allocation hooks. The registry installs Header;
other types still use the original planner. The routing adapter, registration,
logging, FX fallback, and diagnostic formatting are outside the proposed proof scope.
The cursor was moved to `utils/cbor/cursor` so generated readers in core can use it.

`core.DecodeHeaderFirst` enforces a zero destination, returns the original unread
suffix, and clears the destination on a parse error. `core.DecodeHeader` additionally
rejects trailing data. A nonzero destination is rejected unchanged. Duplicate struct
keys are unsupported, as in the baseline planner; they are not silently accepted
with first-wins semantics. Projection semantics remain a separate obligation.

## What Lean checks

`ProofCheck/Header.lean` proves limited paths through the **translated production
functions**, rather than an independent decoder model:

- the zero Header check, and rejection when Hash is nonnil;
- cursor construction, null consumption, opening a null struct, and its Done result;
- the generated header loop and public First entry on null, including exact suffix
  and preserved input ownership;
- the full-input entry's trailing-data error after null;
- structured error construction and empty-input error propagation through the
  cursor, generated reader, and public First entry;
- decoding each of the 24 inline unsigned integers with any suffix.

These are partial-correctness triples under the pinned Goose/Perennial semantics.
They require package initialization and, where used, ownership of the relevant
initialized globals. Initializer correctness has **not** been proved. The empty
and null examples do not prove cleanup after partially populating a Header.

`audit.py` checks every pilot theorem's transitive axiom report for `sorryAx` and
records its source hashes. None of the checked pilot theorems depends on `sorryAx`.
Generated modules still contain unused admitted instances and opaque type axioms
for untranslated declarations. The audit lists the transitive opaque axioms; this
is not an assertion that the whole generated project is free of axioms or admissions.
No theorem proving decoder correctness has been postulated.

## Reproduce

From the repository root:

```sh
go run ./utils/cbor/cmd/cborgen -check
verification/fastcbor-proof/bootstrap.sh
verification/fastcbor-proof/check.sh
go test -short ./utils/cbor/... ./db/typed/value ./core
go test -short -race -gcflags=all=-d=checkptr=2 ./core -run '^TestTypedHeader'
go test -short ./core -run '^$' -fuzz '^FuzzTypedHeaderMatchesPlanner$' -fuzztime 5m -parallel 2
go test -short ./core -run '^$' -fuzz '^FuzzTypedHeaderRoundTrip$' -fuzztime 60s -parallel 2
```

`bootstrap.sh` installs local tools. An existing local elan installation can be
selected with `ELAN_HOME`. `check.sh` regenerates both Goose code and proof support
from the current Go sources, builds Lean, and writes `results/proof-audit.json`.
Generated Lean and tool binaries are ignored. Run the full checker after any Go
change; a cached Lean build alone does not regenerate the translated code.

The recorded byte fuzz run passed 5,001,198 executions in five minutes. The
structure-aware run passed 799,882 executions in one minute. The latter generates
valid Header values, marshals them, and compares the typed decoder with both FX
and the original planner, including nil/empty nested slices and varied felt limbs.
The short Go suite and the targeted race/checkptr run passed. These counts are
historical observations, not assertions about future runs.

## Remaining gates, in order

1. **Complete feasibility:** prove a populated header path and cleanup after partial
   writes; establish actual field-table initialization; verify witnesses for felt
   pointer conversion, nested slices/maps, tagged interfaces, UTF8 and BigInt.
   `cursor.NewFields` still sorts and folds names at runtime. The required general
   helper contracts and the complete semantic dependency audit are unfinished.
2. **Complete the generator:** cover the agreed known roots, nested types,
   projections, integer-key structs, registration tags, overrides, and special
   allocation patterns. Generate static field metadata. Check the manifest against
   production callers and add drift checks. Arbitrary runtime types remain fallback.
3. **Correctness gate:** differential properties, original golden/corpus parity,
   malformed-input and destination-cleanup tests across every generated entry,
   including nil/empty, tag/depth, duplicate and legacy-key behavior.
4. **Performance gate:** port and smoke the frozen historical case universe on the
   untouched baseline, classify incompatibilities explicitly, then run paired,
   interleaved baseline/candidate samples under the shared lock. Every comparable
   case must satisfy the agreed 3% bound for time and allocation metrics. No FX
   fallback is timed. Use the prespecified 5/10/20 looks; unresolved at 20 is
   inconclusive. Fix regressions before continuing to the full proofs.
5. **Complete proofs:** explicit decoder relations and shared primitive/helper
   contracts, then every typed entry's values, suffix, read-only input, independent
   output ownership, nil/empty distinctions, errors and destination state. Prove
   initializer obligations and audit the actual dependency closure. Compiler,
   runtime implementation, OOM, and termination remain outside the claim.

`results/benchmark-universe.json` freezes all 185 historical cases, corpus records
and source hashes against the original baseline. All corpus hashes were checked.
It records the removed EventsBloom projection and historical zero-route controls;
other cases still require baseline compatibility smoke. **No cases have passed the
new performance gate and no performance timings have been collected.**
