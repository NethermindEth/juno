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

## Immediate target: complete Header

The next target is one general Header contract, covering all 15 fields and its
GasPrice children, arbitrary supported field order/widths and suffixes, and
failure cleanup. It is not a proof about one populated sample. Header needs
unsigned integers, UTF-8 strings, felts, nested signature slices, and GasPrice;
BigInt, tagged transaction interfaces, and typed Go maps are later library work.
This prioritizes a complete Header proof before widening the generator. The
whole-library performance gate remains mandatory before full-library proofs.

The specification is to be independent of fxamacker. RFC 8949 supplies the wire
rules; the Juno schema supplies the mapping into Go values. CBOR does not specify
that `gasprice` names L1GasPriceETH, that undefined behaves like null, or how a
four-word Montgomery felt is represented. Differential tests against FX remain
compatibility evidence, not an assumption of the Lean theorems.

The complete contract must account for these current application choices:

- A zero destination is required. First returns the exact unread suffix; the full
  entry rejects trailing bytes. Parse failures clear the destination; a nonzero
  destination is rejected unchanged.
- Header and GasPrice are definite-length maps (or null/undefined). Missing fields
  stay zero. Known text names match exactly or with ASCII case folding; legacy
  `gasprice` and `gaspricestrk` map to their explicit fields. Repeated known fields
  are unsupported, including differently cased names of the same field.
- Integer fields accept all unsigned CBOR widths, including non-preferred wider
  encodings, plus null/undefined leaving the field unchanged. On the 64-bit target,
  L1DAMode accepts any representable uint, not just the two named enum constants.
- String fields check UTF-8. Felt pointers distinguish null from an allocated
  zero felt; array elements are uint64 Montgomery limbs. Short arrays zero-fill;
  long arrays discard extra elements using Skip.
- Signatures preserve nil versus empty at both slice levels and nil felt entries.
  The two GasPrice pointers may share one allocation, with separate element ownership.
- Unknown text fields and supported integer keys have their values skipped. Skip
  deliberately does not validate text UTF-8, so successful Header decoding is
  **not** a blanket assertion of RFC 8949 validity for ignored content. Duplicate
  unknown map keys are not tracked as known-field duplicates.
- Indefinite lengths and some tags are unsupported. Container/tag depth is bounded
  by the production limit of 32, including the special chained-tag counting rule.

These choices are to be formalized explicitly; this list is not a completed
proof. Strict validation of all ignored CBOR content would require a separately
specified behavior change and performance evaluation. No such change is made here.

### Header work queue and exit criteria

Current status: general head/uint/container success and null-field contracts,
container count/depth failures, invalid-head classification, scalar type/tag and
extended-head truncation failures, the arbitrary-limb 37-byte felt fast path,
general felt success through an independent nullable/short/extended wire relation,
felt depth rejection, general recursive Skip success and atomic rejection/EOF contracts,
definite payload success/truncation and public byte/string propagation contracts,
and bounded public Header examples. The reproducible audit records the current lemma count.
The count is not a coverage percentage. The complete Header theorem is absent.

