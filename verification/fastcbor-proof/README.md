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
The generator also emits static field metadata and small allocation factories; the
typed Header and GasPrice readers no longer sort or fold field names at startup.
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

The follow-up modules extend this pilot:

- `Values.lean`: inline uints at arbitrary valid offsets, overwriting scalar fields,
  and writing Header.Number while preserving every other field.
- `Fields.lean`: the actual Header and GasPrice metadata allocation factories,
  plus checks of each entry's encoded name, index, bit, cached words, and masks.
- `Number.lean`: the actual key search, cursor advancement, field tracking and
  generated Header loop, composed through both public entries for the nine-byte
  record `{"Number": n}`, for every inline uint `n` from 0 through 23. These proofs
  preserve input bytes, restore nesting depth and establish the exact empty suffix.
  The internal reader theorem also preserves all other fields of an arbitrary Header.
- `BigInt.lean`: the actual Go `nat.norm` and `Int.SetBits` implementations for
  arbitrary word slices, including preservation of their mathematical magnitude;
  in-place `Set` and `Neg`; and fresh `SetUint64` for zero and every nonzero uint64.
- `Header.lean` additionally proves that the production cleanup function clears
  **any** Header state. Its composition with a parser failure after a successful
  field write is still pending.

These are partial-correctness triples under the pinned Goose/Perennial semantics,
on the 64-bit target. They require package initialization and, where used, ownership
of the relevant initialized globals. The field factories are proved, but the global
package initializer and its connection to those factories are **not** yet proved.
The populated example does not cover arbitrary Header fields, widths or suffixes.

The BigInt decoder now packs bytes into owned Go words, uses `SetBits` to normalize,
and uses in-place `Neg`. Negative CBOR bignums add one with a Go carry loop. This
removes the decoder's dependence on `Int.Not` and its assembly arithmetic. The real
required math/big implementations are translated locally; no arithmetic helper
correctness theorem has been assumed. **The byte packing, carry loop and complete
CBOR BigInt contract still need proofs.** This change is not yet performance-tested.

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
go test -short -race -gcflags=all=-d=checkptr=2 ./core ./utils/cbor/cursor ./utils/cbor/compatibility
go test -short ./core -run '^$' -fuzz '^FuzzTypedHeaderMatchesPlanner$' -fuzztime 5m -parallel 2
go test -short ./core -run '^$' -fuzz '^FuzzTypedHeaderRoundTrip$' -fuzztime 60s -parallel 2
go test -short ./utils/cbor/compatibility -run '^$' -fuzz '^FuzzReadersMatchFxamacker$' -fuzztime 5m -parallel 2
GOARCH=386 go test -short ./utils/cbor/cursor -run '^TestBigIntCarryAndOwnership$'
```

`bootstrap.sh` installs local tools. An existing local elan installation can be
selected with `ELAN_HOME`. `check.sh` regenerates both Goose code and proof support
from the current Go sources, builds Lean, and writes `results/proof-audit.json`.
Generated Lean and tool binaries are ignored. Run the full checker after any Go
change; a cached Lean build alone does not regenerate the translated code.

The latest byte fuzz run passed 5,162,067 executions in five minutes after the
static-table change. The structure-aware Header run passed 779,586 executions in
one minute. It generates valid Header values, marshals them, and compares the typed
decoder with both FX and the original planner, including nil/empty nested slices
and varied felt limbs. Primitive differential fuzzing passed 4,424,589 executions
in five minutes after the BigInt change. The short Go suite and race/checkptr run
passed. New BigInt tests cover zero, leading zeros, word boundaries, carry chains,
MaxUint64, suffixes and output ownership, on amd64 and 386. The formal target remains
64-bit. These counts are historical observations, not promises about future runs.

## Remaining gates, in order

1. **Complete feasibility:** connect cleanup to an actual parse failure after partial
   writes; connect the proved field factories to global initialization; verify
   witnesses for felt pointer conversion, nested slices/maps, tagged interfaces,
   UTF8 and the complete BigInt helper. The populated Number path is checked; the
   required general helper contracts and complete semantic dependency audit remain
   unfinished. Legacy planner types still use runtime `cursor.NewFields`.
2. **Complete the generator:** cover the agreed known roots, nested types,
   projections, integer-key structs, registration tags, overrides, and special
   allocation patterns. Extend the static metadata generator. Check the manifest against
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