| Order | Work | Exit criterion | Current status |
| --- | --- | --- | --- |
| 1 | Formalize the complete Header application contract | Independent definitions of accepted encodings, decoded values, rejection cases and ownership, covering all 15 fields and GasPrice; the checklist above is represented formally | Head-argument, recursive Skip and felt value relations exist; full Header relation pending |
| 2 | Finish primitive error contracts | Reserved/invalid heads, wrong types, truncation, excessive counts and depth have the specified result, cursor state and error information | Head classification, scalar type/tag and extended-head truncation failures, and container count/depth failures proved; general EOF and propagation of all head errors through remaining callers pending |
| 3 | General felt and UTF-8 contracts | Fast and slow felt paths agree on limb contents, consumption and failure behavior; text validation and conversion have proved contracts | General felt success (null/undefined, short arrays and arbitrary valid extras) and depth rejection proved through the typed reader; payload extraction and truncation propagation proved; remaining felt failures, converse and UTF-8 pending (generic slice-index fix proposed below) |
| 4 | General field lookup and Skip | Every supported field order, width, case variant, unknown key/value, duplicate and nested skip is covered, including rollback/error behavior | Static table factories and restricted Number lookup proved; general recursive Skip success proved, including both wide shortcuts; atomic rejection/EOF and public rollback proved, with recursive rejection and other head errors pending |
| 5 | Nested outputs and GasPrice | Arbitrary signature lengths and contents, nil/empty distinctions, disjoint owned elements in shared allocations, and partial-construction errors are covered | Null-field behavior proved; general non-null outputs pending |
| 6 | Initialization and immutable table sharing | Relevant actual initialization establishes the table contents and shareable ownership needed by independent decoder calls | Factories proved; wiring and shared ownership pending |
| 7 | Header loop and public APIs | Compose the preceding contracts into general theorems for cborReadHeader, DecodeHeaderFirst and DecodeHeader, including arbitrary supported records and all specified rejection cases | Restricted null/empty/Number examples and one late-error family proved |
| 8 | Reproduction and validation | Regenerate from the current Go source, Lean-check the full dependency chain, audit axioms/assumptions, and pass the applicable Go regression/property/corpus checks | Harness works; final complete-Header run pending |

This is dependency order, not eight isolated phases: nested GasPrice uses field
lookup, and successful felt decoding now composes the recursive Skip theorem.
The next primitive work is the remaining felt/Skip rejection contracts and UTF-8;
the latter has a proposed one-line repair to the pinned generic slice-index semantics
(see the approval proposal below), while independent work remains available.
The general Header relation and field lookup still need development alongside
them. BigInt, tagged transaction interfaces and ordinary Go maps do not block
this Header milestone.

### Definition of Header complete

The deliverable is the existing generated Go Header/GasPrice readers plus explicit
specifications and Lean proofs about their translated production implementations.
For any input bytes and valid owned destination satisfying the API's memory
preconditions, the public theorems must characterize the specified outcomes:

- **Successful First decoding:** all Header fields and nested values match the
  application contract; the returned slice is exactly the unread input suffix;
  input bytes are unchanged. Output data does not borrow the input; permitted
  sharing between output elements is described by the ownership predicate.
- **Successful full decoding:** the same result, with no trailing bytes.
- **Failures:** malformed, type-mismatched, unsupported and depth/count-limited
  cases follow the specified classification and structured error information.
  Parse failures and trailing-data rejection leave a zero Header; First returns
  a nil suffix on failure. A nonzero destination is rejected unchanged.
- **Coverage:** all fields, arbitrary supported encodings, orders and nested
  contents, including null/undefined and nil/empty distinctions. A theorem for
  one fixed serialized Header, or only encoder-produced seeds, does not qualify.
- **Closed decoder obligations:** helper correctness, field-table initialization
  and read-only sharing are proved rather than postulated or left as unexplained
  decoder-specific preconditions. Generated-code drift and relevant source hashes
  are checked. The final audit documents the actual semantic/dependency boundary.

These remain partial-correctness/model-safety claims on the pinned 64-bit target;
termination, resource exhaustion, the Go compiler and concrete runtime are outside
scope. They are not Starknet consensus validation or validation of every ignored
CBOR value. FX equivalence and the routing/registration/fallback/formatting layer
are outside the formal claim. The generator itself is not the proof target: the
emitted readers and their mapping to the independent schema are checked.

Header completion is the first substantial end-to-end result, not completion of
the decoder-library project or approval to deploy. It will show whether the proof
contracts compose for this representative type. Further required shapes still need
feasibility checks. Before expanding to full-library proofs, enumerate the exact
remaining production roots in a checked manifest, implement their typed readers,
pass compatibility checks and the frozen performance gate below. The 185-case
benchmark universe is not itself a manifest of formally verified entry points.

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
  **any** Header state.
- `LateError.lean`: a map declares two pairs but contains only `Number: n`, for
  every inline uint from 0 through 23. The internal reader writes Number, then
  returns the truncation error at byte 9. Both public entries clear the partial
  result and preserve the original error and input bytes; First returns a nil suffix.
- `Felt.lean`: the production felt reader on an empty-array encoding, including
  allocation, conversion from `*felt.Felt` to `*[4]uint64`, limb stores and
  publication of a nonnil, independently owned zero felt. This is a pointer-conversion
  witness. The arbitrary-limb fast path is now covered separately in `FeltWide.lean`;
  the complete felt contract remains pending.

The general Header proof now has reusable primitive contracts:

- `Head.lean`: an explicit definite-head argument relation covering inline values
  and 1/2/4/8-byte big-endian arguments; both head paths and both expectation paths;
  Uint64 and the typed uint/mode readers at arbitrary offsets with arbitrary
  remaining input. Non-preferred widths are included. A separate theorem proves
  cursor/input preservation and the error offset for truncation inside an extended
  head. Rejection and propagation contracts are extended in `Reject.lean`.
- `Containers.lean`: all supported widths through both ArrayLen and MapLen paths,
  OpenStruct, and restoration of the enclosing depth when a struct finishes. The
  success contracts require a mathematical bound on the remaining bytes and room
  in the nesting limit.
- `Errors.lean`: arbitrary supported count widths and offsets through ArrayLen,
  MapLen and OpenStruct. Excessive counts report truncation before checking depth;
  valid counts at excessive depth report the depth error. Both preserve the cursor
  and input and report the original offset. The depth diagnostic's argument slice
  is allocated and owned in the proof. Public API regression tests cover all five
  widths, maximum uint64 counts, repeated failures and depth restoration.
- `Reject.lean`: every reserved/invalid additional-information head, stray break,
  and indefinite-length head is classified by the actual slow head parser. The
  diagnostic helper is proved for all 256 head bytes, including its byte-slice
  allocation and string conversion. Type/tag mismatch and extended-argument
  truncation contracts reach both expectation paths, Uint64 and the typed
  uint/mode field readers, preserving the cursor, input and prior field value.
  Public API tests check all 256 head bytes and every cut through an extended
  argument, including truncation before type/tag rejection. General EOF and
  propagation of invalid-head failures through callers remain to be composed.
- `Null.lean`: null and undefined at arbitrary offsets, composed with every shared
  scalar/string/felt/signature field reader. Scalar fields stay unchanged; pointer
  and slice fields become nil. Non-null nested allocation contracts are pending.
- `FeltWide.lean`: the real 37-byte fast path for all four arbitrary uint64 limb
  values, arbitrary valid input offsets and suffixes, and arbitrary prior output.
  It proves exact consumption, unchanged depth/scratch/input, and owned output
  words. The typed reader proof includes fresh nonnull allocation and the named
  felt-to-array pointer conversion; a round-trip corollary states the result
  directly in terms of four uint64 words. General depth rejection covers every
  supported array-head width/count through both limb entry points and the typed
  reader, preserving the prior output and cursor. `FuzzLimbsWide` constructs
  valid encodings from arbitrary words, suffixes and nesting depths.
- `FeltSequence.lean`: an unsigned wire-item relation separates value from chosen
  width, with exact suffix consumption. A loop invariant proves the actual slow
  felt reader for zero through four limbs with independently chosen widths and
  arbitrary values, zero-filling the remaining words. The public Limbs and typed
  felt reader compose this for every short array and every extended-width
  four-element array head, including fresh owned allocation. `FuzzLimbsWidths`
  generates counts, array-head widths and limb widths independently.
- `FeltShortcut.lean`: the actual inline four-element shortcut loop, including all
  five unsigned widths, standard-library big-endian reads, bounds checks and
  stores. A classification lemma ties the independent wire-item relation to
  each branch; a shape lemma rules out false wide-path matches on valid input.
- `FeltDispatch.lean`: composition of the wide path, mixed-width shortcut and
  slow loop into a public `Limbs` contract for every zero-to-four-limb array,
  with all supported array-head and limb widths. The typed reader additionally
  proves fresh nonnull output ownership, exact suffix, unchanged input and
  cursor advancement without signed wraparound. `TestLimbsWidthCombinations`
  checks all 3,905 count/array-width/limb-width combinations with three suffixes
  (11,715 cases), including trailing bytes resembling wide-integer heads.
  Extended arrays now compose this work with recursive Skip in `FeltLong.lean`;
  the remaining error cases are still needed for a complete felt contract.
- `SkipAtom.lean`: an independent atomic acceptance relation and proofs of the
  actual recursive worker and public `Skip` for unsigned/negative integers,
  simple values, floats, and byte/text strings. It covers every supported head
  width, arbitrary payloads and suffixes, including invalid UTF-8 in ignored
  text. The public contract preserves input/depth/scratch and establishes the
  exact suffix and cursor advancement without signed wraparound.
- `SkipReject.lean`: simple values below 32 encoded in two bytes, strings longer
  than the available input, tags 0–3, and EOF. Public `Skip` restores the complete
  cursor state while retaining the worker's diagnostic offset and owned error.
  Propagation of other head errors and recursive rejection remain unproved.
- `SkipSpec.lean`: the recursive application acceptance relation for ignored
  values, including arrays, arbitrary map keys/values, supported tags, all
  definite head widths, and the chained-tag depth rule. Pure proofs establish
  nonempty child encodings and show that a wide-shape match on a valid value
  consumes exactly 37 bytes without stealing bytes from its suffix.
- `SkipWide.lean`, `SkipElements.lean`, `SkipContainer.lean`, and `SkipTag.lean`:
  the actual scanner shortcut, container loop (including its separate inline
  shortcut), array/map dispatch, and supported tag path. The loop and dispatch
  lemmas expose child contracts as induction hypotheses.
- `SkipValid.lean`: well-founded induction on encoded length discharges every
  child hypothesis and proves the actual worker and public `Skip` for every
  encoding in `SkipValid`, at starting depths 0 through 32. It proves exact
  consumption/suffix, no signed cursor wraparound, and unchanged input, depth
  and scratch. This is success on the specified valid inputs, not yet a converse
  or complete rejection theorem; recursive malformed-input behavior remains
  to be proved. It does not assume a correct recursive child scanner.
- `FeltLong.lean`: the actual slow loop for four arbitrary-width unsigned limbs
  followed by any number of values satisfying `SkipValid` at the felt's nested
  depth. Its invariant separates retained limbs from skipped values and proves
  count/position bounds, unchanged retained words, restored depth and exact
  consumption. Public Limbs dispatch and the typed field reader compose the
  result, including fresh nonnull allocation and owned words.
- `FeltValid.lean`: one application acceptance/value relation for null,
  undefined, short zero-filled arrays and extended arrays. `FeltResult`
  distinguishes a nil pointer from an owned allocated zero felt.
  `wp_readFelt_valid` proves the actual typed reader against this relation with
  exact suffix, unchanged input/depth/scratch, no signed cursor wraparound and
  output ownership. This remains a success theorem, not a complete rejection
  or converse theorem.

`Payload.lean` proves complete definite payload extraction at arbitrary offsets and
length-head widths, returning the exact borrowed range while preserving the whole
input and exact unread suffix. Declared lengths larger than the remaining input
restore the original cursor and return an owned truncation error at that offset,
including lengths above the signed 64-bit range. These contracts compose through
`BytesBorrowed`; truncation also composes through `String` and the typed string
field reader, preserving its old destination. They do not assume UTF-8 validity or
prove the validator. The actual UTF-8 word-helper attempt found a reversed operand
tuple in the pinned generic slice-index rule; the separate proposal below is
pending approval, and the failed reproducer is excluded from the audited target.

These contracts are not yet composed into a general Header theorem. Its remaining
critical dependencies are the remaining felt paths, UTF-8, nested signature/GasPrice
ownership, field lookup and Skip (including their errors), plus initialization and
shared table ownership. The earlier restricted Header examples still define the
current end-to-end proof coverage.

These are partial-correctness triples under the pinned Goose/Perennial semantics,
on the 64-bit target. They require package initialization and, where used, ownership
of the relevant initialized globals. The field factories are proved, but the global
package initializer and its connection to those factories are **not** yet proved.
The populated examples do not cover arbitrary Header fields, widths or suffixes.
The field-table proofs currently require exclusive ownership and return it unchanged;
shared read-only table ownership for concurrent decoder calls is not yet established.

The BigInt decoder now packs bytes into owned Go words, uses `SetBits` to normalize,
and uses in-place `Neg`. Negative CBOR bignums add one with a Go carry loop. This
removes the decoder's dependence on `Int.Not` and its assembly arithmetic. The real
required math/big implementations are translated locally; no arithmetic helper
correctness theorem has been assumed. **The byte packing, carry loop and complete
CBOR BigInt contract still need proofs.** This change is not yet performance-tested.

`audit.py` rejects `sorryAx` and native evaluation axioms in every pilot theorem's
transitive axiom report and records source hashes. The byte-mask identity is checked
by kernel reduction over all 256 bytes; it does not use `bv_decide`'s native oracle.
None of the checked pilot theorems depends on either rejected class of axioms.
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
64-bit. The new public-API regressions exercise partial-result cleanup for all 24
inline uints and compare null versus empty-array felt results against FX and the
original planner. `go test -short ./core -run '^TestTypedHeader'` passed with these
additions. A further public Header test constructs all five unsigned widths directly,
including non-preferred encodings and width maxima, checks suffix/input preservation,
and tests every truncation inside each extended head. That test also passes without
using an FX-produced expected result. These counts are historical observations, not
promises about future runs.

After closing the zero-to-four-limb dispatch proof, the full regeneration/check
passed with 161 audited lemmas and no new transitive axioms. The complete cursor
short suite passed; the bounded width combinations and both felt fuzz seed suites
also passed with `-race -gcflags=all=-d=checkptr=2`. The new bounded test exercises
11,715 valid inputs independently of FX. These checks do not establish long-array
Skip or malformed-limb behavior, which remain the next felt proof dependencies.

The subsequent atomic Skip work passed full regeneration/check with 169 audited
lemmas and no new transitive axioms. `go test -short ./utils/cbor/cursor` passed;
`TestSkipAtomicWidths` and `TestSkipAtomicRejections` also passed with race and
checkptr instrumentation. They exercise supported head widths, ignored invalid
UTF-8, rejected short simple values/tags/oversized lengths, EOF, repeated errors,
nonzero cursor offsets, and enclosing depths 0, 31 and 32. The recursive Skip
rejection proof remains outstanding.

The recursive Skip success proof subsequently passed full regeneration and the
axiom audit with 183 lemmas and no new transitive axioms. The complete cursor
short suite passed. `FuzzSkipRecursive` builds nested values directly and checks
exact consumption, suffix/input preservation and rollback on a sampled proper
prefix; a 10-second run passed 528,339 executions. Deterministic tests cover 32
nested arrays/maps, 33 consecutive tags, tags interleaved with arrays, and the
wide-felt shortcut at the last permitted level, including every truncation of
those examples. The recursive fuzz seeds and boundary tests also passed with
race/checkptr instrumentation. Recursive rejection is tested here but is not yet
formally proved.

The extended felt composition passed full regeneration/check with 189 audited
lemmas and no new transitive axioms. `FuzzLimbsExtended` passed 466,284 executions
in 10 seconds, generating independent limb widths, array-head widths, nested
extra values, enclosing depths and suffixes, plus sampled proper-prefix failures
that preserve the prior output. `TestTypedHeaderExtendedFelt` agrees with FX and
the planner across all array-head widths, checks ownership/suffix preservation,
and tests cleanup at every truncation of a record containing Number and Hash.
The cursor suite and targeted Header test passed, as did the new tests/seeds
with race/checkptr instrumentation. These malformed-input checks do not replace
the outstanding rejection proofs.

The payload milestone passed full regeneration/check with 196 audited theorems
and no added transitive axioms. The cursor suite passed; `FuzzBytePayload`
completed 301,972 executions in ten seconds, exercising all length widths,
nonzero offsets, depth up to 32, exact suffixes, byte borrowing/copying and
retryable truncation. The explicit string vectors cover Unicode scalar bounds,
overlong encodings, surrogates, invalid continuation bytes and values above
U+10FFFF, placed around the validator's ASCII batching thresholds. Every body
truncation is checked before UTF-8 validation. These tests and fuzz seeds passed
with `-race -gcflags=all=-d=checkptr`. The generic slice-index Go witness runs and
Goose translates it; its actual UTF-8 word-helper Lean reproducer fails at the
reversed operands as documented below. No semantics patch has been applied.

## Whole-library gates after the Header milestone

1. **Complete feasibility:** connect the proved field factories to global
   initialization; verify witnesses for nested slices/maps, tagged interfaces,
   UTF8 and the complete BigInt helper. The populated Number path, a late failure
   with cleanup, and an empty-array felt pointer conversion are checked. General
   helper contracts, shared table ownership and the complete semantic dependency
   audit remain unfinished. Legacy planner types still use runtime `cursor.NewFields`.
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

## Proposed fix: generic slice indexing in the pinned semantics

Status: **awaiting explicit approval; no trusted source has been changed**.
This is a concrete prerequisite for proving the translated Go 1.27
`unicode/utf8.Valid` implementation, whose generic `word[T string | []byte]`
helper instantiates `Index T` at `[]byte`.

At pinned Perennial `84d5380639a38f56cd517f1e81246fad7be5b21b`,
`Perennial/Golang/Defn/Slice.lean:159–161` accepts `Index (slice, index)` but
expands it into `Load (IndexRef (index, slice))`. The `index_ref_slice` rule
at lines 152–157 requires `(slice, index)`. The first load in the word-helper
proof therefore stops at `IndexRef (#(W64 0), #data)`. This is an incomplete
model path: Go executes the operation, while the intended semantics cannot
continue using the existing slice-reference rule. It is not evidence of a Go
runtime bug or a failure of ordinary slice indexing proofs, which generally
translate directly through `IndexRef`.

Evidence and reproducibility:

- `go run ./verification/fastcbor-proof/repros/genericindex` succeeds for a
  byte slice, a named byte slice and a string. The generic source is deliberately
  small: `func First[T ~string | ~[]byte](s T) byte { return s[0] }`.
- Running the pinned Goose on that package emits `Index T (s, 0)`.
- From this proof directory, after a normal `lake build ProofCheck`,
  `lake env lean repros/Utf8Word.lean` is an **expected failure** that exposes
  the reversed tuple in the actual translated word helper. This reproducer is
  outside the checked/audited proof target and is not counted as a theorem.
- `repros/generic-slice-index.patch` is the proposed one-line correction:
  `glv((#i, #s))` becomes `glv((#s, #i))`. It has not been applied.
  It is a zero-context patch; applicability was checked using
  `git apply --unidiff-zero --check` against the untouched pin.

Approval scope and validation plan:

1. Create a separate Perennial repair worktree based on the pinned revision.
   Apply that exact one-line correction to the existing `index_slice` rule.
   No new semantics rules, Goose branches, trusted models, axioms or negative
   translation tests are required. The trusted delta is one changed line.
2. Add checked regression proofs for generic byte-slice reads, including a
   named slice, preservation of input ownership and the existing bounds-panic
   behavior. Retain a string instantiation as a sibling regression. Array,
   string and map indexing rules were inspected; none has this reversed
   slice-reference tuple. No other occurrence of this reversed `IndexRef`
   pattern was found in `Golang/Defn`.
3. Run the repair's Lean checks (`etc/lean-ci.sh -q`), inspect
   `etc/trusted-delta.py`, audit the regression axioms, and perform the trusted
   change review. Prove the actual eight-byte UTF-8 helper against the repaired
   rule, then continue its caller's proof. No validator-correctness assumption
   or hand-edited generated code substitutes for those proofs.
4. Record the approved decision in Perennial's fix-semantics examples. Commit
   the local repair and explicitly repin this proof harness to that commit;
   regenerate and audit the Juno proofs. Publishing upstream is outside this
   approval request. The original and benchmark baseline worktrees remain
   untouched.

This follows fix-semantics Case 1 (complete the existing slice scheme) and
Case 9 (inspect sibling indexing rules and keep one logical bug per change).
It excludes unrelated semantics defects, translator changes, compiler/runtime
verification and changes to Juno's decoder behavior. The requirement for a
separately approved plan comes from the active goal and the pinned
`.agents/skills/fix-semantics/SKILL.md`, step 3: “Never change trusted files
before the user signs off.” Independent payload, schema and decoder proof work
can continue while this proposal is pending.
