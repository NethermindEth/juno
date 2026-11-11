# Typed fastcbor proof pilot

This directory is an **incomplete first stage** of the typed-decoder verification plan.
The feasibility gate has not passed. There is no whole-library correctness theorem.
The frozen whole-library performance gate passes for the recorded candidate sources;
formal verification remains incomplete.

## Source and scope

- Baseline: `a4a67242cc67f3855915a97bf4e79a97d217fae4`
  (`origin/danielntmd/cbor-fastcbor` when the task started).
- Worktree: `cbor-fastcbor-verified`, branch `danielntmd/cbor-fastcbor-verified`,
  based on the earlier assessment commit `2f33ecf2f`.
- Perennial/Goose: the commit in `perennial.rev`; Lean: `lean-toolchain`;
  Go: 1.27.0, linux/amd64. Upstream semantics are unchanged.

`utils/cbor/cmd/cborgen` reads the actual Go types and emits typed Header/GasPrice,
index, receipt, transaction, state-update, class and projection readers in core. Unknown field shapes fail generation.
The generated entries use typed stores, allocation, and zero checks instead of
runtime field offsets and runtime allocation hooks. The registry installs Header, its five projections, BlockTransactionsIndexes,
TransactionReceipt (value and pointer), both receipt projections, Transaction and
transactionHashProjection, StateUpdate and ClassDefinition.
The transaction and class adapters capture the actual registry tags in immutable
value profiles; the direct decoders take those profiles explicitly. The routing adapter, registration,
logging, FX fallback, and diagnostic formatting are outside the proposed proof scope.
The generator also emits private static field metadata and small allocation
factories in `core/internal/cborschema`; the typed Header and GasPrice readers no
longer sort or fold field names at startup.
The cursor was moved to `utils/cbor/cursor` so generated readers in core can use it.

`core.DecodeHeaderFirst` enforces a zero destination, returns the original unread
suffix, and clears the destination on a parse error. `core.DecodeHeader` additionally
rejects trailing data. A nonzero destination is rejected unchanged. Duplicate struct
keys are unsupported, as in the baseline planner; they are not silently accepted
with first-wins semantics. The generated projections retain the planner’s separate
first-wins policy through `NextWanted`; their full formal contracts remain open.

## Immediate target: complete Header

The next target is one general Header contract, covering all 15 fields and its
GasPrice children, arbitrary supported field order/widths and suffixes, and
failure cleanup. It is not a proof about one populated sample. Header needs
unsigned integers, UTF-8 strings, felts, nested signature slices, and GasPrice;
BigInt, tagged transaction interfaces, and typed Go maps are later library work.
Header remains the first complete-record proof milestone. Typed implementation
work has progressed while its trusted-model repair awaits approval. The
whole-library performance gate has passed for the recorded candidate Go sources.

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

Latest checkpoint: both public Header APIs have checked contracts for **every byte
input and every owned initial destination**, against `HeaderFirstSpec` and
`HeaderFullSpec`, conditional on `Utf8ValidatorContract`. The destination guard is
fully composed. The complete actual UTF-8 validator loop is now checked conditional
only on `Utf8WordWindowContract`, and the implication from that smaller helper
contract to the validator contract is checked. The actual eight-byte generic helper
remains unproved because of the documented pinned slice-index defect. The proposed
trusted repair is still unapplied; no unconditional Header proof is claimed.

The latest full-check evidence is recorded under the performance and signed-integer
checkpoints below. The theorem count is not a coverage percentage. See
“Complete UTF-8 loop reduced to the word helper” below for exact
scope, unresolved premises and reproduction evidence.
The checked inventory records all 15 manifested destinations as registered typed
readers and retains all 185 frozen benchmark IDs. BlockTransactionsIndexes now also has conditional all-input public API contracts;
its independent specification now has unique outcomes and success/rejection separation
on representable input lengths. Other records
do not yet have complete formal decoder contracts. See “Checked remaining-entry inventory” below.
The map feasibility witnesses now prove the resource-key operation sequence and
expose missing array-value equality for felt-key maps; see the map-feasibility
section below. Twelve transaction-interface witness contracts now also check
boxing and type-switch recovery for the five concrete pointer variants, including
typed nils and unknown-type handling. These are independent of the unresolved
Header helper; they do not prove transaction payload decoding.
The BigInt dependency work now checks the complete public `Cursor.BigInt` decoder
for every byte input at any valid cursor position, including all integer forms,
owned output, exact consumption and every specified rejection/rollback. Its
head, payload, magnitude, category identity and `errors.Is` dependencies are
proved under the documented pinned model. See “Complete public BigInt decoder”.
The frozen-corpus smoke now classifies every case on the baseline and current
candidate: 177 compatible fast decodes, 6 removed-projection cases and 2 controls
with no decoder calls on each. This is saved, source-specific test evidence; the
performance gate has also passed, while the remaining complete record proofs
are still open.
The same corpus and no-fallback instrumentation checks also pass on both revisions
with `-race -gcflags=all=-d=checkptr=2`; these are runtime checks, not formal safety
or performance results.

| Order | Work | Exit criterion | Current status |
| --- | --- | --- | --- |
| 1 | Formalize the complete Header application contract | Independent definitions of accepted encodings, decoded values, rejection cases and ownership, covering all 15 fields and GasPrice; the checklist above is represented formally | Complete independent relations, all-input classification, success/error separation and outcome uniqueness are checked for Header and GasPrice. These specification results do not assume FX behavior. |
| 2 | Finish primitive error contracts | Reserved/invalid heads, wrong types, truncation, excessive counts and depth have the specified result, cursor state and error information | Complete primitive/head/count/depth errors and ownership/progress contracts are checked. Text UTF-8 rejection composition still has the validator premise described below. |
| 3 | General felt and UTF-8 contracts | Fast and slow felt paths agree on limb contents, consumption and failure behavior; text validation and conversion have proved contracts | General felt success/failure and both fast/slow paths are checked. The complete production UTF-8 loop is now checked conditional only on Utf8WordWindowContract; the actual eight-byte generic helper remains unproved because of the pinned indexing defect. Text conversion/composition uses the resulting validator contract. |
| 4 | General field lookup and Skip | Every supported field order, width, case variant, unknown key/value, duplicate and nested skip is covered, including rollback/error behavior | General recursive Skip is checked, preserving its intentional ignored-text and chained-tag policies. Complete field lookup, key widths/case folding and duplicate handling compose for both schemas conditional on the UTF-8 validator; no other key-helper WP premise remains. |
| 5 | Nested outputs and GasPrice | Arbitrary signature lengths and contents, nil/empty distinctions, disjoint owned elements in shared allocations, and partial-construction errors are covered | General signature allocation, nil/empty distinctions and cleanup are checked. Complete GasPrice records, shared slots, both Header price fields and all failures compose conditional on the UTF-8 validator. |
| 6 | Initialization and immutable table sharing | Relevant actual initialization establishes the table contents and shareable ownership needed by independent decoder calls | Relevant cursor, schema and cborerr initialization is proved after modeled runtime CPU startup. Exact immutable tables and canonical errors come from initialization; no initializer WP premise remains for Header. Runtime/hardware startup and unrelated core/planner initialization remain outside scope. |
| 7 | Header loop and public APIs | Compose the preceding contracts into general theorems for cborReadHeader, DecodeHeaderFirst and DecodeHeader, including arbitrary supported records and all specified rejection cases | Complete actual Header loops and both public APIs now compose for every byte input and every owned initial destination, including guard rejection, exact suffixes, read-only input, owned results/errors and cleanup. The two API contracts still explicitly require Utf8ValidatorContract, now reduced to the unproved word helper; the Header milestone is not closed. |
| 8 | Reproduction and validation | Regenerate from the current Go source, Lean-check the full dependency chain, audit axioms/assumptions, and pass the applicable Go regression/property/corpus checks | Full regeneration, dependency audit and archived performance verification pass at the checkpoints below. All 15 manifested destinations have typed readers; compatibility evidence includes ordinary/race/checkptr checks. All 177 comparable performance cases pass. Explicit unresolved premises remain reported separately; the final unconditional Header check is still pending. |

This is dependency order, not eight isolated phases. Nested GasPrice uses field
lookup, and felt extras use recursive Skip. The independent record specifications
cover every input and have checked deterministic outcomes. All Header composition
work is now connected, but the UTF-8 helper premise must be discharged before the
Header milestone can pass. BigInt, tagged transaction interfaces and ordinary Go
maps remain later library work. The existing Go differential/property checks
protect compatibility; neither they nor the conditional Lean proofs satisfy the
frozen whole-library performance gate.

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
  `FeltComplete.lean` now supplies the general typed felt contract described below.

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
  Malformed/incomplete heads and recursive rejection are covered by the later contracts below.
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
  and scratch. This is the success direction; the rejection and arbitrary-input
  contracts below complete the public outcome characterization. It does not
  assume a correct recursive child scanner.
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
  output ownership. This supplies the success direction of the complete outcome
  contract below.


- `FeltReject.lean`: the actual slow loop consumes any valid prefix of zero to
  three unsigned limbs, then propagates a proved Uint64 error. It restores the
  original cursor/depth, preserves the destination and input, and retains the
  diagnostic's inner offset. It also supplies shared non-inline-four public
  dispatch and a general Uint64 EOF contract that performs no input loads.
- `FeltShortcutReject.lean`: `UnsignedReject` describes absent, non-unsigned,
  reserved or incomplete unsigned input independently of the shortcut. The
  actual mixed-width shortcut declines such input after any valid prefix shorter
  than four limbs, returning zero without writing the destination. The invariant
  bounds its local offset by 28 at the rejecting limb, so all width-check additions
  are safe. Earlier valid words reside only in a temporary array.
- `FeltWideReject.lean`: a pure wire proof excludes a longer complete sequence of
  nine-byte unsigned items when an earlier prefix ends in `UnsignedReject`. It
  proves that the four fixed wide-felt guards cannot all match a failing prefix
  when the 37-byte input window is available.
- `FeltInlineReject.lean`: checks the actual wide-path dispatch and all four guard
  loads, calls the proved mixed-width rejection and then the actual slow-loop
  error contract. The typed prefix-error composition now covers every array-head
  width, including inline `0x84`, with exact outer rollback and owned diagnostics.
- `FeltLimbReject.lean`: closed typed-reader type/tag, incomplete-argument and EOF
  contracts instantiate those wrappers with the actual proved unsigned readers.
  Pure lemmas connect non-unsigned and incomplete heads to `UnsignedReject`.
  These replace the earlier non-inline-four-only contracts. Prefix limb widths
  and array counts larger than the prefix remain arbitrary, including large
  unsigned counts. The old typed pointer is preserved; temporary partial words
  are not published. Malformed limb-head composition is now proved as described
  below; remaining array-header failure cases still need composition.
- `FeltExtraReject.lean`: the actual slow loop reads four arbitrary-width limbs
  and any valid ignored prefix before propagating a proved Skip error. This is
  composed through public Limbs dispatch and the typed felt reader; arrays with
  an extra element necessarily bypass both four-limb shortcuts. Closed contracts
  cover EOF and all `SkipReject` atomic cases (invalid two-byte simple values,
  short byte/text bodies and tags 0–3), preserving the original cursor, depth,
  input and old output pointer. Atomic diagnostics retain the position after the
  offending inner head; EOF retains the end position. The generic composition
  lemmas take explicit leaf-proof arguments, and the closed contracts discharge
  them with actual proved helpers. Malformed-head errors now compose as described
  below; recursive Skip failures remain pending.

`TestLimbsErrorsAfterPrefix` checks public Limbs errors after zero to seven valid
items, all array-head widths, mixed limb widths and nested valid ignored values.
Repeated failures at nesting depth 31 check restoration through observable error
behavior; sentinel outputs and the original bytes must stay unchanged. It checks
all cuts through extended arguments, EOF, type/tag, reserved/break and atomic Skip
errors. `TestTypedHeaderFeltErrorsClearPartialResult` checks both public Header
APIs with a Number field before the failing Hash field: the complete result is
cleared, First returns a nil suffix and the inner diagnostic offset is retained.
Inline-four type/tag/truncation/EOF cases have formal composition as above;
malformed-head cases now have composition as described next.
`TestLimbsRejectsWideShapePadding` supplies at least 37 bytes and a bad limb after
zero to three wide unsigned values, with later bytes imitating wide-felt heads.
It checks each offending limb independently, exact error offsets, repeated-read
rollback, unchanged sentinel outputs and preservation of input bytes.


`HeadFault.lean` records the independent reserved/indefinite head cases and their
error categories. `HeadFaultError` owns the exact diagnostic, including variadic
arguments for invalid additional information. The closed slow-head contract
instantiates the earlier classification proofs; shared composition lemmas carry
it through expectation for any major type and public Uint64 without consuming
input. They preserve error precedence over a destination-type mismatch.

`FeltHeadFault.lean` proves the head classification covers every information code
above 27, connects it to short-limb rejection, and composes malformed limbs after
any valid prefix through all felt shortcuts and the typed reader. It also proves
malformed outer-array heads preserve the old pointer/cursor/input at every depth:
head errors precede the array-depth check, including at depth 32. The shared
outer-head adapters accept actual expectation proofs. The closed outer
wrong-type/tag, truncated-argument and EOF contracts are described below;
null/undefined remain valid nullable felt values.

`SkipHeadFault.lean` verifies actual Skip shortcut/peek dispatch and public
rollback on malformed heads at arbitrary depths and offsets. This composes into
the typed felt reader after four limbs and any valid ignored prefix, preserving
the inner error's original offset and rolling back the outer array. Recursive
Skip rejection remains pending; incomplete argument heads are covered below.

`TestMalformedHeadsInFeltAndSkip` exhausts all reserved/indefinite head bytes as
outer felt/Skip inputs and after zero to seven valid felt items. It covers all
array-head widths, nested valid ignored prefixes, empty and long suffixes,
repeated failures, unchanged input/output, exact offsets and observable depth
restoration. Outer head errors are also checked at depth 32 to verify precedence
over depth rejection. This is regression evidence alongside the contracts, not
a complete arbitrary-input rejection theorem.

`FeltOuterReject.lean` closes outer type/tag, incomplete-argument and EOF
contracts through the actual typed felt reader. These preserve the old pointer,
whole input and original cursor, with owned diagnostics at the original offset.
They apply at any nesting depth: these failures precede the depth check. EOF
passes through actual Null, both public Limbs depth branches, the empty short
shortcut and slow expectation; it leaves the prior pointer intact, while actual
null/undefined are successful values setting it to nil.

`SkipHeadTruncated.lean` proves incomplete arguments for every major type through
the actual worker and public Skip. The closed contract composes into the typed
felt reader after four limbs and arbitrary valid ignored contents, preserving
the incomplete head's initial diagnostic offset while restoring the outer cursor,
depth and old output pointer. The later unified rejection/classification contracts
below compose this with the other error classes.

`TestIncompleteHeadsInFeltAndSkip` checks every major type and every cut through
extended heads at depths 0, 31 and 32 through both public APIs.
`TestLimbsOuterRejection` checks EOF and complete wrong-type/tag heads at every
argument width, nonzero offsets and the same depths. Both tests repeat failures
and check unchanged output/input and cursor rollback. Existing ignored-tail tests
exercise incomplete heads after mixed-width limbs and valid nested extras.

`SkipTagReject.lean` proves the actual accepted-tag failure dispatch with an
explicit child-proof argument. `SkipTagPrefix.lean` defines an independent tag
prefix relation, including each encoded head, the tag-number restriction and
chained-tag depth accounting. Induction over that relation proves every
intermediate recursive call, preserving the leaf's final cursor and owned error.
The closed malformed-head worker/public contracts discharge the leaf with the
actual head proof; the public wrapper restores the complete original cursor.

`SkipTaggedErrors.lean` similarly closes incomplete-head and atomic-rejection
contracts after arbitrary supported tag prefixes. Atomic errors retain their
post-head offsets; malformed and incomplete heads retain their initial offsets.
`SkipTagBoundary.lean` proves actual Peek EOF, missing tagged content and a tagged
child at the depth limit. The missing-content proof follows Peek's ignored error
through the actual recursive worker, which allocates the returned EOF diagnostic.
The depth proof owns the error's limit argument. `SkipTaggedBoundary.lean`
composes these boundary failures through arbitrary supported prefixes and public
rollback. All these proofs preserve whole-input ownership and cursor depth and
scratch state. Recursive array/map rejection and arbitrary-input classification
are supplied by the later contracts below.

`TestSkipTaggedErrors` checks mixed head widths and 0–34 tags at enclosing depths
0, 1, 15, 31 and 32, ending in EOF, malformed/incomplete heads and atomic failures.
It checks precedence when a tagged child would exceed depth before its head is
read, exact inner offsets, repeated-read rollback and unchanged input.
`TestTypedHeaderTaggedErrorsClearPartialResult` checks both Header APIs after a
Number field, with tagged failures in an unknown field or an ignored felt value;
the result clears, First returns nil and the inner offset survives.

`SkipElementsBoundary.lean` proves depth rejection before count or child access,
and oversized-count rejection before multiplication. `SkipContainerReject.lean`
composes the actual array/map dispatch with an explicit loop proof, checking
all outer wide-shortcut guards when depth permits that shortcut.
`SkipContainerBoundary.lean` closes both worker/public boundary contracts for
all array/map head widths, including unsigned counts above the signed range.
A wire lemma excludes the outer four-limb shortcut on count-truncated input;
at depth 32 the production guard disables it directly. Diagnostics use the end
of the container head, and public Skip restores all cursor fields.

`SkipElementsReject.lean` proves actual loop error propagation after an arbitrary
valid prefix. Its invariant counts remaining children without multiplication
wraparound, tracks exact cursor progress and preserves the whole input. Valid
children use the proved recursive acceptance contract, including the loop's own
inlined wide-felt shortcut. A supplied leaf proof and a wire property excluding
that shortcut discharge the failed-child branch; these are explicit composition
premises, not assumed decoder correctness.

`SkipContainerPrefix.lean` derives the count/multiplication bounds from remaining
input and composes that loop through actual array/map dispatch. Pure wire lemmas
show that valid skipped contents before a malformed head cannot satisfy the
outer four-limb shortcut, including inline `0x84`. Closed worker/public contracts
instantiate every premise for malformed heads after arbitrary valid nested
prefixes, retaining the exact inner diagnostic and whole-input ownership.
The recursive and unified contracts below extend this to failing nested children
and the other leaf-error classes.

`TestSkipContainerBoundaries` covers all head widths, short bodies, counts through
max uint64 and depth 32 precedence, including empty containers.
`TestSkipContainerMalformedChildren` covers arrays/maps, all head widths, later
malformed children, valid wide felts and nested maps before the failure, loop
shortcut/depth boundaries, long fast-path windows, repeated rollback, exact
inner offsets and unchanged input.

`SkipHeadFailure.lean` gives an independent recursive relation for malformed
heads reached through arrays, maps and tags after arbitrary valid nested
siblings. Count-fit and depth conditions specify when the inner diagnostic is
reached instead of an earlier boundary failure. Its pure lemmas prove the input
is nonempty, the diagnostic offset lies inside it and no enclosing wide-felt
shortcut can match, including inline-four arrays.

`SkipHeadFailureSound.lean` proves the actual worker by induction over that
relation. Every recursive child obligation is discharged; the closed statement
has no helper-WP or shortcut-exclusion assumptions. Public Skip restores all
original cursor fields and input ownership while owning the exact inner error;
it also proves the diagnostic offset does not wrap and lies within the input.
`FeltRecursiveReject.lean` composes this result through arbitrary valid ignored
felt prefixes, restoring the whole array cursor/depth and prior output pointer.
This is the recursive malformed-head soundness theorem. The later shared-context
and classification contracts below also cover incomplete arguments, atomic/count/
depth failures and EOF.

`TestSkipRecursiveMalformedHeads` checks all reserved/indefinite head bytes
inside arrays, maps and mixed tag/container paths through the nesting limit,
with mixed head widths, valid siblings, long shortcut windows and nonzero
starting offsets. It checks repeated rollback, exact inner diagnostics and
unchanged input. `TestTypedHeaderRecursiveMalformedHeadsClearPartialResult`
checks both Header APIs after a Number field, with 1, 8 or 30 nested tagged
containers in an unknown field or an ignored felt value; results clear, First
returns nil and the innermost offset survives.

`SkipFailureContext.lean` factors supported enclosing containers/tags and valid
siblings into a relation independent of the eventual error class. Its wire
properties distinguish a complete wide unsigned value from EOF or an incomplete
`0x1b` argument. The actual felt shortcut also requires a 37-byte window and depth
below 32; earlier generic dispatch/loop adapters now use exactly those conditional
exclusions. This admits short inputs whose head bytes match every wide guard and
wide-shaped children rejected at the depth limit, while retaining all existing
closed proofs.

`SkipContextSound.lean` proves worker and public propagation by induction over
that context relation, requiring an explicit proved leaf contract. Pure context
lemmas discharge intermediate shortcut exclusions. `SkipContextErrors.lean`
closes public contracts for incomplete arguments of every major type, atomic
failures and EOF after valid siblings. All preserve the original cursor and
whole input, with owned diagnostics at the original inner head, after the inner
head or at EOF respectively. The closed contracts instantiate actual leaf
proofs and leave no helper-WP or shortcut premises to callers. The remaining
count/depth and missing-tag-content instances, and the arbitrary-input contract,
are supplied below.

`TestSkipRecursiveTruncationAndAtomicErrors` checks all cuts through extended
heads of every major type, atomic errors and EOF under tagged arrays/maps, valid
wide siblings, nonzero starting offsets and depths through the limit. Its
inline-four cases include an incomplete fourth wide uint: all head checks may
match, but the fewer-than-37-byte window must prevent the fast path. Tests repeat
failures and verify exact diagnostics, full cursor rollback and unchanged input.

`SkipContextBoundary.lean` closes count, container-depth, tag-depth and missing
tag-content failures through the shared contexts. It includes full wide-shaped
arrays at depth 32, empty-container depth failures, and tag-depth precedence over
an incomplete child head. Every contract preserves original public cursor/input
ownership and exact owned diagnostics.

`SkipFailureSpec.lean` unifies all specified leaf classes and enclosing contexts
in `SkipFailure`: malformed and incomplete heads, atomic rejection, EOF, count,
container depth, missing tagged content and chained-tag depth. Its independent
reason type specifies error category, diagnostic text and owned arguments.
Pure offset lemmas include EOF and bound every specified diagnostic by the
remaining input length. `SkipFailureSound.lean` instantiates all existing actual
reader proofs into `wp_Skip_failure`; its bounded variant additionally proves
nonwrapping offsets inside the original input or at its end. No helper-WP or
shortcut-exclusion assumptions remain in this public theorem.

`FeltSkipFailure.lean` composes the unified rejection theorem through ignored
felt contents after arbitrary valid prefixes, preserving the exact inner error
while restoring outer cursor/depth and the prior output pointer.

`HeadClassification.lean` exhausts every possible byte and remaining input: a
supported head with exact argument/suffix, a malformed/unsupported head, or an
incomplete argument. `SkipClassificationSupport.lean` extends classified child
failures through supported containers/tags and classifies requested child sequences.
`SkipClassify.lean` proves `skip_classify` for every finite input at depths 0 through
32 by induction on input length. It yields a `SkipValid` prefix with exact suffix
or a `SkipFailure` derivation, accounting for depth/count precedence and all leaf
errors. No chosen-outcome or recursive helper hypothesis remains.

`SkipComplete.lean` composes this exhaustive classification with the checked
production contracts. `wp_Skip_complete` takes arbitrary unread bytes and returns
`SkipOutcome`: either exactly one valid item is consumed with its exact suffix,
or the original cursor is restored with an owned specified diagnostic and a
nonwrapping inner offset. Full input ownership is preserved in both cases. Package
initialization and immutable ownership of the four error globals remain explicit
preconditions. This is a partial-correctness/model-safety contract, not termination
or strict RFC validation of ignored text. Separate pure proofs of relation
uniqueness/disjointness have not been added. The general Header composition is
still pending.

`FeltFailureSpec.lean` defines the independent felt rejection relation, including
outer expectation errors, depth rejection, unsigned-limb errors and recursively
failing ignored extras. Null/undefined are excluded from outer errors because
that success branch runs first. Unlike generic Skip containers, felt arrays do
not preflight their declared count against available bytes. Error offsets include
EOF and are bounded by the original unread input length.

`FeltClassify.lean` exhaustively classifies expected heads, unsigned sequences and
then arbitrary felt input at depths 0 through 32. Its accepted values are the
existing `FeltValid` relation: nil versus allocated zero, zero-filled short arrays,
four retained unsigned Montgomery limbs and recursively skipped extras. All head
widths, arbitrary suffixes and oversized counts are covered. There are no assumed
recursive-classification premises.

`FeltFailureSound.lean` composes all actual production leaf/recursive proofs into
`wp_readFelt_failure`, with exact owned diagnostics, nonwrapping offsets and full
cursor/old-pointer/input preservation. `FeltComplete.lean` combines this with the
success proof and exhaustive classification: `wp_readFelt_complete` applies to
arbitrary input and returns `FeltOutcome`, specifying the owned result and exact
suffix on success or the unchanged prior pointer/cursor and owned inner error on
failure. No chosen-outcome or helper-WP premise remains. Package initialization
and immutable ownership of the five error globals remain explicit preconditions.
These are partial-correctness/model-safety claims; termination and pure relation
uniqueness/disjointness are not established here. Shared slice/GasPrice allocations
and full Header composition are still separate work.

`ExpectedFailure.lean` closes the actual major-type expectation and public Uint64
failure contracts for the independent expectation relation. Pure lemmas exclude
both an inline-four array on array-head failure and a successful unsigned shortcut
on a retained-limb failure. `LimbsReject.lean` supplies in-place composition
adapters for failures in retained limbs or ignored extras. `LimbsFailure.lean`
discharges their helper premises using the actual expectation, Uint64 and recursive
Skip contracts, preserving the caller's four-word array and original cursor/input.

`LimbsComplete.lean` reuses the typed felt array/value and error specifications.
The in-place API requires an array: null/undefined are type failures rather than
nil results. `limbs_classify` exhausts arbitrary input, and `wp_limbs_complete`
composes the actual success/failure proofs into `LimbsOutcome`. Success updates
the owned four-word array and leaves the exact suffix; failure preserves the old
array/cursor with an owned specified diagnostic and bounded nonwrapping offset.
Whole-input ownership is preserved. Package initialization and immutable ownership
of the five error globals remain explicit preconditions.

`LimbsSlice.lean` applies the production Limbs contract to one borrowed slot of an
arbitrary owned backing slice. Success restores the whole slice with exactly that
slot updated. Failure restores the original whole slice, including earlier decoded
elements. These contracts provide the element-level ownership step for signature
decoding. The next contracts compose that step through the inner production loop.

`SliceAllocation.lean` strengthens the existing slice-allocation contract with the
actual allocator's nonnil pointer guarantee, including zero-length allocations.
It proves the existing translated `make` operations using unchanged pinned
semantics. The distinction matters because ordinary slice ownership alone can
also describe nil empty slices.

`FeltPointersSpec.lean` independently defines nullable felt-pointer slice encodings
and values. The ownership predicate includes both the pointer table and its shared
backing allocation: each nonnil pointer names its corresponding four-word slot,
while nil entries retain zeroed unpublished storage. An allocated empty array is
distinguished from a null/undefined result. Pure length/prefix lemmas justify
allocation/count bounds and one-element updates.

`FeltPointers.lean` proves the actual `cborReadFeltPointers` success loop for
arbitrary lengths and nullable felt elements, including all supported outer/child
head widths, arbitrary valid ignored extras and exact suffixes. Both actual
allocations, the range loop, null-element continuation, Limbs calls, pointer
publication and final Leave are checked. The loop preserves all earlier pointers
and backing values, whole-input ownership and cursor bounds. The general nullable
wrapper theorem returns `FeltPointersResult`, preserves nil versus allocated empty
and restores the caller's nesting depth. No assumed loop-body/helper proof remains.
Pointer-slice rejection/partial-construction errors, the actual outer
`cborReadSignatures` loop and GasPrice shared ownership are still pending.

`TestSkipRecursiveBoundaryErrors` checks mixed-width tagged arrays/maps leading
to count errors, missing tag contents and depth-limit failures, including wide
felt bytes, empty containers, depth-before-count and depth-before-incomplete-tag
precedence. Nonzero starting offsets, repeated rollback, inner diagnostics and
unchanged input are asserted through public Skip.

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

`FieldFold.lean` proves the actual case-folding loop for arbitrary equal-length
byte sequences: its result is exactly equality after folding ASCII uppercase
letters. Non-ASCII bytes remain unchanged. `FieldMatch.lean` composes this with
both actual lookup passes: success returns a nonnegative index of a matching
name, and failure excludes every name in the table. It also proves that distinct
folded names give a unique successful index, and kernel-checks that property for
the complete Header and GasPrice name lists. `FieldMark.lean` proves that a new
field sets exactly its bit and that duplicate rejection preserves cursor/struct
state while returning an owned error and diagnostic name. Both name matching and
duplicate rejection preserve arbitrary fractional ownership of their read-only
metadata; actual initialization still has to establish that ownership.

The original `a += 'a' - 'A'` and corresponding `b` expression translate into
unsupported untyped-rune arithmetic in the pinned Goose. The Go helper now uses
`const lowerDelta byte = 'a' - 'A'` in both assignments. Regeneration emits
`W8 32`; the proofs check the translated production helper. This changes neither
the intended folding policy nor the trusted tooling. The pending generic
slice-index repair for the standard-library UTF-8 validator is separate.

`KeyInteger.lean` proves integer-key classification for arbitrary unsigned and
negative integer heads, all five argument widths, arbitrary cursor offsets and
suffixes. Arguments below 2^63 are consumed as non-text keys; larger arguments
return an owned type error and restore the original cursor. It checks the byte
subtraction in public `Key` so non-text heads cannot enter the text fast path,
and composes both outcomes through `otherField` (unknown versus error).
`KeyReject.lean` proves wrong-type/tag diagnostics and short text-body rejection,
including oversized length arguments. The public truncation proof follows both
short-text fast-path cases and every wider length header before reaching the
slow worker. The original cursor and whole input are preserved on each error;
the alternate-field path propagates the error without accessing struct metadata.
These are contracts for the actual translated implementations. The successful
short-ASCII-key path and head-error propagation are now proved as described
below; wider/non-ASCII successful text keys and UTF-8 still need proofs.

`ASCII.lean` proves the complete production ASCII helper for arbitrary byte
sequences and lengths. Its independent contract is that every byte is below 128.
The proof connects all eight high-bit mask positions to that predicate, then
verifies the word loop and tail loop, retaining ownership of the entire original
input despite local reslicing. `KeyASCII.lean` composes the helper through public
`Key` for inline and one-byte text lengths, including non-preferred lengths and
empty keys. It returns the exact borrowed key slice, preserves the whole input,
and advances without signed overflow to the exact unread suffix.

`KeyFields.lean` connects these short ASCII keys to the actual `otherField`,
two-pass matcher and bit marking. Its `FieldReadOutcome` distinguishes unknown
names, new known fields and duplicates. Unknown names preserve struct progress;
new fields set exactly their seen bit; duplicates preserve struct progress and
return an owned diagnostic containing the matched name. Duplicate errors occur
after consuming the key, so the cursor and diagnostic offset point to the value.
All outcomes preserve fractional read-only metadata and the whole input. This
handles arbitrary ASCII key bytes and up to 64 names; the existing folded-name
uniqueness proof identifies the matching field for Header/GasPrice tables.
Successful wider text heads, non-ASCII keys and actual initialization remain
separate obligations. Cache coverage is described next.

`CacheWords.lean` proves that the three masked little-endian comparisons are zero
exactly when the input has the encoded key as a prefix, for keys of at most 24
bytes and inputs with at least 24 bytes. Every byte inside the key contributes;
bytes after its end cannot affect the result. Separate checks connect the actual
Header/GasPrice word and mask arrays to this byte-level contract. `CacheFast.lean`
proves the production `NextField` cached-hit branch for a matching, unseen short
key: exact cursor advancement/suffix, one pair consumed, the expected table index
advanced once, and precisely that field's seen bit set. It preserves the whole
input and arbitrary fractional ownership of both table metadata and entries.
The actual generated Header/GasPrice entries satisfy its cache representation.
Both branches of nonempty `find` are now proved for the Header/GasPrice table
shape, as described below, and the public hit/miss cases now compose into one
ASCII field-reading contract.

The general model analysis exposed an addition-before-check guard in `NextField`:
`c.pos + 24 <= len(c.buf)` can pass after signed overflow even when no bytes remain.
The production guard now checks `len(c.buf) - c.pos >= 24` first. Checked arithmetic
lemmas establish that this is the exact remaining-length condition for a valid
cursor and that it makes the subsequent end-index addition safe. An explicit
signed-limit counterexample records why the old check was insufficient. This is
a model-level boundary finding; it does not claim a reproduced crash with an
allocated Go buffer near 2^63 bytes. `CacheEOF.lean` proves the actual public EOF
path at every nonnegative offset, including the signed limit, for arbitrary
search indices and fractional metadata. It returns the owned truncation error,
preserves cursor/map state except for decrementing the outstanding pair count,
and performs no input byte load. The performance effect of the one-line guard
change remains subject to the frozen gate.

`FindShort.lean` proves the actual `find` loop for every input shorter than 24
bytes, arbitrary valid starting indices and tables with nonempty encodings ordered
by first encoded byte. Its independent `FieldFindResult` says a hit identifies an
encoded prefix in the remaining table; a miss excludes every remaining entry.
The loop invariant tracks earlier misses and proves the sorted early exit cannot
skip a match. It covers empty input, exhaustion, continued search, insufficient
bytes and successful comparison, preserving arbitrary fractional table/input
ownership. Separate finite checks prove ordering, nonempty encodings and pairwise
prefix separation for the actual Header/GasPrice tables. The prefix separation
lemma makes the matching table index unique for arbitrary input and suffixes.

`FindHit.lean` composes the search through the actual slow and public `NextField`
for short-buffer table hits. It consumes the exact encoded key, decrements the
pair count and sets the next search index. A new field sets precisely its seen
bit; a duplicate leaves that bit state unchanged and returns an owned diagnostic
at the post-key offset. Both outcomes preserve whole-input and fractional
metadata ownership and return the exact unread suffix. `FindMiss.lean` composes
short-buffer misses with actual ASCII key parsing and name matching, including
unknown names, case variants, earlier entries and duplicates whenever the encoded
prefix is absent from the remaining table. A miss preserves the search index,
and every outcome decrements the outstanding pair count exactly once. These are
contracts for the translated production paths, not assumed search or mark helpers.
The subsequent word-based search and public miss-dispatch milestone is described
next. Remaining key encodings/errors and global initialization are still
outstanding.

`FindWords.lean` proves the production word-based search loop for arbitrary input
lengths of at least 24 bytes and tables whose nonempty encoded keys fit in the
three-word cache. It verifies the first-byte prefilter, every actual word/mask
load, complete masked comparison, sorted early exit, continued search and
exhaustion. `CachedShortTable` records that representation; separate checks show
that the actual complete Header and GasPrice tables satisfy it. The unified
`wp_find_cached` combines byte-based and word-based search into the independent
`FieldFindResult` contract for arbitrary input lengths and valid starting indices.
Keys longer than 24 encoded bytes need a separate word-path tail-comparison proof
before expanding to any library root that uses them; this restriction does not
exclude any Header/GasPrice table entry.

`FindDispatch.lean` composes the unified search through the actual slow field
reader for arbitrary input lengths. Known encoded-prefix hits consume the exact
key, advance the table index and either mark a new field or return the owned
duplicate diagnostic at the post-key cursor. ASCII misses follow actual key
parsing and name matching with unknown/new/duplicate outcomes and exact suffixes.
`CacheMiss.lean` proves that an expected-key word mismatch cannot be hidden by
seen bits and leaves state/input intact before this slow path. Its unified public
`wp_nextField_miss_ascii` handles the word comparison, short-buffer and exhausted-
table cases, preserving fractional metadata and whole-input ownership. This
closes public miss dispatch for the already supported inline/one-byte ASCII keys
when no remaining encoded-table entry matches. Public hit composition and the
unified ASCII contract are described next; wider/non-ASCII keys, remaining errors
and initialization are still pending.

`CacheHit.lean` completes public table-hit handling for arbitrary input lengths.
It proves that a failed expected-key comparison or a set seen bit reaches the
actual slow search/mark path without changing state first. The combined
`wp_nextField_hit` uses prefix separation to distinguish a new expected field
from later-entry hits and duplicates, covering both word and short-buffer paths.
Every result consumes the exact encoded key, decrements the pair count and sets
the search index to one past the matched entry; duplicate diagnostics own the
field name and use the consumed-key offset.

`FieldRead.lean` links these byte-prefix contracts to the actual schema names.
The complete Header and GasPrice tables pass `InlineFieldTable`: each entry's
index refers to the corresponding name, and its encoded bytes are that name's
canonical inline text encoding. A pure prefix theorem then shows that a cache hit
identifies the input key itself, rather than merely an unrelated byte prefix.
The unified `wp_nextField_ascii` composes every public cache/search path into
`FieldReadOutcome` for arbitrary inline/one-byte ASCII keys, remaining input
lengths and field orders. It covers known-new, duplicate and unknown names,
case variants and non-preferred one-byte lengths, with exact cursor/suffix,
whole-input ownership and fractional metadata. `FieldSearchAdvance` records the
exact index change, and a separate lemma preserves its next-iteration bounds.
This is the field-reader contract for the future Header loop, not that loop's
proof. The public non-text/body-error milestone is described next; wider/non-ASCII
text keys, initialization and the other Header dependencies remain outstanding.
Head-error propagation is now proved as described below.

`FieldFallback.lean` factors the common cache/search fallthrough into reusable
Hoare-composition lemmas. They prove the actual word loads, comparisons, search,
state preservation and one-pair decrement, taking the leaf-reader proof as an
explicit proof argument. The completed public contracts instantiate that argument
with proved production helpers; no helper correctness is assumed in those
contracts. The earlier cached-hit and cached-miss proofs now reuse the same lemma,
removing duplicated word-load proofs. The rejection adapter preserves the exact
cursor state supplied by its leaf proof, including the original offset on the
head-error paths proved below.

`FieldInteger.lean` proves public `NextField` behavior for both integer major types
and all five supported argument widths. In-range keys return unknown, consume the
exact encoded key and preserve the suffix. Out-of-range keys return the owned
range diagnostic and restore the original cursor. `FieldReject.lean` similarly
composes non-key type/tag rejection and text-body truncation, including oversized
unsigned lengths, through the actual public cache/search wrapper. A first-byte
lemma rules out false schema matches for non-text heads; a separate truncation
lemma proves an incomplete text body cannot contain a full cached schema key.
Every case decrements the outstanding pair count once, preserves search/seen/open
state and fractional metadata, and retains whole-input ownership. These contracts
include owned diagnostic arguments where applicable. Wider/non-ASCII successful
text keys and UTF-8 errors still need composition.

`HeadKey.lean` proves propagation of every nonempty failing head through the
actual `head`, `keySlow` and public `Key`: indefinite lengths, break codes,
reserved/illegal additional information and all incomplete extended arguments.
The key fast-path guard proof includes `0x78` without its length byte. The closed
contracts instantiate reusable Hoare-composition lemmas with the actual proved
head helpers; they do not assume helper correctness. Each error preserves the
original cursor and whole input and owns the exact diagnostic and its arguments.

`FieldHead.lean` excludes false inline-schema matches for these heads, then
composes the Key contracts through alternate lookup and the complete public
`NextField` cache/search wrapper. Every error decrements the pair count once,
preserves search/seen/open state and fractional metadata, and retains the original
cursor/input and diagnostic offset. Together with the earlier EOF theorem this
covers key-head failures, but not the remaining text-validation failures.

`TestKeyHeadErrors` exercises every malformed head and every cut through the four
extended argument widths via public Key and field APIs. It covers repeated Key
reads, nonzero offsets, the word-cache boundary, previously consumed fields and
exhausted search tables. A fresh map with only one remaining byte fails the
OpenStruct count check before reaching Key; the tests assert that earlier error
and use a preceding valid pair to reach NextField for those same one-byte keys.
The tests assert observable pair progress after an error, cursor/suffix retention,
input preservation, error categories and diagnostic text.

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
The older field-table examples require exclusive ownership and return it unchanged.
The general name matcher and duplicate-marking contracts accept fractional metadata
ownership, but initialization has not yet established shared read-only ownership
for concurrent decoder calls.

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

The field-name milestone passed full regeneration/check with 204 audited theorems
and no added transitive axioms, plus public-API checks for all 16,384 ASCII key/name
byte pairs. Duplicate tests exercise all 64 bits,
reverse field order, case variants, all five key-length widths and error offsets.
The cursor suite and targeted typed Header tests passed. The new field tests and
existing NextWanted tests also passed with `-race -gcflags=all=-d=checkptr`.
On Go 1.27 linux/amd64,
compiling the isolated old and new `equalFoldASCII` functions produced identical
39-line instruction listings (source locations removed), SHA-256
`4f407862f2de9ad5d091752914bcb12b270e2be2ed1c9ac3a97cac475d8d9d56`.
That narrow compiler check is not the frozen whole-library performance gate,
which has not run.

The key-classification milestone passed full regeneration/check with 221 audited
theorems and no added transitive axioms. The cursor suite and targeted typed
Header tests passed. `FuzzIntegerKey` completed 686,996 executions in ten seconds,
covering both integer major types, all five widths, the int64 boundary, nonzero
offsets, depth up to 32, exact suffixes and retryable truncated heads. Key rejection
and fuzz seeds also passed with `-race -gcflags=all=-d=checkptr`. Deterministic
text-key tests cover every short-body prefix around the inline/one-byte boundary
and lengths above the signed range, with both ASCII and invalid UTF-8 prefixes.
Header integration checks repeated unknown integer keys with nested values against
FX and the original planner, and verifies cleanup/error offsets after oversized
keys. These runtime checks do not replace the remaining general Header proof.
No production decoder or trusted semantics changed in this milestone; the frozen
performance gate still has not run.

The ASCII/alternate-field milestone passed full regeneration/check with 232
audited theorems and no added transitive axioms. The cursor suite and targeted
typed Header tests passed. `TestKeyASCIIPositions` checks all 256 byte values at
every position in lengths 1, 7, 8, 9, 23, 24, 25 and 255, using both short-head
widths wherever representable: 102,400 cases. Single high bytes among ASCII bytes
must be rejected as invalid UTF-8; ASCII cases return the exact key and suffix.
`FuzzTextKey` completed 285,602 executions with a ten-second fuzz budget, covering
all five text-head widths, Unicode and invalid byte sequences, depth/offsets,
input borrowing, exact suffixes and retryable errors. The key tests and text-key
fuzz seeds passed with `-race -gcflags=all=-d=checkptr`. Wider-head and Unicode
runtime coverage does not imply a completed Lean validator proof. The complete
Header theorem and frozen performance gate remain outstanding.

The cached-hit/EOF milestone passed full regeneration/check with 253 audited
theorems and no added transitive axioms. The complete `utils/cbor/...` suite,
including compatibility checks, and targeted typed Header tests passed.
`TestFieldsCachedComparisons` exercises matching names, later table entries,
case variants and a changed byte at each name position around all three word
boundaries. It varies following byte-payload contents and lengths, including
inputs immediately below/at/above the 24-byte window. Field tests passed with
`-race -gcflags=all=-d=checkptr`. `FuzzTypedHeaderMatchesPlanner` completed 214,939
executions with a ten-second fuzz budget, comparing typed Header decoding with
both FX and the original planner. The new guard, cached-hit proof and arbitrary-
offset EOF proof are in the audited source set. The subsequent short-buffer search
milestone is recorded below; the complete Header theorem and the frozen performance
gate remain outstanding.

The short-buffer search milestone passed full regeneration/check with 271 audited
theorems and no added transitive axioms. Targeted public cursor field, projection
and key tests passed, as did the typed Header tests (cached against unchanged Go
sources). This milestone changes proofs and documentation only. Existing runtime
coverage includes the cache-window boundary and post-key data cases added in the
previous milestone. The subsequent word-search milestone is recorded below;
broader key/error composition, the complete Header theorem and frozen performance
gate remain pending.

The word-search/public-miss milestone passed full regeneration/check with 280
audited theorems and no added transitive axioms. Targeted public cursor field,
projection and key tests and typed Header tests passed (cached against unchanged
Go sources). This milestone changes proofs and documentation only. The checked
word-search table restriction includes every Header/GasPrice entry; it is not a
claim about arbitrary longer field names. The subsequent public-hit/unified-ASCII
milestone is recorded below; wider/non-ASCII keys and errors, initialization, the
complete Header theorem and frozen performance gate remain outstanding.

The public-hit/unified-ASCII milestone passed full regeneration/check with 288
audited theorems and no added transitive axioms. Targeted public cursor field,
projection and key tests and typed Header tests passed (cached against unchanged
Go sources). This milestone changes proofs and documentation only. All public
cache paths now compose for inline/one-byte ASCII field names with the complete
Header/GasPrice table shape; this does not prove the Header value loop or the
remaining key encodings and errors. The subsequent public non-text/body-error
milestone is recorded below; UTF-8, nested outputs, initialization, the complete
Header theorem and frozen performance gate remain outstanding.

The public non-text/body-error milestone passed full regeneration/check with 300
audited theorems and no added transitive axioms. Targeted public cursor field,
projection and key tests plus integer/text-key fuzz seeds passed, as did typed
Header tests (cached against unchanged Go sources). The shared fallback refactor
also rechecked the earlier cache-hit, cache-miss and unified ASCII contracts.
This milestone changes proofs and documentation only. The subsequent head-error
milestone is described above; wider/non-ASCII text handling, UTF-8, nested outputs,
initialization, the complete Header theorem and frozen performance gate remain
outstanding.

The head-error milestone passed full regeneration/check (1932 build jobs) with
314 audited theorems, no added transitive axioms, and no admissions or native
evaluation axioms. The pinned Perennial checkout remains unchanged. Fresh public
cursor key/field/projection tests, integer/text-key fuzz seeds, head-classification
tests and typed Header tests passed with `-short -count=1`. This milestone adds
proofs and public regression tests; production decoder behavior is unchanged.
The next independent work is the remaining primitive/Skip rejection contracts
and nested output ownership, alongside the complete Header relation and loop.
Wider/non-ASCII text and UTF-8 still need proofs; the proposed generic slice-index
semantics repair remains awaiting approval. The complete Header theorem and the
frozen performance gate have not passed.


The felt-error milestone passed full regeneration/check (1934 build jobs) with
326 audited theorems and no added transitive axioms, admissions or native
evaluation axioms. Fresh cursor felt tests, all felt fuzz seeds and typed Header
tests passed with `-short -count=1`, including the new prefix-error and public
Header cleanup tests. Production decoder behavior and pinned trusted semantics
are unchanged. The subsequent inline-four milestone strengthens those limb-error
contracts as described above. Remaining head-error composition, recursive Skip
errors, nested output ownership, UTF-8, initialization, the general Header theorem
and the frozen performance gate remain unfinished.


The inline-four rejection milestone passed full regeneration/check (1938 build
jobs) with 332 audited theorems and no added transitive axioms, admissions or
native evaluation axioms. The four earlier restricted limb-error contracts were
replaced with contracts covering every array-header width; the superseded typed
composition helper was removed. Fresh public felt tests, all felt fuzz seeds and
typed Header tests passed with `-short -count=1`, including the new long-buffer
wide-shape rejection cases. Decoder behavior and pinned trusted semantics are
unchanged. Remaining felt work is array-header and malformed limb-head
composition, alongside recursive Skip errors and the complete rejection/converse
contract. The complete Header theorem and frozen performance gate remain pending.


The malformed-head propagation milestone passed full regeneration/check (1941
build jobs) with 348 audited theorems, no added transitive axioms, no admissions
and no native evaluation axioms. Fresh public malformed-head, scalar-head and
felt regression tests and typed Header tests passed with `-short -count=1`.
The pinned trusted checkout and production decoder are unchanged. Remaining
felt work includes outer wrong-type/tag, incomplete-argument and EOF contracts,
incomplete ignored heads, recursive Skip errors and a complete rejection/converse
contract. Nested output ownership, UTF-8, initialization, the general Header
theorem and the frozen performance gate are still unfinished.

The outer-head and incomplete-Skip milestone passed full regeneration/check
(1943 build jobs) with 360 audited theorems, no added transitive axioms, admissions
or native evaluation axioms. Fresh public incomplete-head, outer-felt, felt fuzz
seed and typed Header tests passed with `-short -count=1`. Production decoder
behavior and pinned trusted semantics are unchanged. The remaining proof work
includes recursive Skip failures and complete felt rejection/converse, wider and
non-ASCII successful keys, UTF-8, nested output ownership, initialization and
composition into the general Header theorem. The frozen performance gate has
not run.

The recursive-tag rejection milestone passed full regeneration/check (1948
build jobs) with 375 audited theorems, no added transitive axioms, admissions or
native evaluation axioms. Fresh public Skip, later-felt-error and typed Header
tests passed with `-short -count=1`, including tagged error precedence, rollback
and Header cleanup. The production decoder and pinned trusted semantics are
unchanged. Next are array/map count/depth rejection and recursive child-error
propagation, followed by rejection/converse composition and the other Header
obligations above. The general Header theorem and frozen performance gate remain
unfinished.

The container rejection milestone passed full regeneration/check (1953 build
jobs) with 389 audited theorems, no added transitive axioms, admissions or native
evaluation axioms. Fresh public Skip, later-felt-error and typed Header tests
passed with `-short -count=1`, including the new count/depth boundaries and
malformed children after nested/wide prefixes. Production decoder behavior and
pinned trusted semantics remain unchanged. Remaining rejection work includes
failing nested containers and other leaf classes, plus complete classification
and converse composition. The general Header proof, UTF-8, nested output
ownership, initialization and the frozen performance gate remain unfinished.

The recursive malformed-head milestone passed full regeneration/check (1956
build jobs) with 395 audited theorems, no added transitive axioms, admissions or
native evaluation axioms. Fresh public Skip, later-felt-error and typed Header
tests passed with `-short -count=1`, including all malformed head classes through
nested arrays/maps/tags and Header cleanup at deep inner failures. The production
decoder and pinned trusted semantics are unchanged. Remaining work includes
general recursive propagation of the other leaf errors, complete classification
and converse, and the outstanding Header ownership, UTF-8, initialization and
composition obligations. The frozen performance gate has not run.

The shared failure-context milestone passed full regeneration/check (1959 build
jobs) with 405 audited theorems, no added transitive axioms, admissions or native
evaluation axioms. Fresh public Skip, later-felt-error and typed Header tests
passed with `-short -count=1`; the new recursive truncation/atomic regression
suite also passed after its final test-source cleanup. The production decoder
and pinned trusted semantics are unchanged. Remaining work includes shared-context
count/depth and missing-tag-content instances, complete Skip classification and
converse, and the other Header ownership/UTF-8/initialization/composition
obligations. The frozen performance gate has not run.

The unified Skip rejection milestone passed full regeneration/check (1963 build
jobs) with 416 audited theorems, no added transitive axioms, admissions or native
evaluation axioms. Fresh public Skip, later-felt-error and typed Header tests
passed with `-short -count=1`, including the new nested boundary-error regression
cases. Production decoder behavior and pinned trusted semantics are unchanged.
Next is arbitrary-input acceptance/rejection classification and converse, then
the remaining felt and Header ownership/UTF-8/initialization/composition work.
The general Header theorem is still absent and the frozen performance gate has
not run.

The arbitrary-input Skip milestone passed full regeneration/check (1967 build
jobs) with 428 audited theorems, no added transitive axioms, admissions or native
evaluation axioms. Every finite input at a supported depth is classified, and
`wp_Skip_complete` proves the corresponding outcome of actual public Skip with
unchanged input, exact success suffix or full cursor rollback and owned inner
diagnostics. This change adds only proofs and documentation; production Go and
the pinned trusted semantics are unchanged. Complete felt classification,
UTF-8, wider/non-ASCII keys, nested output ownership, initialization and the
general Header composition remain unfinished. The frozen performance gate has
not run.

The arbitrary-input typed felt milestone passed full regeneration/check (1971
build jobs) with 439 audited theorems, no added transitive axioms, admissions or
native evaluation axioms. `wp_readFelt_complete` proves the specified value/error
outcome for arbitrary input with owned outputs, exact suffixes, unchanged input,
and old-pointer/cursor preservation on failure. Only proof/documentation/audit
files changed; production Go and pinned trusted semantics are unchanged. Next is
the in-place Limbs contract and nested signature/GasPrice ownership. UTF-8, wider/
non-ASCII keys, initialization and the complete Header theorem remain unfinished;
the frozen performance gate has not run.

The arbitrary-input in-place Limbs milestone passed full regeneration/check
(1976 build jobs) with 456 audited theorems, no added transitive axioms, admissions
or native evaluation axioms. Both arbitrary-input outcome classification and
single-slot backing-array ownership compose through actual Limbs calls. Only
proof/documentation/audit files changed; production Go and pinned trusted semantics
are unchanged. The actual signature allocation/loops and GasPrice shared ownership,
UTF-8, wider/non-ASCII keys, initialization and general Header theorem remain
unfinished. The frozen performance gate has not run.

The general felt-pointer slice success milestone passed full regeneration/check
(1979 build jobs) with 467 audited theorems, no added transitive axioms, admissions
or native evaluation axioms. The actual nullable reader, both allocations, range
loop, Limbs calls, pointer publication and final depth restoration are checked
for arbitrary valid nullable elements and lengths. The returned ownership
preserves nil versus allocated empty and the pointer/backing-value correspondence.
Only proof/documentation/audit files changed; production Go and pinned trusted
semantics remain unchanged. Pointer-slice errors, outer signature iteration,
GasPrice shared ownership, UTF-8, wider/non-ASCII keys, initialization and the
general Header theorem remain unfinished. The frozen performance gate has not run.

The arbitrary-input felt-pointer slice milestone passed full regeneration/check
(1986 build jobs) with 486 audited theorems, no added transitive axioms, admissions
or native evaluation axioms, matching source hashes, and unchanged pinned trusted
semantics. This closes actual pointer-slice failure propagation before allocation
and after arbitrary valid nullable prefixes, retaining the specified cursor/depth
and exact owned diagnostic while preserving the old destination and whole input.
Exhaustive input classification and the actual success/failure proofs compose into
`wp_readFeltPointers_complete`, with no chosen-outcome or helper-WP premise.
Package initialization and immutable ownership of five error globals remain
explicit preconditions. This is partial correctness/model safety on the pinned
64-bit target; it does not prove termination, OOM handling or runtime correctness.
Production Go is unchanged. Next is the actual outer `cborReadSignatures` loop,
followed by shared GasPrice ownership and composition with the remaining Header
contracts. UTF-8, wider/non-ASCII keys, initialization and the complete Header
proof remain unfinished; the frozen performance gate has not run.

The arbitrary-input nested-signature milestone passed full regeneration/check
(1994 build jobs) with 508 audited theorems, no added transitive axioms, admissions
or native evaluation axioms, matching source hashes, and unchanged pinned trusted
semantics. `wp_readSignatures_complete` now composes exhaustive input
classification with actual outer allocation/range-loop and inner reader proofs.
Success owns the outer slice headers and every nested pointer table/backing array,
preserving nil versus allocated empty at both slice levels and the exact suffix.
Failure preserves the old destination and whole input, keeps partial outputs
unpublished, and retains the exact specified cursor/depth and owned diagnostic.
Pure bounds establish in-buffer cursor/error offsets and retained depth at most
32. Package initialization and immutable ownership of five error globals remain
explicit preconditions. Production Go is unchanged. Next is shared GasPrice
ownership, then composition with the outstanding UTF-8, wider/non-ASCII key,
initialization and complete Header contracts. The frozen performance gate has
not run. This remains partial correctness/model safety, excluding termination,
OOM and compiler/runtime implementation correctness.

The GasPrice field/shared-storage milestone passed full regeneration/check
(1998 build jobs) with 521 audited theorems, no added transitive axioms, admissions
or native evaluation axioms, matching source hashes, and unchanged pinned trusted
semantics. Complete arbitrary-input contracts now cover the actual felt call in
either GasPrice amount field, preserving the other field and retaining exact
owned diagnostics on failure. Shared ownership tracks the two struct slots and
nested felt allocations, supports borrowing/updating either slot, and the actual
zeroed allocation and null/undefined slot branch are proved. Four actual wrapper
adapters cover fresh/reused allocation, pointer publication on success and old
destination preservation with a possibly partial slot on failure. **Those four
adapters still have an explicit child-reader WP premise.** Their output structs
and diagnostic locations are existential, allowing fresh child allocations;
they do not assume predetermined output pointers. The next task is the general
`cborReadGasPrice` map loop and discharge of those child premises. Production Go
is unchanged. UTF-8, wider/non-ASCII keys, initialization, the general Header
proof and the frozen performance gate remain unfinished. The formal scope remains
partial correctness/model safety, excluding termination, OOM and compiler/runtime
implementation correctness.

The GasPrice map/slot success milestone passed full regeneration/check
(2003 build jobs) with 535 audited theorems, no added transitive axioms, admissions
or native evaluation axioms, matching source hashes, and unchanged pinned trusted
semantics. `FieldChoice.lean` identifies the unique known field or excludes every
known field; `GasPriceKeys.lean` composes actual key reading and search progress.
`GasPriceMapSpec.lean` specifies entry values and the final fold independently
of the generated loop. `GasPriceMap.lean` proves the actual map loop for arbitrary
map-head widths and initial owned amounts, either known-field order, nullable
felt values and arbitrary nested skipped values, with exact suffix, unchanged
input and restoration of the caller's depth. Known fields must be fresh; unknown
keys may repeat. The key relation currently covers inline/one-byte-length ASCII
text keys and signed-range integer keys. Wider-length text keys, non-ASCII keys,
duplicates and other errors are not covered by this success theorem.
`GasPriceMapSlot.lean` composes the actual fresh/reused shared-slot wrappers with
that map proof, preserving the other slot and both sets of nested felt ownership;
these two success contracts have no child-reader WP premise. The earlier failure
adapters remain conditional. The next obligations are general key coverage and
GasPrice failure-after-prefix composition/classification, then the remaining
Header dependencies and general Header loop. Production Go is unchanged and the
frozen performance gate has not run.

The GasPrice value-failure milestone passed full regeneration/check (2005 build
jobs) with 539 audited theorems, no added transitive axioms, admissions or native
evaluation axioms, matching source hashes, and unchanged pinned trusted semantics.
`GasPriceMapFailure.lean` specifies failed known/unknown values through the complete
felt/Skip failure relations and proves the actual generated loop after arbitrary
successful entry prefixes. The invariant includes both ongoing and failed states,
so the actual next loop-condition check and error return are proved. Earlier
fields and their child allocations remain owned; a failed value preserves its
prior field. The cursor retains the entered map depth and the position after the
failed entry's key, while the owned diagnostic may point further into the value.
Both offsets are bounded within the input without signed wraparound.
`GasPriceMapFailureSlot.lean` closes the actual fresh/reused wrapper contracts for
these failures: the caller's destination pointer remains unchanged, both shared
struct slots and their nested children remain owned, and the selected slot keeps
its successful prefix updates. These theorems have no child-reader WP premise.
They require the current supported key relation, fresh known fields, an admitted
map count/depth and a specified value failure. Outer-map rejection, malformed or
duplicate keys, wider/non-ASCII key families and exhaustive GasPrice classification
remain pending. The complete Header theorem and performance gate remain unfinished;
production Go is unchanged.

The GasPrice opening/duplicate/unified-failure milestone passed full
regeneration/check (2013 build jobs) with 556 audited theorems, no added transitive
axioms, admissions or native evaluation axioms, matching source hashes, and
unchanged pinned trusted semantics. `MapExpectedFailure.lean` closes actual
MapLen/OpenStruct expectation errors and general null/undefined opening.
`GasPriceOpen.lean` composes expectation/count/depth rejection and nullish success
through the actual generated reader. `GasPriceOpenSpec.lean` exhaustively
classifies opening at supported depth into nullish success, an admitted map head,
or its exact failure, respecting nullish-before-type and count-before-depth
precedence. This classifies opening, not the entire map body.
`GasPriceDuplicate.lean` and `GasPriceMapDuplicate.lean` prove actual duplicate
rejection for the currently supported key families after arbitrary valid entry
prefixes: only the duplicate key is consumed, its value is untouched, the map
remains entered, and previous fields/children and exact diagnostic arguments are
owned. `GasPriceFailureSpec.lean` unifies opening, value and duplicate failures,
including the partially decoded abstract value, exact consumed/diagnostic offsets
and retained depth; it proves offset/depth bounds and exclusion of nullish inputs.
`GasPriceFailureSound.lean` discharges all production paths for this relation,
with initialized error globals and fractional immutable metadata as preconditions.
`GasPriceFailureSlot.lean` closes actual fresh/reused slot wrappers for that
relation without child-reader assumptions, preserving the old destination pointer,
other slot and all nested ownership. General key rejection, wider/non-ASCII key
families and exhaustive GasPrice outcome classification remain pending. UTF-8,
initialization, the complete Header theorem and the performance gate remain
unfinished. Production Go is unchanged.

The GasPrice key-rejection milestone passed full regeneration/check (2016 build
jobs) with 559 audited theorems, no added transitive axioms, admissions or native
evaluation axioms, matching source hashes, and unchanged pinned trusted semantics.
`KeyFailureSpec.lean` independently specifies malformed/incomplete heads, EOF,
integer range errors, unsupported tags, wrong key types and truncated text bodies,
with exact diagnostic contents. `KeyFailureSound.lean` composes every case through
actual public NextField for the existing table contract, preserving the entire
input and restoring the key-start cursor, next/seen/open state, while decrementing
the outstanding pair count once. `GasPriceMapKeyFailure.lean` composes this through
the actual generated loop after arbitrary successful supported-key prefixes,
preserving their abstract values and nested ownership, the exact unread suffix,
retained map depth and bounded diagnostic position. The unified GasPriceFailure
relation and actual reader theorem now include these key errors; both existing
and fresh slot-wrapper theorems apply to them without new helper assumptions.
Invalid UTF-8 remains a missing key-error case; wider/non-ASCII successful keys
and exhaustive GasPrice classification still await the relevant contracts.
Initialization, the complete Header theorem and the performance gate remain
unfinished. Production Go and the pinned trusted semantics are unchanged.

The Header ownership/felt-field milestone passed full regeneration/check (2019
build jobs) with 573 audited theorems, no added transitive axioms, admissions or
native evaluation axioms, matching source hashes, and unchanged pinned trusted
semantics. `HeaderOwnership.lean` defines all 15 decoded application values,
including the exact nullable/list shape of signatures, nullable GasPrice structs,
scalar words and string bytes. HeaderBody owns the non-price children;
HeaderGasState retains the private shared allocation and independent publication
flags; HeaderChildren/HeaderOwned describe caller-visible outputs. A checked
splitting proof turns the owned two-element GasPrice array into separately owned
struct/child results without duplicating storage. Publication handles either
missing/null slot, and zero-value ownership requires no child allocation.
The actual cleanup call resets any owned partially populated Header and returns
owned zero-value contents; this uses the existing affine/GC model and does not
claim physical memory reclamation.
`HeaderFeltFields.lean` borrows/restores each of the six actual felt-pointer fields,
preserves the other fourteen fields and proves value/child-ownership updates.
Actual felt success/failure calls retain unchanged input, exact suffix or bounded
owned diagnostics, and the private GasPrice state is invariant under those writes.
`HeaderFeltComplete.lean` composes exhaustive felt classification into an
arbitrary-input outcome contract for those six calls, carrying both HeaderBody
and HeaderGasState, with no assumed child correctness. This is a field-call
contract; it does not prove Header key dispatch or its whole loop. The full Header
wire relation, other field-kind composition, UTF-8-dependent paths, initialization,
the general Header APIs and the performance gate remain unfinished. Production
Go is unchanged.

The Header signature/scalar composition milestone passed full regeneration/check
(2022 build jobs), with 587 audited theorems, matching source hashes, no added
transitive axioms or admissions, and unchanged pinned semantics. It adds three modules.
`HeaderSignatures.lean` composes the existing complete nested reader at the actual
Header field, preserving all other fields and private GasPrice ownership. It
retains the previous signature value on failure, including failures after partial
nested construction, and carries the exact consumed/error positions and depth.
`ScalarComplete.lean` independently specifies and exhaustively classifies both
typed scalar APIs: null/undefined preserve the previous value, every unsigned
width stores its decoded word, and every other input has a specified expectation
failure with unchanged destination and cursor. On the pinned 64-bit target this
also covers all DA-mode values, including unnamed enum values.
`HeaderScalars.lean` composes those actual calls at Number, TransactionCount,
EventCount, Timestamp and L1DAMode, relating the old machine word to the independent
Header application value and preserving all other fields and nested ownership.
These remain field-call contracts, not proof of Header key dispatch or the whole
map loop. The remaining field kinds are ProtocolVersion and the two GasPrice
slots; the existing GasPrice reader/slot contracts still need full classification
and composition with HeaderGasState. UTF-8/key completion, initialization, the
Header wire relation and general public API proofs remain pending. This milestone
changes no production Go or pinned trusted semantics; the frozen performance
gate remains unrun.

The Header GasPrice integration milestone passed full regeneration/check
(2025 build jobs), with 604 audited theorems, matching source hashes and no added
transitive axioms or admissions. It adds `HeaderGasFields.lean`,
`HeaderGasMap.lean` and `HeaderGasFailure.lean`. A private snapshot records both
slot values and independent publication flags. The actual calls at L1DataGasPrice
and L2GasPrice now restore HeaderBody and HeaderGasState for null/undefined,
successful maps in the proved key families, and every specified GasPrice failure,
using the existing closed child-reader/slot contracts. Both fresh allocation and
reuse are covered. Success publishes exactly the selected slot and carries the
exact suffix, bounded signed cursor advancement and unchanged input. Failure
preserves all Header pointers and carries the exact owned diagnostic and retained
cursor/depth; a previously published selected slot reflects partial updates in
its application value, while unpublished partial contents remain private. The
other slot and every non-price field are preserved. Cleanup is proved from the
private loop ownership state, including unpublished partial slots, under the
existing affine/GC model; physical reclamation is not claimed.

These are field-call contracts with explicit shared-layout premises. They do not
prove Header field dispatch, initialization, the whole map loop, or exhaustive
GasPrice input classification. GasPrice still needs wider/non-ASCII key and UTF-8
completion. The Header loop must connect its seen-field state to the private
slot's initial value, and compose these field contracts with the independent
Header wire relation. ProtocolVersion and the full public APIs remain pending.
Production Go and pinned trusted semantics are unchanged; the performance gate
remains unrun.

The Header entry-guard/string-error milestone passed full regeneration/check
(2028 build jobs), with 614 audited theorems, matching source hashes and no added
transitive axioms or admissions. It adds `HeaderZero.lean`,
`StringFailure.lean` and `HeaderString.lean`. The actual generated zero check is
proved for every Header value, with its result equivalent to equality with the
zero Header, including nil rather than empty signatures. Both DecodeHeaderFirst
and DecodeHeader reject every nonzero destination unchanged, with the exact owned
unsupported diagnostic. This no longer requires Hash to be the nonzero field;
neither API reads input or runs cleanup on this rejection path.

The general payload expectation-failure contract now propagates through String
and the typed string reader. StringFailure specifies malformed/incomplete heads,
EOF, wrong types/tags and oversized/truncated payloads, respecting the typed
wrapper's null/undefined success before expectation. The actual ProtocolVersion
field calls preserve all Header values and nested/shared ownership for those
errors and for null/undefined, with exact consumption on null and unchanged
cursor/input plus owned diagnostics on error. These string failures occur before
UTF-8 validation; the relation is not exhaustive and no successful-text or
invalid-UTF-8 theorem is claimed. The general Header loop, initialization,
UTF-8-dependent key/GasPrice completion and performance gate remain pending.
Production Go and pinned trusted semantics are unchanged.

The general record-opening milestone passed full regeneration/check (2033 build
jobs), with 623 audited theorems, matching source hashes and no added transitive
axioms or admissions. It extracts the independent shared relation
into `StructOpenSpec.lean`; GasPriceOpenFailure is now an alias of StructOpenFailure,
and the existing GasPrice classification reuses the shared theorem. This avoids
separate Header and GasPrice definitions of null precedence, count limits and
nesting checks. `StructOpenSound.lean` proves every failure and every admitted map
opening through actual OpenStruct, with exact head consumption, remaining body,
signed bounds and depth. `StructOpenComplete.lean` composes exhaustive arbitrary-
input classification into a closed production outcome contract: null/undefined
produce a closed state without depth entry; a definite map produces an open state
with count/next/seen initialized and depth incremented; rejection preserves the
original cursor/input and returns a closed state with an owned diagnostic. No
field metadata is read during opening and no child correctness is assumed.

`HeaderOpen.lean` composes the actual generated Header loop's opening-error and
null/undefined paths. `HeaderOpenContract.lean` preserves arbitrary existing Header
child ownership on null/undefined at arbitrary offsets/depths and proves every
specified opening failure through both DecodeHeaderFirst and DecodeHeader,
including actual cleanup, unchanged input, exact error ownership and an owned
zero Header. These are general opening paths, not a complete map-body theorem.
Header key dispatch, the complete wire relation/map loop, UTF-8-dependent paths,
initialization and the performance gate remain pending. Production Go and pinned
trusted semantics are unchanged.

The Header key/readiness milestone passed full regeneration/check (2037 build
jobs), with 641 audited theorems, matching source hashes and no added transitive
axioms or admissions. It adds `HeaderKeys.lean`,
`HeaderKeyDuplicate.lean`, `HeaderSeen.lean` and `HeaderGasReady.lean`. HeaderKeys
introduces all fifteen schema identities and proves their indices/names against
the actual static metadata, including the legacy gasprice/gaspricestrk wire tags.
The actual NextField contract now selects these identities for the currently
proved ASCII inline/one-byte-length text and signed-integer key families,
including case variants, unknown keys and fresh seen-bit updates. Duplicate
known keys return Done with the exact schema-name diagnostic, preserve seen bits,
consume only the key and leave its value unread. Wider and non-ASCII text remain
outside this closed key contract.

HeaderSeen proves distinct bit masks, the exact fresh-set change after marking a
key, and correspondence between seen bits and a list of schema identities. Its
HeaderGasUnseen invariant connects an unvisited price field to zero, unpublished
private storage. HeaderGasReady strengthens the private ownership predicate with
that invariant. The existing HeaderGasMap and HeaderGasFailure contracts now
retain HeaderGasReady through actual fresh/reused slot calls, using an explicit
prior seen mask and readiness premise for existing snapshots. Marking non-price
keys, null/undefined price calls and actual cleanup also preserve/discharge the
stronger state. Ready can be weakened to the earlier HeaderGasState for output
publication and cleanup, without duplicating ownership.

This supplies the missing connection between duplicate bookkeeping and a child
map's initial value; it is not a proof of the complete Header dispatch/map loop.
The loop still must compose these transitions with all field contracts, and
successful/invalid UTF-8 strings, wider/non-ASCII keys, full GasPrice classification,
initialization and the performance gate remain pending. Production Go and pinned
trusted semantics are unchanged.

### Uniform Header field-state milestone

The uniform field-state milestone passed full regeneration/check (2039 build
jobs), with 651 audited theorems, matching source hashes and no added transitive
axioms or admissions. `HeaderFeltComplete`, `HeaderScalars`, `HeaderSignatures`
and the currently proved `HeaderString` paths now retain `HeaderGasReady`, rather
than losing the unseen-slot invariant by returning only `HeaderGasState`.

`HeaderGasFresh` closes actual price-field success and every currently specified
failure from a common readiness precondition. The unseen key implies a zero,
unpublished selected slot; owned array contents supply its concrete element.
The wrapper handles both allocation states without requiring callers to choose
one or supply layout/zero-value premises. Successful maps publish their specified
value from zero. Failure preserves every public Header field and abstract value,
while retaining ownership of private partial contents and the exact diagnostic,
cursor and depth. It does not claim exhaustive GasPrice classification.

`HeaderFieldSuccess` composes actual felt/scalar/signature calls with their
independent successful input relations and the updated seen mask. It retains
other fields, nested ownership, exact suffixes, input bytes and bounded cursor
advancement. These contracts can be used directly by the map-loop proof without
assuming child correctness or first proving that an existentially classified
success equals the intended value.

These are field-call composition proofs. The complete Header wire relation,
actual dispatch/map loop, successful/invalid UTF-8 paths, wider/non-ASCII keys,
exhaustive GasPrice classification, initialization and public API composition
remain unfinished. Production Go, generated translations and pinned trusted
semantics are unchanged; the frozen performance gate has not run.

### Header map and public-API composition milestone

This milestone passed full regeneration/check (2042 build jobs), with 661 audited
theorems, matching source hashes and no added transitive axioms or admissions.
`HeaderMapSpec.lean` introduces explicit successful entry/map values and proves
the byte-count bounds needed by record opening. Scalars distinguish replacement
from nullish preservation; signatures and prices retain all nil/empty distinctions.

`HeaderMap.lean` proves the actual generated `cborReadHeader` loop from a zero
Header for arbitrary lists of these entries. The invariant tracks consumed and
remaining entries, the target application value, exact cursor position/suffix,
search-index bounds, duplicate freshness and the shared-price readiness state.
All fifteen concrete dispatch arms and the unknown-field Skip arm are checked.
Both GasPrice fields may occur in either order, with actual shared allocation
and independent publication; successful return discharges private storage into
owned output children. Input bytes and fractional field metadata are preserved.
No child-reader WP is an assumed premise.

`HeaderMapAPI.lean` composes that loop through actual `DecodeHeaderFirst` and
`DecodeHeader`. First returns the precise unread input slice and a Header owning
its nested outputs. Full decoding succeeds when no bytes remain. For any nonempty
suffix it executes actual cleanup, returns an owned zero Header and an owned
extraneous-data diagnostic with the exact suffix length, preserving the input.

**Scope remains explicit:** `HeaderEntry` currently allows inline/one-byte-length
ASCII and supported signed-integer keys, as in HeaderKeySupported; ProtocolVersion
is null/undefined only. Felt/scalar/signature/ignored values are general within
their independently proved relations; child GasPrice maps use the previously
proved key families. This is a general composition theorem for that family, not
the final all-encoding or arbitrary-input Header theorem. Wider/non-ASCII keys,
successful/invalid UTF-8 text, exhaustive GasPrice classification, arbitrary
Header map-body error propagation and initialization remain pending. The next
integration step is map-body rejection after arbitrary valid prefixes, alongside
completion of those missing input families. Production Go, generated translations
and pinned trusted semantics are unchanged; the performance gate has not run.

### Header lookup-error and cleanup milestone

This milestone passed full regeneration/check (2046 build jobs), with 667 audited
theorems, matching source hashes and no added transitive axioms or admissions.
`HeaderMapDuplicate.lean` proves actual duplicate rejection after any successful
HeaderEntry prefix. The duplicate is identified by schema identity, including
legacy wire names and already-proved case variants; the key is consumed, its
value remains unread, and earlier fields/nested allocations remain owned. The
owned diagnostic contains the exact field name and bounded post-key offset.

`HeaderMapKeyFailure.lean` proves the corresponding actual loop path for every
currently specified KeyFailure: malformed/incomplete heads, EOF, integer range,
type/tag and text-payload truncation. The cursor remains at the rejected key,
with earlier Header values/allocations owned and exact diagnostic information.
Both loop proofs preserve input and fractional metadata and compose all fifteen
successful prefix branches, including arbitrary signatures and shared prices.

`HeaderLookupFailure.lean` gives these paths one independent relation and closed
actual-reader contract. Its bounds connect the retained cursor, diagnostic and
nesting depth without machine wrap. `HeaderLookupFailureAPI.lean` carries either
failure through actual zero checking, reading and cleanup in both public APIs.
First returns a nil suffix; both leave an owned zero Header, unchanged input and
an owned diagnostic at the exact nonwrapping input offset. Thus parse-error
cleanup after populated Headers is no longer limited to the original fixed
Number/truncation example.

This is not exhaustive Header rejection. Successful prefixes retain HeaderEntry's
explicit key and nullish-protocol-string restrictions. Invalid UTF-8 key rejection,
wider/non-ASCII successful keys, general field-value failures (including failures
inside signatures/prices), exhaustive GasPrice classification and initialization
remain unfinished. The next composition step is general field-value rejection
after arbitrary valid prefixes. Production Go, generated translations and pinned
trusted semantics are unchanged; the frozen performance gate has not run.

### Header field-value failure and cleanup milestone

This milestone passed full regeneration/check (2050 build jobs), with 675 audited
theorems, matching source hashes and no added transitive axioms or admissions.
`HeaderFieldFailure.lean` supplies direct actual scalar/signature failure contracts
at their Header fields, retaining all prior field/child ownership. This allows a
specified failing input to compose without assuming a child-reader WP or proving
uniqueness of an existentially classified outcome first.

`HeaderValueFailureSpec.lean` independently describes failures at every selected
field kind and unknown-value Skip. It distinguishes retained consumed bytes from
diagnostic offsets and retained nesting depth, and proves their input/depth bounds.
The whole-map witness records opening/count precedence, fresh keys and arbitrary
successful HeaderEntry prefixes. String failures still cover only pre-validation
errors, and GasPrice retains its previously documented failure/key coverage.

`HeaderMapValueFailure.lean` proves the actual generated map loop for every case
of that relation: all six felt fields, all five scalar fields, nested signatures,
ProtocolVersion, both price fields and ignored values. The loop invariant covers
both ongoing parsing and a stored error awaiting the loop condition. Earlier
Header values remain owned. Failed fresh/reused GasPrice slots may retain private
partial contents; these remain unpublished and can be discarded when private
storage is converted to the unchanged public prefix value. Cursor advancement,
diagnostic position and retained child depth are preserved without word wrap.

`HeaderValueFailureAPI.lean` composes the same failures through actual zero
checking, reading and cleanup in both public APIs. First returns a nil suffix;
both APIs leave an owned zero Header, unchanged input and the exact owned
field/child diagnostic at its nonwrapping input offset. Cleanup is a reset and
ownership result in the GC/affine model, not physical allocation reclamation.

The actual success and currently specified opening/lookup/value/trailing-error
paths now have public-API composition proofs. This still is not a complete Header
proof: HeaderEntry retains its key/nullish-protocol-string restrictions, UTF-8
validation and wider/non-ASCII key contracts are missing, GasPrice and Header
need exhaustive input classification, and metadata initialization/shared ownership
must be established. The next work is closing those input/dependency gaps and
assembling the final general outcome contract. Production Go, generated
translations and pinned trusted semantics are unchanged; the performance gate
has not run.

### Immutable metadata ownership milestone

Full regeneration/check passed (2052 build jobs), with 686 audited theorems,
233 matching source hashes and no added transitive axioms or admissions. The
local check log is `check-header-metadata-full.log`.

`Fields.lean` now proves the actual Header and GasPrice factories with only the
cursor dependency initialized. Neither factory requires core initialization to
have completed, so their contracts can be used during core startup.

`FieldsShared.lean` proves that the allocated table structs and both backing
slices can be frozen into persistent read-only ownership, retaining the generated
keys, names and exact masks. Its factory contracts establish that ownership from
actual allocation. `HeaderMetadata` packages both shared tables and their global
pointers. A publication lemma freezes the globals **after their stores have been
established**; it does not assume or prove that those stores executed.

`HeaderSharedAPI.lean` composes this persistent metadata through First/full map
success, the currently specified lookup/value failures, and trailing-data cleanup.
Independent decoder invocations can reuse the metadata without dividing ownership
fractions. Each call still requires its destination and input ownership, and
returns the same owned output/error, unchanged input and applicable suffix/offset
facts as the underlying API contracts. No child-reader correctness assumption is
introduced, and the existing key/nullish-string coverage restrictions remain.

Inspection of the generated core initializer confirms that each global store is
inside the lexical binding of its corresponding factory call. The raw nested
printer layout separates those calls and stores; it is not a missing initializer
or an unbound temporary. Proving actual package startup and establishing the error
globals remain open, alongside UTF-8, wider/non-ASCII keys, exhaustive GasPrice/
Header classification and the unified Header theorem. Production Go and pinned
trusted semantics are unchanged; the frozen performance gate has not run.

### Cursor and UTF-8 initialization milestone

Full regeneration/check passed (2059 build jobs), with 693 audited theorems,
238 matching source hashes and no admissions or native-evaluation axioms. The
17 newly referenced opaque code constants are explained below. The local check
log is `check-cursor-utf8-initialization-full.log`.

`InitializationState.lean` now contains the package initialization instances
previously defined in `Header.lean`. The cursor predicate records the five actual
immutable `errors.New` objects with their messages. The UTF-8 predicate records
both complete lookup arrays: 256 first-byte classifications and 16 second-byte
ranges. These are initialization guarantees to establish, not newly assumed
helper correctness. Existing reader proofs continue to require package
initialization, now with these concrete contents.

`Utf8Initialize.lean` proves the actual translated Go initializer allocates and
stores both exact arrays and retains them persistently, including the already
initialized case. This does **not** prove UTF-8 validation; the generic slice-index
semantics issue and validator loop proofs remain open.

`BinaryInitialize.lean` composes the pinned Perennial initialization proofs for
`encoding/binary`, `io`, `sync` and their dependencies without caller-supplied
initializer premises. `CursorInitialize.lean` proves the actual cursor startup,
including global allocation, dependency calls, five proved `errors.New` calls,
all stores, immutable publication and repeat-call behavior. Its only remaining
initializer premise is `DecoderDependencyInit` for the separately translated
`math/big.initialize'`. This is an explicit unresolved proof obligation, not an
end-to-end initialization result.

That premise and the actual cursor initializer reference **17 existing opaque
math/big initializer code constants** (including `hasADX.init`, `natOne.init`,
`intOne.init` and the threshold/table initializers). They newly enter the audited
theorem dependency set. No new axiom declarations, trusted toolchain edits or
admissions were introduced. These code constants are not established initializer
correctness facts: the premise still requires a proof of their whole initializer's
contract. The current math/big translation config leaves their bodies opaque, so
closing that dependency requires addressing that translation/initialization scope.
Source inspection identifies literal tables/slices, numeric thresholds, error
construction and interface-conformance variables, plus `hasADX` reading
`internal/cpu.X86.HasADX` and `HasBMI2`. The next investigation is their real
translation and CPU-initialization boundary, not a replacement correctness axiom.

`InitializationErrors.lean` derives the existing `FeltErrorGlobals` resources from
cursor initialization and, transitively, core initialization. Final Header APIs
can therefore obtain their error-category values from established package state
instead of separately asking callers to supply them.

Core metadata stores, fastcbor startup and the remaining relevant dependency chain
are still unproved. Successful/invalid UTF-8 strings, broader keys, exhaustive
GasPrice/Header classification, the complete Header theorem and the frozen
performance gate remain outstanding. Production Go is unchanged.

### Closing the math/big initializer contract

Full regeneration/check passed (2070 build jobs), with 697 audited theorems,
241 matching project hashes and 28 matching standard-library Go-source hashes.
No admissions or native-evaluation axioms occur. The local check log is
`check-big-initialization-full.log`; the changed opaque dependency set is explained
below and recorded in the audit.

The earlier math/big initializer premise is now discharged. The harness translates
all actual math/big package-variable initializers and their required Go types,
including literal arrays/slices, thresholds, two error objects, interface
conformance expressions and the CPU-feature read. It also translates the CPU
feature-record layout and relevant package globals. Scratch translation and
Lean compilation established support before this configuration was adopted; no
pinned Goose/Perennial source or trusted model was changed.

`RuntimeInitialize.lean` proves the real package-state check for an already
initialized package: its initializer body is not executed, and its recorded
resources are recovered. It applies this to the actual translated CPU initializer.
A separate checked lemma establishes that the two generated anonymous-struct
representations use the same ADX/BMI2 field addresses, using equality of their Go
types and the existing deterministic field-reference semantics.

`BigInitialize.lean` proves the actual translated math/big initializer after CPU
startup. Both feature-flag branches, allocations, interface conversions, error
construction and stores are checked. Its fmt/io/sync/error initialization
dependencies use existing checked Perennial proofs. `CursorInitialize.lean` now
composes that theorem directly; **no initializer WP is a premise**.

The explicit precondition is `ownInitializing get_is_pkg_init` together with
`PackageInitialized internal/cpu`. The package-state mapping retains immutable
ownership of the real `HasADX` and `HasBMI2` fields, with arbitrary Boolean values.
Go's `internal/cpu.Initialize` source states that the runtime populates these flags
before normal init functions; `runtime.cpuinit` calls it. Hardware probing and the
runtime's creation of this initial state are outside the decoder proof, consistent
with the existing runtime boundary. The proof neither assumes false flags nor
reallocates/resets them. It is not a proof of whole-process startup from empty
state, or of unrelated math/big operations.

All 17 formerly referenced opaque math/big initializer constants leave the audited
theorem dependency closure. The retained fmt dependency brings in six existing
pinned type/underlying/zero-value placeholders for `fmt.pp` and `fmt.readRune`.
These are reported standard-library representation assumptions, not new
initializer-correctness axioms. The audit now also hashes the build-selected Go
sources of directly translated `unicode/utf8`, `math/big` and `internal/cpu`, and
records the Go build target.

Core metadata/global stores, fastcbor startup and other relevant core dependencies
remain open. The UTF-8 validator, wider/non-ASCII keys, exhaustive GasPrice/Header
classification, the complete Header theorem and the performance gate remain
unfinished. Production Go is unchanged.

### Decoder-owned schema initialization

Full regeneration/check passed (2074 build jobs), with 700 audited theorems,
245 matching project hashes and 28 matching standard-library source hashes.
The axiom dependency set is unchanged; there are no admissions or native-evaluation
axioms. The local check log is `check-schema-initialization-full.log`.

The generator now emits private Header/GasPrice tables into
`core/internal/cborschema/fields_generated.go`. The existing core types and typed
reader bodies remain in core; readers call `cborschema.OpenHeader` or
`cborschema.OpenGasPrice`. These functions do not return a mutable Fields pointer.
Both factory bodies and all literals are unchanged by the move, and Go compiler
inlining diagnostics confirm that both opening functions inline into the readers.
`cborgen -check` checks both generated outputs.

`SchemaState.lean` contains the independent exact table contents and persistent
ownership predicate. `SchemaInitialize.lean` proves the complete actual schema
initializer: both global allocations, cursor dependency startup, both factories,
both global stores, immutable publication and already-initialized reuse. Its only
runtime precondition is the existing CPU-startup boundary; it does not assume a
helper's correctness. The initialized package predicate retains `HeaderMetadata`.
The existing shared Header API contracts now recover this resource through core's
declared schema dependency, removing the separate metadata precondition. Those
contracts retain their explicit input-family restrictions and the core initialized
precondition; this milestone does not prove unrelated core/planner startup or
fastcbor error-global initialization.

Go regression, race and checkptr=2 checks pass for `./core ./utils/cbor/...`.
Two 20-second fuzz runs also pass: 250,792 structure-aware round trips and
268,127 differential executions against the planner.
`BenchmarkTypedHeader` measures direct generated decoding with zero, scalar,
price and populated/nested Header fixtures. Five alternating before/after pairs
on CPU 2, GOMAXPROCS=1, 300ms per case under the shared benchmark lock showed
median paired time changes of +0.71%, -1.55%, -0.39% and +0.41%, respectively.
Bytes and allocations were unchanged in every sample, including the zero-allocation
case. Raw samples, source/binary hashes and commands are in
`results/schema-initialization-benchmark.json` and the adjacent before/after files.
The Lean rebuild ran concurrently: this is an exploratory check against the
immediately preceding typed implementation, **not** statistical equivalence or the
frozen 185-case gate against the original planner baseline. That gate remains unrun.

UTF-8 validation, broader keys, exhaustive GasPrice/Header outcome classification,
remaining relevant initialization and the complete Header theorem are still open.
No pinned trusted semantics were changed.

### Shared entry errors and minimal decoder initialization

Full regeneration/check passed (2076 build jobs): 713 audited theorems,
249 matching project hashes and 28 matching standard-library source hashes.
There are no admissions or native-evaluation axioms and no new opaque axioms.
The 54 planner-specific type/underlying/zero-value placeholders previously pulled
in through fastcbor leave the audited dependency closure. The full log is
`check-entry-errors-initialization-full.log`.

`utils/cbor/cborerr` now owns the two canonical entry-error objects and
`DecodeError`. The generated readers use it directly. The existing fastcbor
sentinel names retain the same canonical values, and `fastcbor.DecodeError` is a
Go type alias, so `errors.Is`/`errors.As` and diagnostic messages remain compatible.
As in the persistent formal resources, exported sentinel variables must be treated
as read-only; reassignment of those variables is outside the contract. The named
type's reflected package path is now cborerr. The registered Unmarshal API continues
to format its own trailing-data error; the direct typed API returns DecodeError.
A public-API regression test checks these existing distinctions and identities.

`EntryErrorsInitialize.lean` proves the actual complete shared-error initializer,
including errors/fmt startup, allocations, both errors.New calls, both global stores
and repeat-call behavior. It also proves that the actual Unwrap method returns the
stored category without mutation. Both direct imports are retained in translation;
no dependency initializer is assumed correct.

Every core-reader contract now requires schema initialization instead of core
initialization. The proved schema initializer supplies the cursor, primitive error
objects and exact shared tables; the proved cborerr initializer supplies public-API
errors where read. The decoder does not access unrelated core globals or planner
state, and no IsPkgInit core/fastcbor premise remains. Their startup, routing and
registration stay outside the proof. This is proof of the decoder from its relevant
initialized state, not a proof of whole-process startup or the compiler/runtime.
The existing CPU-startup precondition and pinned standard-library representation
assumptions remain explicit.

`HeaderInitializedAPI.lean` recovers metadata and actual error categories from these
initialization predicates for the currently proved First/full successes,
nonzero-destination rejection, trailing-data cleanup, lookup failures and value
failures. Callers supply input/destination ownership, with no separately assumed
metadata or error-global memory. Error posts retain the link to the canonical global
values. The existing wire-relation restrictions remain: UTF-8, broader keys and
exhaustive GasPrice/Header classification are still unfinished, so this is not the
complete Header theorem.

Go regression, race and checkptr=2 checks pass for `./core ./utils/cbor/...`.
Five alternating before/after benchmark pairs on CPU 2, GOMAXPROCS=1, 300ms per
case under the shared lock showed median paired changes of +2.30%, +0.80%, -0.95%
and -0.87% for zero/scalar/price/full headers, with unchanged bytes and allocations
in every sample. `results/entry-errors-benchmark.json` records raw-file locations,
commands, hashes and scope; adjacent text files contain the samples. A Lean rebuild
ran concurrently, and this only compares the immediately preceding typed revision.
It is exploratory evidence, not the frozen 185-case gate or statistical equivalence.

No pinned trusted semantics were changed. The proposed UTF-8 semantics repair
remains unapplied; the complete Header theorem and full performance gate remain open.

### UTF-8 specification and illegal-leading-byte rejection

Full regeneration/check passed (2080 build jobs): **724 audited theorems**,
253 matching project hashes and 28 standard-library source hashes. No theorem
uses admissions or native-evaluation axioms; the complete axiom set is unchanged
from the previous checkpoint. The full log is
`check-utf8-leading-rejection-full.log`.

`Utf8Spec.lean` defines the independent
[RFC 3629 section 4 byte grammar](https://www.rfc-editor.org/rfc/rfc3629.html#section-4),
including the E0/ED/F0/F4 restrictions that exclude overlong encodings, surrogates
and values above U+10FFFF. It proves concatenation and arbitrary ASCII-prefix
preservation, and that continuation bytes, C0/C1 and F5–FF cannot begin valid text.
The specification imposes no Unicode normalization or BOM policy.

`Utf8Tables.lean` checks the actual initialized classification/accept tables for
all illegal leading bytes using kernel reduction. `Utf8Reject.lean` proves the
actual Go validator returns false for any such byte followed by arbitrary bytes,
while preserving fractional ownership of the input. A range-borrowing contract
restores the entire containing input after this read.

`StringUtf8Reject.lean` composes this through the actual `cursor.String` and
`cborReadString`, for any supported definite text-length encoding, arbitrary
starting position, complete payload and following suffix. Rejection restores the
original cursor, leaves input and the old string untouched, and allocates the
exact syntax diagnostic at the original offset. `StringFailure` now includes this
case, so the existing Header field/map/value-failure and initialized public-API
proofs also cover it after their supported successful field prefixes. Both public
entries perform the already-proved cleanup of partially populated Headers.

This is not a complete validator or Header proof. It covers illegal **first**
payload bytes; it does not yet cover illegal bytes after valid characters,
malformed continuation sequences, valid non-null strings or UTF-8 key rejection.
The successful-prefix/key restrictions in HeaderEntry remain. The generic
word-helper semantics repair remains unapplied, and no trusted source changed.
This checkpoint changes proofs and documentation only; the preceding Go test,
race/checkptr and exploratory benchmark evidence is unchanged. The frozen
185-case performance gate has not run.

### Illegal UTF-8 leading bytes in Header and GasPrice keys

Full regeneration/check passed (2082 build jobs): **733 audited theorems**,
255 matching project hashes and 28 matching standard-library source hashes.
The complete axiom set is unchanged, with no admissions or native-evaluation
axioms. The full log is `check-utf8-key-rejection-full.log`.

`KeyUtf8Reject.lean` proves the actual `keySlow`, short-key ASCII shortcut,
wider-head path, public `Key` and alternate lookup for complete text keys whose
first byte is an illegal UTF-8 starter. This includes every supported definite
length encoding, arbitrary remaining key bytes and suffixes, and arbitrary
in-bounds cursor offsets. The error preserves the whole fractional input and
restores the cursor, with the exact syntax diagnostic at the original key offset.

`FieldUtf8Reject.lean` proves that such input cannot match a cached ASCII schema
name. It checks all actual Header and GasPrice names and composes the existing
cached-search/fallback proof with the real key-reader rejection. The general
`wp_nextField_failure` contract now explicitly requires ASCII schema names;
both concrete schema callers discharge that premise with checked proofs. Field
reading preserves the search index, seen bits, open state and field metadata;
only the remaining pair count decreases, as the implementation specifies.

The new `KeyFailure.badLead` case propagates through the actual Header and
GasPrice map loops after their supported successful prefixes. Existing GasPrice
slot ownership, Header lookup/value failure, initialized public-API and cleanup
proofs therefore cover illegal-leading-byte keys both at the top level and inside
either nested GasPrice. This adds a rejection family, not a complete UTF-8 or
Header classification. Valid strings, errors after valid character prefixes,
malformed continuation sequences and wider/non-ASCII successful keys remain open.

This checkpoint changes proofs and documentation only. The pinned trusted
semantics and production Go are unchanged; the proposed generic-index repair
remains unapplied. The full Header theorem and frozen performance gate remain open.

### Complete UTF-8 specification classifier and table correspondence

Full regeneration/check passed (2084 build jobs): **740 audited theorems**,
257 matching project hashes and 28 matching standard-library source hashes.
There are no admissions, native-evaluation axioms or additions to the complete
axiom set. The full log is `check-utf8-classification-full.log`.

`Utf8Classify.lean` supplies an exact one-character decomposition of `Utf8Valid`
for every nonempty byte list. A successful `utf8Next` step consumes one to four
bytes; rejection covers an illegal or incomplete character. `utf8CheckFuel` uses
this classifier with a decreasing step budget, and the checked progress bound
shows that the input length always supplies enough steps. `utf8Check_spec` proves
`utf8Check xs = true ↔ Utf8Valid xs` for every byte list. This includes valid
multibyte sequences, invalid continuation bytes, truncated characters, overlong
forms, surrogates and out-of-range scalars, without selecting a finite corpus.

This evaluator is **specification machinery**, not a replacement Go decoder or a
proof of Go's validator. Its grammar is independent of the implementation's
packed classification values and optimized loops.

`Utf8TableSpec.lean` connects that grammar to Go's actual initialized tables.
`utf8LeadInfo_grammar` proves that the independently derived sequence length and
second-byte interval accept exactly the two-, three- and four-byte character
forms. `utf8Tables_classify` checks all 256 starters against the actual first-byte
class table and accept-range table, including index and size/range bounds. A
proved byte-pair view of the accept table removes the abstract Go context from
the finite computation; the equality back to actual struct entries is checked.
All finite computation uses kernel reduction, not native-evaluation axioms.

The next production obligation is to use these lemmas in the validator's loop
invariant: a consumed valid-character/ASCII prefix preserves the validity of the
remaining bytes, and each branch either advances or returns the specified result.
The ASCII word-helper semantics repair remains unapplied. No general production
validator, successful string decoder or complete Header theorem is claimed here;
existing production UTF-8 proofs still cover illegal initial bytes only.
Production Go, trusted semantics and the frozen performance-gate status are
unchanged by this proof/specification checkpoint.

### UTF-8 scan invariant and input ownership foundations

Full regeneration/check passed (2086 build jobs): **753 audited theorems**,
259 matching project hashes and 28 matching standard-library source hashes.
No admissions, native-evaluation axioms or new axioms were introduced. The full
log is `check-utf8-scan-foundations-full.log`.

`Utf8Window.lean` relates each successful independent character step to its exact
unconsumed suffix and proves validity preservation across that step. Its
`utf8Window` describes the production validator's capacity clamp and successive
suffix slices. Checked pointer/index and slice-composition identities account for
signed-word bounds and the unchanged underlying buffer.

Three weakest-precondition contracts prove the actual translated expression
shapes for the initial full-slice operation, direct byte indexing/loading at any
in-bounds window offset, and suffix slicing by any bounded count. Each preserves
ownership of the **entire original input** at its original fraction. These are
ordinary `IndexRef`/load operations used by the validator's multibyte branches;
they neither invoke nor repair the separate generic `Index` rule used by its
ASCII word helper.

`Utf8Scan.lean` defines the pure scan invariant: the current position is in bounds,
and validity of the original input is equivalent to validity of the remaining
suffix. It proves initialization, exact progress by any successfully classified
character, progress by arbitrary ASCII batches, validity on exhaustion and
invalidity when the next nonempty suffix is rejected. Those statements allow
malformed bytes after an arbitrary valid prefix; they are not restricted to a
fixed record, character width or string length.

The production validator's loop still needs to compose these facts with its
actual table lookups, continuation-byte tests, control flow and ASCII word path.
This checkpoint supplies expression contracts and an invariant, not the missing
whole-function theorem. The generic-index semantics repair remains unapplied.
Production Go and trusted semantics are unchanged, and the complete Header and
frozen performance gates remain open.

### Production UTF-8 loop composition and precise remaining obstruction

Full regeneration/check passed (2086 build jobs): **755 audited theorems**,
259 matching project hashes and 28 matching standard-library source hashes.
No admissions, native-evaluation axioms or new axioms were introduced. The full
log is `check-utf8-loop-descriptor-full.log`.

`Utf8TableSpec.lean` now relates the independent one-character classifier to
its size/second-byte-interval continuation checks for every non-ASCII descriptor,
including malformed continuations and arbitrary following bytes. A separate
lemma rejects every descriptor whose required bytes are truncated. These are
checked supporting theorems, not assumptions about the production validator.

At this checkpoint, the general all-input validator proof attempt lived in
`repros/Utf8Loop.lean`, outside the checked/audited target. Its multibyte and
single-ASCII paths elaborated, but the whole theorem failed at the generic word
helper's first byte load: the pinned model supplied `IndexRef (0, window)` instead
of `IndexRef (window, 0)`. No complete validator theorem was counted here.

**Superseded by “Complete UTF-8 loop reduced to the word helper” below.** The old
loop attempt has now moved into `ProofCheck/Utf8Loop.lean`, with all ASCII batching
branches completed under an explicit word-helper premise. The remaining concrete
semantics obstruction is still reproducible with:

```sh
lake env lean repros/Utf8Word.lean
```

That helper reproducer is still expected to fail and remains outside the audited
proof target. The proposed trusted repair remains unapplied. The later checked
conditional loop proof does not claim to have proved the helper or completed Header.

### Complete independent string outcome specification

Full regeneration/check passed (2089 build jobs): **766 audited theorems**,
262 matching project hashes and 28 matching standard-library source hashes.
No admissions, native-evaluation axioms or new axioms were introduced. The full
log is `check-string-specification-full.log`.

`StringSpec.lean` now defines the complete string-field wire contract independently
of fxamacker and of a production-validator correctness assumption. `StringValid`
preserves the previous Go string for null/undefined; otherwise it accepts a
complete definite text payload exactly when the independent UTF-8 grammar holds.
Declared length equals payload length in mathematical integers, preventing word
wraparound from creating spurious encodings. `StringRejected` covers every
expectation failure, truncated payload and invalid UTF-8 payload, including errors
after arbitrarily long valid character prefixes. Truncation takes precedence over
validation of the available payload bytes.

Checked theorems establish that every byte list has a specified outcome; a
successful result has exactly one consumed prefix, output value and suffix;
success consumes at least one byte; and success and rejection are disjoint.
Nullish preservation deliberately does not require the previous Go string to be
valid UTF-8. The existing production-proved `StringFailure` cases embed into the
full rejection specification under the representable-input-length bound already
implied by actual slice ownership.

`HeadDeterminism.lean` supplies reusable uniqueness facts for head width, argument
value and exact remaining suffix. `ExpectedDeterminism.lean` proves that a complete
head of the requested major type cannot also have a specified expectation error,
including incomplete extended arguments. These support the string disjointness
proof and later enclosing-record classification.

This is a complete **specification classification**, not a completed string-reader
implementation proof. The latter must connect the actual UTF-8 validator and
byte-to-Go-string conversion to these relations, then propagate them through keys,
GasPrice and Header. Existing production contracts retain their documented
restrictions. The pending generic-index semantics repair, full Header theorem and
frozen performance gate remain open. Production Go and pinned trusted semantics
are unchanged.

### String conversion ownership and complete key outcome specification

Full regeneration/check passed (2091 build jobs): **772 audited theorems**,
264 matching project hashes and 28 matching standard-library source hashes.
No admissions, native-evaluation axioms or new axioms were introduced. The full
log is `check-key-specification-full.log`.

`StringConversion.lean` proves the actual byte-slice-to-string conversion used
by the successful string reader. Given any bounded payload subrange, it returns
exactly those bytes as the Go string and restores fractional ownership of the
entire original input buffer. It composes Perennial's checked conversion theorem
with subrange borrowing; it adds no conversion semantics or helper-correctness
assumption. Conversion accepts arbitrary bytes, so the separate UTF-8 validator
still must establish the text requirement before this result is used by String.

`KeySpec.lean` specifies every supported key encoding independently of dispatch
shortcuts and field caches: definite UTF-8 text at every head width, and positive
or negative integer keys whose argument is below 2^63. Integer keys produce a
non-text result and are ignored by the Header/GasPrice schema lookup. Unlike
string field values, null and undefined are rejected as keys. Declared text
lengths are related to payload lengths in mathematical integers.

The specification covers all key errors, including arbitrary malformed UTF-8,
head faults/truncation, missing input, integer overflow, tags, wrong major types
and truncated text payloads. Checked theorems show that every input has an
outcome, successful keys consume bytes, successful prefix/result/suffix triples
are unique, and success and rejection are disjoint. Existing production-proved
`KeyFailure` cases embed into the full relation under the representable-length
bound supplied by actual slice ownership.

These results remove specification and conversion obligations from the remaining
key/string work. They do not complete the production UTF-8 or Key proofs, wider
and non-ASCII field-lookup proofs, exhaustive GasPrice/Header classification or
the final Header contract. The general validator remains at the documented
unapproved generic-index repair. Production Go and pinned trusted semantics
are unchanged, and the frozen performance gate remains open.

### Complete Header/GasPrice key interpretation

Full regeneration/check passed (2092 build jobs): **793 audited theorems**,
265 matching project hashes and 28 matching standard-library source hashes.
No admissions, native-evaluation axioms or new axioms were introduced. The full
log is `check-schema-key-specification-full.log`.

`SchemaKeySpec.lean` connects the complete key wire specification to the actual
Header and GasPrice application schemas. Integer keys are unknown fields; text
keys match the ASCII-folded wire names or are unknown. Header's fifteen schema
identities and both legacy gas-price names are checked against the actual field
metadata. Each schema's text interpretation is total and unique.

The generic schema-key relation now has all-input classification, exact
successful prefix/field-choice/suffix uniqueness, nonempty consumption and
success/rejection disjointness. Both schemas instantiate that relation without
restricting text to ASCII or short head encodings. The existing production-proved
`HeaderKeySupported` and `GasKeySupported` families embed into the new contracts
under representable input-length bounds, preserving their previous meanings.

Checked ASCII-folding facts additionally show that a non-ASCII key cannot match
any name in these ASCII-only schemas. A valid non-ASCII text key is therefore an
unknown key whose value must be skipped; it is not an alternative spelling of a
known field. UTF-8 validity is still required by `KeyValid`, so this does not turn
malformed text into an accepted unknown key. Duplicate handling for known fields
remains a separate seen-set obligation of the enclosing record specification.

These facts establish complete schema interpretation, not the missing production
Key/NextField contracts. The production UTF-8 proof still requires the documented
unapproved generic-index repair; general field lookup, exhaustive GasPrice/Header
outcome composition, the complete Header theorem and the frozen performance gate
remain open. Production Go and pinned trusted semantics are unchanged.

### General GasPrice record and nullable-pointer specification

Full regeneration/check passed (2093 build jobs): **807 audited theorems**,
266 matching project hashes and 28 matching standard-library source hashes.
No admissions, native-evaluation axioms or new axioms were introduced. The full
log is `check-gasprice-specification-full.log`.

`GasPriceSpec.lean` now describes the complete GasPrice application behavior using
full `GasKeyValid` keys, felt decoding for the two known amounts and recursive
Skip for unknown values. A semantic list of seen field identities enforces
one occurrence per known field while permitting repeated unknown names. This
specification is independent of cache order, planner layout and allocation hooks.

The pair relations cover all supported key widths/UTF-8 and every child outcome.
They specify malformed keys, duplicate detection before value decoding, failed
felt/ignored values, and errors after arbitrary successful prefixes. Each error
retains the partial GasPrice value, cursor consumption and diagnostic offset
separately. The record layer incorporates null/undefined preservation, map
opening, count-preflight precedence and the nesting limit. Count preflight sees
the entire unread suffix, as the production decoder does.

Checked classification theorems provide an outcome for every input under the
depth bound, for pair sequences, direct records and the Header's nullable price
wrapper. The wrapper starts each non-null record from zero and distinguishes nil
from a non-nil record containing two nil amounts. Its failed non-null record keeps
a private partial value until caller cleanup; no such failure can be intercepted
by the wrapper's initial nullish check.

Additional theorems establish nonempty item consumption, at least two bytes per
successful pair, bounded cursor/diagnostic offsets and retained record depth.
Already-seen amounts remain unchanged by both successful remaining pairs and any
later failure. The specifications therefore capture partial-result behavior as
well as decoded values.

This closes general GasPrice **specification classification**, not the production
reader theorem. Whole-record uniqueness/disjointness, the connection to the actual
map/slot implementation, general Header composition and final ownership/API
proofs remain open. The production UTF-8/key path still has the documented
unapproved generic-index repair dependency. Production Go and pinned trusted
semantics are unchanged; the frozen performance gate has not run.

### Complete Header record and public API specification coverage

Full regeneration/check passed (2096 build jobs): **824 audited theorems**,
269 matching project hashes and 28 matching standard-library source hashes.
No admissions, native-evaluation axioms or new axioms were introduced. The full
log is `check-header-specification-full.log`.

`HeaderItemSpec.lean` connects all fifteen Header identities to the complete
independent scalar, felt, string, signature and nullable GasPrice relations.
Unknown fields use recursive Skip. Each field relation specifies its exact
application-value update; failures distinguish consumed bytes, diagnostic offset
and retained child depth. Failed GasPrice values carry private partial contents
without publishing a replacement. Exhaustive enum cases check every field's
category, and every selected field/input has a specified outcome.

`HeaderRecordSpec.lean` composes these into arbitrary-order pair and record
relations using the complete Header key contract and semantic seen-field lists.
Known duplicates are rejected after their key and before their value; unknown
names may repeat. Count preflight and nesting checks precede pair processing.
Every input has a successful record prefix and exact suffix, or a specified error
with the last published Header value and exact cursor/diagnostic/depth progress.
Nonempty consumption, pair-count bounds and error bounds are proved.

`HeaderAPISpec.lean` adds the independent public policies: nonzero destinations
are rejected unchanged; First starts from zero, returns the exact suffix on
success and clears partial output on parse errors; full decoding rejects a
nonempty suffix and clears the otherwise successful output. Every initial
application value and input has a First/full specification outcome. Additional
theorems establish exact success consumption, cleanup policy and bounded decode
diagnostic offsets. Concrete slice aliasing/nil error suffixes, output heap
ownership and private allocation cleanup remain implementation-proof obligations.

This closes **all-input specification coverage** for Header and both public APIs.
It does not prove the general translated Go implementation correct. Whole-record
uniqueness/disjointness, production UTF-8/key and broader field-lookup proofs,
GasPrice/Header loop refinement and final public ownership proofs remain open.
Existing production API theorems still apply only to their documented restricted
families. The generic-index semantics repair remains unapplied and awaits explicit
approval. Production Go and pinned trusted semantics are unchanged; the frozen
performance gate has not run.

### Production-proved families refine the complete specification

Full regeneration/check passed (2101 build jobs): **853 audited theorems**,
274 matching project hashes and 28 matching standard-library source hashes.
The transitive axiom union is unchanged: no admissions, native-evaluation axioms
or new axioms. The full log is `check-supported-spec-refinement-full.log`.

`SupportedSpec.lean` connects the existing successful Header/GasPrice wire
families to the complete independent specification. The actual seen-bit masks
correspond to membership in semantic field lists; setting a field bit corresponds
to recording that identity. Induction over successful entries preserves byte
order, fresh-field checks and every decoded value. Nested GasPrice results retain
the distinction between nil and an allocated record with nil amounts. The
representable-length premises exclude mathematical word-wrap cases and are
derived from actual slice ownership in the API theorems.

`SupportedFailureSpec.lean` and `SupportedHeaderFailureSpec.lean` connect all
existing GasPrice failure constructors and Header lookup/value failure witnesses
to the complete record relations. They preserve every successful prefix update,
duplicate-before-value precedence, bytes consumed, diagnostic offset and retained
nested depth. Failed GasPrice values retain their private partial record without
publishing it into the Header.

`SupportedAPI.lean` and `SupportedFailureAPI.lean` provide seven actual production
WP contracts with the independent `HeaderFirstSpec`/`HeaderFullSpec` in their
postconditions: successful First/full decoding, full trailing-data rejection, and
lookup/value failures through both APIs. They preserve exact suffix aliasing,
whole fractional input ownership, output/diagnostic ownership and actual cleanup.
Tables and error globals come from the checked initialization contracts; no
missing helper correctness is assumed.

These theorems retain the existing `HeaderMapWire`, `HeaderLookupFailure` and
`HeaderValueFailureWire` restrictions. In particular, successful prefixes still
use the proved key families and nullish ProtocolVersion values. This connects
previous production proofs to the new specification; it does not expand their
input coverage or complete the general Header milestone. Whole-record
uniqueness/disjointness, production UTF8/key completion, general field lookup,
record-loop refinement and final arbitrary-input public contracts remain open.
The proposed generic-index semantics repair remains unapplied. Production Go and
pinned trusted semantics are unchanged, and the frozen performance gate has not
run.

### Unique successful outcomes for the complete Header specification

Full regeneration/check passed (2107 build jobs): **874 audited theorems**,
280 matching project hashes and 28 matching standard-library source hashes.
The transitive axiom union is unchanged, with no admissions or native-evaluation
axioms. The full log is `check-header-success-determinism-full.log`.

`WireDeterminism.lean` proves that exact item boundaries and decoded values lift
to fixed-length sequences without requiring equality of proof-bearing witnesses.
It also proves scalar value/prefix/suffix uniqueness and scalar success/error
disjointness, including null/undefined preservation and every unsigned head width.

`SkipDeterminism.lean` proves unique consumption for all recursively valid ignored
values: atoms, arrays, maps and tags. It preserves the existing deliberate lack of
UTF8 validation for ignored text and the chained-tag depth policy.
`FeltDeterminism.lean` normalizes short and extended felt arrays into up to four
unsigned limbs plus ignored extras, then proves unique values and boundaries for
every successful felt encoding. `SignaturesDeterminism.lean` lifts that result
through nullable felt-pointer slices and nested signatures, including nil versus
allocated-empty distinctions at both levels.

`GasPriceDeterminism.lean` proves complete child-value, pair, record and nullable
pointer success uniqueness. `HeaderDeterminism.lean` checks that every schema
identity selects one field kind/update, then composes key and child determinism
through arbitrary successful Header pair sequences and complete records. Two
successful `HeaderFirstSpec` outcomes on the same input have the same decoded
Header and exact unread suffix; two successful `HeaderFullSpec` outcomes have the
same Header value. These results apply to the complete independent specification,
including all text keys/strings, every field order and nested child encoding.

This closes successful-outcome uniqueness, not full specification determinism:
success/error disjointness and unique failure values/progress/diagnostics remain
open. It does not prove the missing production UTF8/key paths or the general
Header implementation theorem. The trusted generic-index repair remains unapplied;
production Go and pinned semantics are unchanged. The frozen performance gate has
not run.

### Complete Header success/error separation on representable input

Full regeneration/check passed (2113 build jobs): **900 audited theorems**,
286 matching project hashes and 28 matching standard-library source hashes.
The transitive axiom union is unchanged, with no admissions or native-evaluation
axioms. The full log is `check-header-success-error-separation-full.log`.

`StructOpenDisjoint.lean` proves that complete successful Header/GasPrice prefixes
cannot fail their opening checks. It accounts for null/undefined precedence,
exact decoded heads, pair counts and depth.

`SkipDisjoint.lean` normalizes accepted bodies, excludes every failure leaf on a
successful prefix, and lifts that exclusion through arbitrary enclosing failure
contexts. Deterministic sibling boundaries identify a valid next child after any
strict valid prefix. `FeltDisjoint.lean` then excludes all felt errors: outer
expectations, depth, unsigned-limb failures and failures in recursively skipped
extras. `SignaturesDisjoint.lean` composes these results through both nullable
slice levels, including failures after arbitrary successful child prefixes.

The array failure relations use mathematical natural counts converted to 64-bit
words. Their disjointness theorems therefore require the entire input length to
be below 2^64; the count preflight then excludes word wrapping. This bound follows
from physical Go slice ownership on the pinned target. No disjointness claim is
made for unrepresentable mathematical inputs where those legacy natural-count
witnesses could wrap. Header inherits this explicit bound through signatures.

`GasPriceDisjoint.lean` proves complete value/pair/record/pointer separation,
including key failures, duplicates before values, child errors and opening errors.
`HeaderDisjoint.lean` checks every selected field kind and composes the complete
Header pairs and records. Successful `HeaderFirstSpec` outcomes exclude all
failure outcomes on the same representable input and initial Header. Full
success also excludes trailing-data errors using the already-proved unique First
suffix, as well as parse errors and destination-guard errors.

Together with the previous success uniqueness proofs, the specification now has
at most one successful outcome and cannot both succeed and fail on representable
input. Unique failed values/progress/diagnostics remain to be proved. These new
results do not add production WP coverage or close the general Header decoder
milestone. Production UTF8/key completion, broader field lookup, general record
refinement and final arbitrary-input implementation contracts remain open. The
trusted generic-index repair remains unapplied, production Go is unchanged, and
the frozen performance gate has not run.

### Unique primitive and recursive Skip failure diagnostics

Full regeneration/check passed (2118 build jobs): **916 audited theorems**,
291 matching project hashes and 28 matching standard-library source hashes.
The transitive axiom union is unchanged, with no admissions or native-evaluation
axioms. The full log is `check-primitive-skip-failure-uniqueness-full.log`.

`ExpectedFailureUnique.lean` proves unique head-fault, type-expectation and scalar
failure reasons, including the offending head byte. `OpeningFailureUnique.lean`
proves unique complete string and typed-map-opening diagnostics. It checks
expectation versus complete-head errors, truncation before UTF8 validation, and
count truncation before depth rejection.

`KeyFailureUnique.lean` proves unique diagnostic reasons for the complete key
rejection relation: malformed/incomplete heads, EOF, integer overflow, tags,
type mismatch, text truncation and arbitrary malformed UTF8. The existing
production-proved key/string failure families inherit uniqueness through their
checked bounded embeddings into the complete relations.

`SkipLeafFailureUnique.lean` proves unique atomic rejection reasons and exact
local Skip offsets/reasons, including count/depth and chained-tag EOF/depth
precedence. `SkipFailureUnique.lean` extends that to arbitrary recursive failure
paths. A local error cannot also lie below supported enclosing checks. Equal
successful sibling counts have identical byte boundaries; unequal counts would
place a successful child where the other path already fails, contradicting the
checked success/error separation. Induction through containers and chained tags
then fixes the entire failure offset and reason.

This closes failure determinism for these primitive contracts and all recursive
Skip errors. Felt, nullable/nested slices, GasPrice and Header still need their
failure-determinism composition. No additional production WP coverage is claimed;
UTF8/key completion, broader field lookup, general record refinement and the final
Header implementation theorem remain open. Production Go and pinned trusted
semantics are unchanged, the generic-index repair remains unapplied, and the
frozen performance gate has not run.

### Complete Header specification outcome uniqueness

Full regeneration/check passed (2124 build jobs): **935 audited theorems**,
297 matching project hashes and 28 matching standard-library source hashes.
The transitive axiom union is unchanged, with no admissions or native-evaluation
axioms. The full log is `check-header-outcome-uniqueness-full.log`.

`WireFailurePrefix.lean` aligns successful sibling prefixes before a failing
child. Unequal prefix lengths would place a successful item at a failure point,
contradicting the checked success/error separation. `FeltFailureUnique.lean`
uses this for unsigned limbs and recursively skipped extras, and proves exact
failure offsets and reasons for every felt failure constructor.

`SliceFailureUnique.lean` carries uniqueness through nullable felt-pointer slices
and nested signatures, including consumed bytes and retained nesting depth.
`GasPriceFailureUnique.lean` proves complete child/pair/record failure uniqueness,
fixing the partially populated value, cursor progress, diagnostic offset, depth
entry and reason. It also transfers this result to the existing production-proved
GasPrice failure relation.

`HeaderFailureUnique.lean` checks all field-kind combinations and composes key,
duplicate, child and opening failures through arbitrary pair prefixes and the
whole record. It uniquely determines the partial Header, consumed bytes, exact
diagnostic, retained nested depth and reason.

`HeaderOutcomeUnique.lean` closes failure and arbitrary-outcome uniqueness for
both public specifications. Together with their existing all-input classification,
`headerFirstSpec_total_unique` and `headerFullSpec_total_unique` establish exactly
one specified result for every initial Header and byte input of length below
`2^64`. This includes successful values and exact First suffixes, nonzero-destination
rejection, parse-error cleanup and trailing-data counts. The length bound excludes
natural-count wrapping in legacy slice-failure witnesses; physical Go slice
ownership supplies a stronger bound.

These are independent specification theorems. They neither add production WP
coverage nor prove execution or termination. The general production UTF8 validator,
Key/field lookup, complete GasPrice/Header refinement and final owned public API
contracts remain open. The proposed trusted generic-index repair is unapplied.
Production Go and pinned trusted semantics are unchanged; the frozen performance
gate has not run.

### General key-to-field proof composition

Full regeneration/check passed (2125 build jobs): **941 audited theorems**,
298 matching project hashes and 28 matching standard-library source hashes.
The transitive axiom union is unchanged, with no admissions or native-evaluation
axioms. The full log is `check-field-key-composition-full.log`.

`wp_otherField_text` composes a proved text Key contract with the actual borrowed
name matching, case folding, seen-bit update and owned duplicate diagnostic.
`wp_nextField_miss_text` and `wp_nextField_text` lift this through the actual
cache/search paths, preserving exact cursor/suffix/search-index progress and
fractional input/table ownership. Their statements no longer restrict text keys
to ASCII or inline/one-byte length encodings. The existing closed ASCII theorems
now instantiate this composition with `wp_Key_ascii`; their caller contracts and
proved input families are unchanged.

`FieldKeySpec.lean` proves cached ASCII schema entries satisfy the complete
independent `KeyValid` relation. Consequently every `KeyRejected` input misses
every cached entry, including all malformed UTF8 sequences. The production
`wp_nextField_keyRejected` composition takes a proved Key rejection through
`otherField` and public cache/search dispatch, retaining exact owned errors,
cursor state and the one-pair decrement.

The general success and rejection wrappers have **explicit Key WP premises**.
They do not add closed wider/non-ASCII Key coverage, assume validator correctness,
or complete the Header implementation theorem. They isolate the remaining parser
obligation so that the eventual general Key proof can reuse the checked matching,
duplicate and cache composition. The trusted generic-index repair remains
unapplied and the frozen performance gate has not run. Production Go is unchanged.

### Complete text caller contracts with an explicit validator obligation

Full regeneration/check passed (2133 build jobs): **961 audited theorems**,
306 matching project hashes and 28 matching standard-library source hashes.
The transitive axiom union is unchanged, with no admissions or native-evaluation
axioms. **15 theorems explicitly require the unproved validator contract**; their
names are recorded separately in the audit and are not closed correctness claims.
The full log is `check-text-contract-composition-full.log`.

`Utf8Contract.lean` names the outstanding production `unicode/utf8.Valid` WP as
`Utf8ValidatorContract`. This is a proposition passed explicitly to callers, not
an axiom, opaque declaration, typeclass instance or claim that the validator is
proved. Its range composition borrows a payload and restores the entire fractional
input. The audit now lists every theorem with this direct unresolved premise
under `unproved_contract_premises`, separately from transitive axiom dependencies.

`StringText.lean` proves the actual complete text path through payload extraction,
validation and byte-slice-to-string conversion, and the actual rollback/owned
error path for arbitrary malformed UTF8. `StringComplete.lean` composes these
with null/undefined old-value preservation, expectation errors and truncation.
`wp_readString_complete` covers every input under the independent `StringValid`
and `StringRejected` relations, with exact suffixes, unchanged input, destination
ownership and preservation on failure, **conditional on the validator contract**.

`HeaderStringComplete.lean` composes the real ProtocolVersion field call. Success
updates exactly that field; failure preserves it. Both branches retain every
other Header field, all nested child ownership and the private shared GasPrice
allocation/readiness invariant. This removes the old nullish-only restriction
from the conditional field contract, not from the existing closed Header map/API
theorems.

`KeyTextSlow.lean` proves text success and arbitrary UTF8 rejection through the
actual slow key reader at every definite-length head width. `KeyTextDispatch.lean`
proves that complete short non-ASCII keys and wider heads reach that reader with
unchanged input/cursor. `KeyText.lean` composes the slow path with these proofs
and the already-checked ASCII shortcut, normalizes the direct/nested borrowed
slice representations, and proves public text success and malformed-UTF8
rejection conditional only on the validator contract.

`FieldTextComplete.lean` supplies those Key contracts to the general lookup
composition. Arbitrary valid text keys now have an exact field/search/duplicate
contract, and every independent `KeyRejected` case propagates through public
NextField with cursor rollback, owned diagnostics and one-pair progress, again
conditional on the validator. Integer-key success already has a closed contract.

The trusted generic-index repair remains unapplied. The validator premise still
must be proved; it is not acceptable as an assumption in the final Header milestone.
General GasPrice/Header map refinement and final public API composition remain
open. Production Go is unchanged, and the frozen performance gate has not run.

### Complete GasPrice record and Header price-field composition

Full regeneration/check passed (2145 build jobs): **982 audited theorems**,
318 matching project hashes and 28 matching standard-library source hashes.
The transitive axiom union is unchanged, with no admissions or native-evaluation
axioms. **36 theorems explicitly require the unproved validator contract**;
all 21 new theorems in this checkpoint are conditional in that way. Their names
are recorded separately in `unproved_contract_premises`. The full log is
`check-gasprice-complete-full.log`.

`SchemaKeysComplete.lean` supplies actual NextField contracts directly against
`GasKeyValid` and `HeaderKeyValid`, including arbitrary supported text widths,
UTF-8 names, integer/unknown keys, fresh fields and duplicate rejection. Search
bounds, seen masks, pair-count progress, owned errors and exact key consumption
are preserved. These contracts remove the previous successful-key witness
restriction from the new composition proofs.

`GasPriceMapComplete.lean` and `GasPriceMapRejected.lean` prove the actual generated
loop directly against `GasPairsValid` and `GasPairsRejected`. The invariant relates
the semantic seen-field list to the machine mask and tracks the remaining pair
specification, owned partial record and exact input suffix. Failure tracks retained
consumption separately from the diagnostic offset. It covers rejected keys,
duplicates and felt/Skip errors after arbitrary successful prefixes. Failed values
preserve the current record; earlier fields stay owned. Success closes the map and
restores its incoming depth; pair failure retains the entered map depth.

`GasPriceRecordComplete.lean` and `GasPriceRecordRejected.lean` add null/undefined
old-value preservation and all opening failures. `GasPriceComplete.lean` combines
these with independent all-input classification into `wp_readGasPrice_complete`:
every input has the specified owned successful or rejected outcome, conditional
on the validator. This is a partial-correctness contract, not a termination claim.

`GasPriceSlotValid.lean` and `GasPriceSlotRejected.lean` lift the full contracts
through both existing and newly allocated shared storage. On failure the selected
slot retains exactly its private partial contents and the caller's destination
pointer is unchanged. Both slots and their nested felt allocations remain owned.
`HeaderGasMapValid.lean`, `HeaderGasRejected.lean` and
`HeaderGasFreshComplete.lean` connect this to either Header GasPrice field while
preserving every other public field. A first use of either slot starts from zero,
including when its sibling already allocated the shared pair.

`HeaderGasComplete.lean` composes null/undefined, complete successful maps and every
record rejection through the actual Header field call. `wp_headerGas_valid` covers
`GasPointerValid`, distinguishing nil from an allocated record with nil amounts.
`wp_headerGas_complete` gives an arbitrary-input field outcome: success publishes
the specified value and exact suffix; failure preserves the entire public Header
and retains private partial allocation ownership for cleanup.

All new production contracts still require `Utf8ValidatorContract`. The trusted
generic-index repair remains unapplied, and these results do not close the general
Header milestone. The next independent work is the complete Header success/failure
loop, followed by record/public API composition and cleanup against
`HeaderFirstSpec`/`HeaderFullSpec`. The validator must also be proved before the
milestone passes. Production Go and pinned trusted semantics are unchanged; the
frozen 185-case performance gate has not run.

### Complete Header loops and initialized zero-destination APIs

Full regeneration/check passed (2154 build jobs): **995 audited theorems**,
327 matching project hashes and 28 matching standard-library source hashes.
The transitive axiom union is unchanged, with no admissions or native-evaluation
axioms. **49 theorems explicitly require the unproved validator contract**;
all 13 new implementation theorems in this checkpoint have that premise. They
are separately listed in `unproved_contract_premises`. The full log is
`check-header-zero-complete-full.log`.

`HeaderStringValid.lean` and `HeaderStringRejected.lean` expose direct field
contracts for complete independent string success/rejection, preserving the other
fields and private shared state. Successful ProtocolVersion reads also mark the
field in the loop invariant. Null/undefined preserve the prior string value.

`HeaderMapComplete.lean` proves the actual generated Header loop directly against
`HeaderPairsValid`. It covers all fifteen fields, arbitrary supported key encodings
and field order, arbitrary valid strings, felt limbs, nested signatures, complete
nullable GasPrice records and ignored nested values. Its invariant connects the
semantic seen-field list to the machine mask, the remaining pair specification to
the partial Header, and the shared GasPrice storage to the publication state.
Successful map closure restores the incoming depth, preserves the whole fractional
input, returns the exact suffix and publishes independently owned children.

`HeaderMapRejected.lean` proves every `HeaderPairsRejected` outcome after arbitrary
successful prefixes. Malformed keys, duplicates and all child-value failures have
the exact partial public Header, error category and owned diagnostic. Retained
consumption, diagnostic position and retained child depth are tracked separately.
A failed field never publishes its replacement; private partial allocations remain
accounted for until the loop's ownership is converted to the caller-visible result
under the existing GC/affine model. This is not a physical reclamation claim.
`HeaderRecordError.lean` maps the independent record reasons to actual diagnostic
ownership, including duplicate-field arguments.

`HeaderRecordComplete.lean` adds null/undefined and every opening error, yielding
complete successful/rejected record contracts from the public zero destination.
`HeaderValidAPI.lean` proves actual First success with exact suffix aliasing, full
success, and trailing-byte rejection with cleanup. `HeaderRejectedAPI.lean` proves
both actual public error paths with a cleared destination, unchanged input and
exact diagnostic; First returns the nil suffix on failure.

`HeaderZeroComplete.lean` obtains immutable field metadata and canonical error
globals from the proved relevant package initialization. Its two all-input public
contracts, `wp_decodeHeaderFirst_zero_complete` and `wp_decodeHeader_zero_complete`,
combine the independent record classification with the implementation WPs. They
state results against `HeaderFirstSpec` and `HeaderFullSpec`, including owned
outputs/errors, exact suffixes, trailing-data policy and cleanup. Callers provide
initialization, a zero destination and input ownership; they do not provide
metadata, error globals or a record-decoder correctness assumption.

These are **conditional partial-correctness theorems**, not a completed Header
proof: `Utf8ValidatorContract` remains unproved and the trusted generic-index
repair remains unapplied. The already-checked nonzero-destination guard must also
be joined to the unified independent-spec outcome, relating concrete Header zero
checks to the owned abstract value. Those are the next Header proof obligations.
Production Go and the pinned trusted semantics are unchanged. Termination and
compiler/runtime implementation correctness remain outside the claim, and the
frozen 185-case performance gate has not run.

### Unified all-destination Header API composition

Full regeneration/check passed (2157 build jobs): **1003 audited theorems**,
330 matching project hashes and 28 matching standard-library source hashes.
The transitive axiom union is unchanged, with no admissions or native-evaluation
axioms. **51 theorems explicitly require the unproved validator contract**;
the two new public implementation contracts have that premise. The six new
ownership/result bridge theorems have no validator premise. The full log is
`check-header-api-complete-full.log`.

`HeaderOwnedZero.lean` proves that the concrete Header zero test agrees exactly
with the independent abstract value under `HeaderChildren`. Allocated zero felts,
allocated empty signature slices and allocated GasPrice records with nil amounts
remain distinct from absent children. This discharges the ownership connection
needed to use the separately checked guard with the independent API specification.

`HeaderAPIOutcome.lean` defines a common owned representation of successful and
failed First/full results and converts the complete zero-destination outcomes to
it. Success retains the decoded Header and First's exact aliased suffix. Failures
retain the specified destination and a concrete diagnostic tied to initialized
canonical error sentinels; parser offsets have an explicit no-wrap equality.
First failures return the nil suffix.

`HeaderAPIComplete.lean` proves `wp_decodeHeaderFirst_complete` and
`wp_decodeHeader_complete` for **every owned initial Header and every byte input**.
The nonzero branch preserves the original Header and its nested ownership, and
returns the canonical unsupported error. The zero branch composes the complete
record and wrapper proofs, including all fifteen fields, nested children, cleanup
and trailing-data policy. Both branches preserve fractional input ownership.
The postconditions state `HeaderFirstSpec` / `HeaderFullSpec` and the concrete owned
result. Callers supply relevant initialized packages, an owned initial Header and
input ownership; no record-decoder, metadata or initializer WP is assumed.

**The general Header milestone remains open:** these two API theorems explicitly
require `Utf8ValidatorContract`, which is not proved. The proposed trusted generic
slice-index repair remains unapplied. The destination-guard composition is now
complete; the next Header obligation is to discharge the actual production UTF-8
validator premise and instantiate the final API theorems without it. Repairing the
pinned semantics requires the separately documented explicit approval below.
Production Go and the pinned trusted semantics are unchanged in this checkpoint.
The formal claim remains partial correctness, excluding termination and compiler/
runtime implementation correctness. No new performance-gate results are claimed.

### Complete UTF-8 loop reduced to the word helper

Full regeneration/check passed (2161 build jobs): **1008 audited theorems**,
334 matching project hashes and 28 matching standard-library source hashes.
The transitive axiom union is unchanged, with no admissions or native-evaluation
axioms. The full log is `check-utf8-loop-from-word-full.log`.

`Utf8Batches.lean` proves the actual eight-, sixteen- and thirty-two-byte OR-mask
conditions against the independent per-byte ASCII predicate and scan invariant.
These three pure theorems have no helper correctness premises. A failed ASCII mask
makes no assertion that the input is invalid UTF-8; it falls back to character checks.

`Utf8Loop.lean` now checks the actual complete translated `unicode/utf8.Valid`
implementation, including capacity clamping, all two/three/four-byte character
checks, malformed continuations, illegal leading bytes, truncation, success at
exhaustion and all ASCII fast-path control flow. The nested loop handles arbitrarily
many thirty-two-byte batches. Every branch preserves fractional ownership of the
whole original input and relates the returned boolean to `Utf8Valid`.

The theorem `wp_utf8_valid_from_word` explicitly requires
`Utf8WordWindowContract`: a WP for the actual resolved generic `word[[]byte]` call,
reading eight bytes at an arbitrary valid window offset and preserving the whole
input. This is a proposition supplied by the caller, not an axiom or an instance.
**That helper remains unproved.** Its generic slice load encounters the documented
pinned-semantics defect, and the proposed trusted repair remains unapplied.

`Utf8FromWord.lean` checks `Utf8WordWindowContract → Utf8ValidatorContract` through
the complete implementation loop. Consequently the proof dependency is now:

```text
actual generic eight-byte helper (unproved)
    → complete actual UTF-8 validator (checked implication)
    → complete actual Header APIs (checked implications)
```

The audit separately lists **51 direct validator-premise theorems** and **2 direct
word-helper-premise theorems**. Its binder matching distinguishes an assumed
contract from a contract produced as a theorem's conclusion. The former unaudited
loop attempt was removed; the current loop and its supporting modules are imported
by the checked root and included in source hashes and transitive axiom reports.
`repros/Utf8Word.lean` remains the excluded expected-failure reproducer.

This completes the validator's control-flow composition, **not the unconditional
Header milestone**. The next Header work is to prove the actual word helper after
the separately approved semantics repair, supply it to the checked implication,
and instantiate the public API contracts without unresolved helper premises.
Production Go, the trusted semantics pin and the formal scope are unchanged.
Termination, compiler/runtime correctness and performance remain separate; the
frozen 185-case performance gate has not run.

### Checked remaining-entry inventory (preparatory)

`results/entry-manifest.json` is now generated and checked by
`cmd/entrymanifest`. It scans all **154 build-selected repository packages** with
Go types and syntax on Go 1.27.0, linux/amd64/v1, excluding tests. It finds concrete
instantiations of `FastCbor`, direct fast `Unmarshal`/`UnmarshalFirst`, typed-reader
registration and `indexed.LazySlice`/`NewLazySlice`. Generic forwarding sites remain
explicit in the artifact. Package import aliases do not hide concrete roots.

The result has **14 production destination types**, plus **1 benchmark-only
destination**, **140 structural Go shapes**, **18 ordered registry entries**, and
all **185 frozen benchmark case IDs**. Header, its five projections and block indexes have registered typed readers.
The seven variants reachable from the two production interfaces are five transaction
variants and two class variants. The other registry entries are recorded, without
silently expanding the fast production-root scope to trie journals or consensus WAL.

| Destination group | Entries | Remaining decoder work |
| --- | ---: | --- |
| `Header` | 1 | Discharge the actual UTF-8 word-helper premise; the full API composition is checked conditionally |
| Header projections | 5 | Typed implementation and static metadata generated; prove projection/NextWanted semantics, table initialization and selected output ownership |
| Receipt pointer and receipt projections | 3 | `*TransactionReceipt`, events and execution-status projections; nested records/slices, bool/string handling and cleanup |
| `Transaction` and hash projection | 2 | Five registered pointer variants, version/felt representation, resource-bounds map and projection tag behavior |
| `StateUpdate` | 1 | Felt-key maps, nested storage-diff maps, migrated-class typed felt keys/values, allocations and duplicate/error policy |
| `ClassDefinition` | 1 | Two registered variants, recursive `SegmentLengths`, raw JSON values and felt slices; public BigInt dependency now proved |
| `BlockTransactionsIndexes` | 1 | Typed integer-key record and signed `[]int` implementation generated; independent relations and complete API proofs remain open |
| `TransactionReceipt` by value | 1 benchmark-only | Preserve the historical destination type instead of conflating it with the production pointer root |

The four map shapes are `map[Resource]ResourceBounds`, `map[felt.Felt]*felt.Felt`,
`map[felt.Felt]map[felt.Felt]*felt.Felt`, and
`map[felt.SierraClassHash]felt.CasmClassHash`. The graph records raw struct fields,
embedding, CBOR/JSON tags, custom decoder hooks, pointers/slices/arrays, recursive
references and registered interface variants. **This graph is not an independent
wire specification or a feasibility proof.** External named representations,
custom felt/slice encodings and projection discard markers are explicit boundaries;
field precedence, hooks, overrides and shared allocation still need contracts.

Every historical benchmark case maps to its constituent decoder roots or an
explicit incompatibility candidate. The six removed Header bloom-projection cases
remain present; they are not silently dropped or marked passed. Compound block
operations map to the index reader and the element/projection readers they call.
The benchmark-only receipt value root remains separate from the production pointer.
No compatibility smoke, timing, allocation or no-fallback gate is claimed here.

From the repository root:

```sh
go run ./verification/fastcbor-proof/cmd/entrymanifest
go run ./verification/fastcbor-proof/cmd/entrymanifest -check
```

`check.sh` now runs the drift check before regeneration and Lean. The proof audit
hashes the command and emitted manifest, and the manifest records **29 source
hashes**, including the frozen benchmark universe and relevant planner/registry/
wrapper sources. A behavioral drift probe confirmed that an unrelated declaration
leaves the manifest current, while adding a concrete `FastCbor[uint64]` site through
an aliased import makes `-check` fail. The temporary probe was removed, and the
restored-source drift check passed.

Full regeneration/check passed (2161 build jobs): **1008 audited theorems**,
**336 project hashes** and 28 standard-library hashes. The theorem dependency sets
are unchanged; there are no admissions or native-evaluation axioms. The full log is
`check-entry-manifest-full.log`. Production Go decoder code and pinned semantics
are unchanged. This is preparatory work while the separately required trusted
repair approval remains outstanding; Header is still the first proof milestone.

### Frozen-corpus compatibility smoke on both revisions

`smoke/run.py` now ports the frozen historical case universe to the actual APIs
available on untouched baseline `a4a67242cc67f3855915a97bf4e79a97d217fae4` and the
current candidate. It uses Go overlays held in temporary directories: neither
worktree's production files are edited. Historical sources and all corpus files
are hash-checked before running; each case retains its frozen ID, input length and
class framing/prefix behavior. `results/compatibility-smoke.json` records both
revisions, all tracked Go source hashes, overlay/harness hashes and every result.

| Classification | Baseline | Candidate |
| --- | ---: | ---: |
| Compatible fast decode, equal to FX | 177 | 177 |
| Removed `headerEventsBloomProjection` | 6 | 6 |
| Empty lazy operations with zero decoder calls | 2 | 2 |
| Decoder/reference errors, value/input/ownership mismatches, or FX calls inside fast scope | 0 | 0 |
| Total frozen cases retained | 185 | 185 |

For records/projections/indexes, the test calls the actual exported fast decoder.
For compound block operations it calls the actual public serializer or lazy API,
with a test-only overlay of `fastCborSerializer` that returns fast errors directly
instead of falling back. The independent reference run uses FX at those same
wrapper points. Another overlay rejects and counts `Unmarshal`, `UnmarshalFirst`
and `UnmarshalStrict` calls into the FX adapter while the fast decode is active.
All successful fast rows report zero such calls. FX-only block framing setup is
outside this guarded scope, matching the historical lazy-container setup.

Every successful row matches FX and preserves input. Materialized outputs are
compared again after overwriting input; index-prefix results instead check owned
indexes and the exact intentionally borrowed suffix, including nonempty pointer
aliasing and slice capacity. Full serialized block containers retain the historical
framing/caller-contract classification rather than an extra ownership assertion.
The two empty lazy controls remain explicit and are not evidence of decoder speed.
The six removed-projection cases remain explicit incompatibilities; the driver
refuses that classification if the projection type reappears in either source tree.

The instrumentation has a negative check on both revisions: malformed input must
return its fast error without an FX call, and each guarded FX API must reject and
count an intentional attempt. This prevents an ordinary fallback success from
being reported as a compatible fast decode.

From the repository root:

```sh
python3 verification/fastcbor-proof/smoke/run.py
python3 verification/fastcbor-proof/smoke/run.py --check
```

The first command reruns both targets and writes evidence. The second verifies the
saved evidence against both Go source trees, the harness, frozen historical sources
and corpus hashes without rerunning the decoders. It requires the baseline to
remain clean at its frozen commit. This check is separate from `check.sh` so Lean
checking does not require access to the external corpus or baseline worktree.
The proof audit hashes the harness and saved result and reports both case counts.
The frozen universe itself remains unchanged; its original pending-status labels
are historical, while the source-specific smoke evidence lives in the new artifact.

**No timings were collected and no performance gate passed.** The overlays contain
instrumentation and are unsuitable for measurements. This smoke covers the current
planner plus the typed Header reader; it does not validate typed readers that have
not yet been implemented. Rerun it after decoder changes, then use an uninstrumented
paired/interleaved benchmark harness under the frozen protocol for the performance
gate. The 6 incompatibilities and 2 controls must remain visible in that report.

Full regeneration/check passed (2161 build jobs): **1008 audited theorems**,
**339 project hashes** and 28 standard-library hashes. The transitive proof
dependencies are unchanged, with no admissions or native-evaluation axioms. The
full log is `check-frozen-smoke-full.log`. The baseline, original worktree and pinned
trusted semantics are unchanged. The Header proof still requires the separately
approved indexing repair and actual eight-byte helper proof.

### Race and strict pointer checks on the frozen corpus

The full smoke now also passes on both the untouched baseline and current candidate
with **`-race -gcflags=all=-d=checkptr=2`**. Each revision retains the same complete
classification: 177 compatible fast cases, 6 removed-projection cases and 2 controls
with no decoder calls. No data race or strict pointer-check failure was reported.
The negative instrumentation test also passed on both revisions, and every recorded
fast case reports zero FX calls inside its guarded decode scope.

The driver records the exact Go build flags and clears ambient `GOFLAGS` so an
ordinary run cannot silently inherit instrumentation or other compiler flags.
Ordinary and instrumented results use separate artifacts; source/corpus evidence
checks require the requested flags to match the saved record.

```sh
# Repository root: execute both revisions with race and strict checkptr.
python3 verification/fastcbor-proof/smoke/run.py --race --checkptr
# Revalidate the saved instrumented evidence without running the decoders again.
python3 verification/fastcbor-proof/smoke/run.py --race --checkptr --check
# Ordinary evidence is retained and checked independently.
python3 verification/fastcbor-proof/smoke/run.py --check
```

The instrumented artifact is `results/compatibility-smoke-race-checkptr.json`.
The ordinary smoke was rerun with the updated harness and retains its prior case
classification. Both artifacts match the recorded source trees, harness and frozen
historical sources/corpus files. These checks exercise the current implementation;
they do not prove universal race freedom, memory safety or correctness of future
typed readers. No instrumented or ordinary timings were collected.

Full regeneration/check passed (2161 build jobs): **1008 audited theorems** and
**340 project hashes**, with unchanged theorem/axiom dependencies and 28 matching
standard-library hashes. There are no admissions or native-evaluation axioms. The
full log is `check-frozen-race-checkptr-full.log`. Decoder Go code, the baseline and
the pinned semantics remain unchanged. Header's unresolved helper proof, remaining
typed implementation and the performance gate remain open.

### Typed Go map feasibility: resource keys pass; felt keys need array equality

`feasibility/maps/witness.go` contains isolated Go witnesses for all four map
shapes in the checked entry manifest. They import the real `core.Resource`,
`core.ResourceBounds`, `felt.Felt`, `felt.SierraClassHash` and `felt.CasmClassHash`
types. Goose and proofgen translate all four functions. The translation selection
now includes Resource/ResourceBounds and the two named felt aliases; none is
replaced with a handwritten type or trusted implementation. These functions are
not production decoder entries and do not widen the generated decoder.

`ProofCheck/MapFeasibility.lean` proves the actual translated `ResourceEntries`
function: it allocates `map[Resource]ResourceBounds`, inserts two arbitrary
key/value pairs in order, looks up an arbitrary query, and returns exact map
ownership together with the value and presence flag. The contract covers equal
keys (second insert replaces the first), different keys, absent queries and nil
pointer fields in the values. It has no assumed helper or `SafeMapKey` premise.
It needs the same ambient modeled Go semantics and generated type/function
interpretation assumptions as the other proofs. It neither dereferences nor
claims ownership of the felt pointees held inside ResourceBounds.

The other three shapes share an additional **pinned-model incompleteness**:

| Map shape | Evidence and remaining gap |
|---|---|
| `map[Resource]ResourceBounds` | Translation and complete map-operation witness proof pass. The CBOR reader, loop, value ownership and error contract remain future work. |
| `map[felt.Felt]*felt.Felt` | Actual translated witness reaches the first insertion, where `SafeMapKey felt.Felt.ty k1` cannot be discharged. |
| `map[felt.Felt]map[felt.Felt]*felt.Felt` | Translation and Go execution pass. Outer and inner keys use the same blocked felt equality; nested ownership composition is not yet proved. |
| `map[felt.SierraClassHash]felt.CasmClassHash` | Translation and Go execution pass. The named key alias has the same missing array-equality rule; by-value map proof remains open. |

Felt and SierraClassHash ultimately have underlying type `[4]uint64`.
`Perennial/Golang/Defn/Array.lean` has `equals_array`, but that rule only establishes
`CheckComparable` for an array whose element type is comparable. It does **not**
evaluate `GoOp GoEquals` on two array values. The map theory deliberately requires
actual Go key equality, including the self-comparison before insertion. The
pinned comparison scheme supplies scalar, pointer and struct comparison rules,
but no corresponding array-value rule. Thus adding ordinary typed readers alone
does not discharge this obligation. This issue does not affect Header, which
decodes felt limbs without using felts as Go map keys.

The following diagnostics are deliberately outside the audited proof root:

```sh
# From verification/fastcbor-proof, after check.sh has generated/built imports.
# Both commands are EXPECTED TO FAIL on perennial.rev.
"$ELAN_HOME/bin/lake" env lean repros/FeltMap.lean
"$ELAN_HOME/bin/lake" env lean repros/MapKeyEquality.lean
```

The first fails at the actual map insertion's `SafeMapKey` obligation. The second
isolates three missing comparison instances: bare `[4]uint64`, Felt and
SierraClassHash. It avoids confusing the gap with an untranslated named alias or
an absent `GoLocalContext`. No failed proof or admission is imported into
`ProofCheck`, and no key-equality correctness premise is added to a passing theorem.

The public Go witness test runs 216 key/collision/query combinations for each of
four map shapes (864 calls). Keys distinguish every limb separately, zero and
all-one limbs. It checks absent versus present nil values, duplicate replacement,
named felt values and updates through the nested-map alias. Ordinary and
`-race -gcflags=all=-d=checkptr=2` runs pass. This is execution evidence for the
witnesses, not exhaustive decoder testing or a proof of array equality.

No trusted semantics change is made. A future array-equality proposal must follow
the existing type-directed comparison scheme (including element comparisons and
short-circuiting), inspect nested arrays/structs and named/generic array types,
and remain separate from the pending one-line UTF-8 slice-index correction.
The active goal and `.tools/perennial/.agents/skills/fix-semantics/SKILL.md` require
separate approval before either trusted change. Merely assuming `SafeMapKey`, or
changing Juno's key representation to avoid this evidence, is not this result.

The checked manifest still contains 15 roots, 140 shapes, 18 registration entries
and all 185 benchmark cases; the build-selected non-test package list increases
from 154 to 155 because it now includes the witness package. The frozen ordinary
and race/checkptr corpus smoke were rerun for the updated source inventory, with
177 compatible fast cases, six retained incompatible projections and two no-decode
controls on each revision. Production decoder Go and the untouched baseline are
unchanged; no timing gate was run.

Full regeneration/check passes with **1009 audited theorems**, **346 project
hashes**, **28 standard-library hashes** and 2164 build jobs. No new axioms,
admissions or native-evaluation axioms are introduced. Translating Resource and
ResourceBounds removes their six previous opaque type/underlying/zero-value
declarations from the theorem dependency union; other documented generated opaque
declarations remain. The 51 direct validator-premise theorems and two direct
word-helper-premise theorems remain conditional. All recorded hashes match.
The full log is `check-map-feasibility-full.log`.

### Transaction interface feasibility: boxing and type switches pass

`feasibility/interfaces/witness.go` imports the actual `core.Transaction` interface
and its five registered concrete pointer variants: DeployTransaction,
DeployAccountTransaction, InvokeTransaction, DeclareTransaction and
L1HandlerTransaction. Five exported functions box the corresponding pointer as
Transaction. `ClassifyTransaction` uses an ordinary Go type switch to recover each
pointer type and check whether it is nil; it has explicit nil-interface and
default branches. Its numbers 0–6 are local witness labels, **not CBOR wire tags**.
The witness is isolated from production decoder entries.

`ProofCheck/InterfaceFeasibility.lean` contains twelve checked weakest-precondition
contracts for these actual translated functions:

- Five boxing contracts preserve the exact dynamic type and pointer, for any
  pointer including null.
- Five classification contracts cover arbitrary pointers of each recognized
  dynamic type; the returned nil flag is exactly pointer equality with null.
- The nil-interface contract returns the separate nil label.
- The default contract covers every dynamic type distinct from those five,
  without reading its payload or calling an interface method.

All twelve contracts use the pinned model's existing interface conversion,
type-assertion, type-comparison and pointer-comparison rules. There is no new
trusted semantics and no assumed helper-correctness contract. The generated
Transaction representation now comes from its actual method-set interface rather
than an opaque type placeholder. At the witness checkpoint, the concrete transaction payload layouts remained
opaque. The later transaction-reader checkpoint translates their layouts; these
witnesses still neither allocate nor read
a transaction payload, so they establish only pointer publication and recovery.
Future typed payload readers still need translated concrete layouts and their own
independent ownership/correctness contracts. These results do not establish
CBOR tag parsing, the numeric registry mapping, dynamic method invocation, class
interfaces, or complete transaction decoding.

The Go test checks all five non-null pointer round trips, all five typed-nil
pointers, a nil interface, and an external implementation of Transaction with
both non-null and nil pointers. That external implementation's methods panic if
called, so the passing test also exercises the intended no-method-call default
path. Ordinary and race/strict-checkptr tests pass. `check.sh` now runs both map
and interface witness tests before translating and checking the Lean target.

```sh
# Repository root; public witness APIs, no production decoder replacement.
go test -short ./verification/fastcbor-proof/feasibility/...
go test -short -race -gcflags=all=-d=checkptr=2 ./verification/fastcbor-proof/feasibility/...
```

The checked entry manifest retains its 15 roots, 140 shapes, 18 registrations and
185 benchmark cases; its build-selected non-test package list now has 156 entries,
including the new witness package. Ordinary and race/checkptr frozen-corpus smoke
were rerun for the changed source inventory. Each revision retains 177 compatible
fast cases, six incompatible projections and two no-decoder controls, with no FX
fallback in fast scopes. Production Go decoder code, original and baseline
worktrees, pinned semantics, and the frozen benchmark universe remain unchanged.
No performance timing was collected. Header still needs the approved generic
slice-index repair and actual UTF-8 word-helper proof; felt-key map proofs still
need the separate array-equality model work.

Full regeneration/check passes with **1021 audited theorems**, **350 project
hashes**, **28 standard-library hashes** and 2167 build jobs. The twelve interface
witness contracts add no axioms, admissions or native-evaluation dependencies.
Translating Transaction removes its three previous opaque type/underlying/zero
value declarations from the dependency union; other documented generated opaque
declarations remain. The 51 direct validator-premise theorems and two direct
word-helper-premise theorems are unchanged. All recorded hashes match.
The full log is `check-interface-feasibility-full.log`.

### BigInt magnitude: arithmetic foundation and actual word-load dependency

`ProofCheck/BigIntMagnitudeSpec.lean` specifies an arbitrary-length big-endian
magnitude as an unbounded base-256 number. It defines mathematical 64-bit word
grouping and fixed-length carry propagation, and proves their numeric meaning.
These definitions are proof-side functions, **not replacements for the Go code**.
No correctness of the Go packing/carry helper is assumed from those definitions.

The checked arithmetic results cover:

- Every byte list, including empty input and leading zeros: grouping its reversed
  bytes into eight-byte words preserves its unbounded magnitude exactly.
- Removing a full final byte group gives the next low word; a nonempty partial
  final group occupies one word. Group count agrees with the Go `len/8` plus
  remainder increment. Even one additional carry word fits the signed 64-bit
  count for every valid Go input length. This is an arithmetic bound, not an OOM
  or runtime-allocation guarantee.
- The actual partial-word expression `acc << 8 | uint64(byte)` equals
  `256*acc + byte` when the accumulator fits 56 bits; the big-endian prefix bound
  supplies this condition. This connects the bit operations to the byte meaning.
- Increment preserves word count. With the spare high zero word allocated by the
  implementation, increment adds exactly one, even when every preceding word
  overflows. Applied to the grouped bytes, the result is exactly magnitude + 1,
  the magnitude needed for the tag-3 negative result.

`ProofCheck/BigIntMagnitudeWord.lean` also checks the actual standard-library
`BigEndian.Uint64` call used by `bigIntMagnitude`, borrowing an arbitrary valid
input window. Its unsigned return equals the unbounded eight-byte magnitude and
it preserves ownership of the entire fractional input. This composes the existing
word-load proof with the new numeric relation; it is not a proof of the enclosing
allocation or loop.

At the arithmetic-foundation checkpoint, the remaining implementation obligations
were the following. The complete helper milestone below now closes items 1–4
and the helper/output portion of item 5:

1. Establish the fresh, zeroed word allocation, including empty input and the
   optional extra carry word.
2. Prove the full-word loop with the invariant that the original packing is the
   already-written words followed by the packing of the pending byte prefix;
   preserve the whole input and the disjoint output allocation.
3. Prove the partial-word loop using the checked base-256 step and then store its
   single final word.
4. Prove the actual range/carry loop refines `incrementMagnitudeWords`, including
   its early break and complete carry chains.
5. Compose with the existing real `SetBits` normalization and in-place `Neg`
   contracts, establish owned normalized output, then prove the complete public
   `Cursor.BigInt` success and failure relation, including tags, plain negative
   overflow, exact suffixes, diagnostics and cursor behavior.

That arithmetic-foundation checkpoint added 21 results: 20 arithmetic/specification results and
one actual word-load contract. All pre-existing theorem dependencies are
unchanged. Full regeneration/check passes with **1042 audited theorems**, **352
project hashes**, **28 standard-library hashes** and 2169 build jobs. The axiom
union is unchanged, with no admissions or native-evaluation dependencies. The
full log is `check-bigint-magnitude-full.log`.

One new theorem is entirely axiom-free. `audit.py` now recognizes Lean's explicit
“does not depend on any axioms” report as the empty dependency set, while still
requiring an explicit report for every handwritten theorem. Negative checks
confirmed that removing that report, replacing it with `sorryAx`, or replacing it
with `Lean.ofReduceBool` each fails the audit; those checks leave the authoritative
audit artifact unchanged.

The existing public `TestBigIntCarryAndOwnership` and `TestRFC8949AppendixA` tests
passed with `-race -gcflags=all=-d=checkptr=2`. Go decoder code, translation
selection, all Go witness sources and pinned semantics are unchanged. The saved
ordinary and race/checkptr frozen-corpus evidence still matches both source trees
and all corpus/harness hashes; it was revalidated, not rerun for proof-only edits.
Header remains conditional on its documented helper repair, full BigInt and
felt-key maps remain open, and no performance gate has been run.


### Complete production BigInt magnitude helper

`ProofCheck/BigIntMagnitude.lean` proves `wp_bigMagnitude` for the actual translated
`cursor.bigIntMagnitude`, for every owned byte slice and both values of each
`negative` and `increment` flag. The theorem states the exact mathematical result:

```text
(negative ? -1 : 1) * (bigEndianMagnitude(input) + (increment ? 1 : 0))
```

Its postcondition preserves the entire fractional input ownership and supplies a
separately owned `big.Int` and normalized word slice. Empty magnitude has a false
sign; nonempty normalized storage ends in a nonzero word. Leading zero bytes,
empty input, arbitrary-length carry chains, and the extra high carry word are
included. No assumption of a packing, carry, allocation or magnitude-helper
correctness contract appears in the theorem.

The proof follows the actual Go control flow:

- The allocation count is computed using signed machine division/remainder and
  the partial/carry increments; it fits for every valid input slice length.
- The full-word loop writes the low words while shrinking the pending byte
  prefix. Its invariant equates the original packing to the written words
  followed by the packing of that prefix. Interior eight-byte reads restore
  ownership of the entire input, including previously consumed bytes.
- The partial-byte range loop accumulates the pending big-endian prefix without
  overflow and writes its last word.
- The carry range loop refines `incrementMagnitudeWords`, including its early
  break and loop exhaustion. The spare zero word gives the exact +1 meaning.
- Existing contracts for the actual `SetBits` and in-place `Neg` implementations
  establish the canonical, independently owned result. The new pure
  `bigNormalizedPrefix_take` lemma transfers normalization to the retained slice.

This closes the magnitude-helper dependency; it does **not** prove the public
`Cursor.BigInt` CBOR parser. Next, define its independent success/error relation
and compose head/tag/payload parsing, plain integer paths and their overflow
case, exact consumption, diagnostics and cursor rollback with this helper.
The Header UTF-8 helper, felt-key map semantics gap, remaining typed readers and
frozen performance gate remain open. No production Go or trusted semantics were
changed for this milestone.

Full regeneration/check passes with **1045 audited theorems**, **353 matching
project hashes**, **28 matching standard-library hashes** and 2170 build jobs.
All 1042 prior theorem dependency lists and the transitive axiom union are
unchanged; the three new results contain no admissions or native-evaluation
axioms. The full log is `check-bigint-magnitude-implementation-full.log`.

The public `TestBigIntCarryAndOwnership` and `TestRFC8949AppendixA` tests pass
with `-race -gcflags=all=-d=checkptr=2`. Saved ordinary and race/checkptr frozen
corpus evidence was revalidated against the unchanged Go sources and corpus;
the corpus was not rerun and no performance timings were collected. The trusted
Perennial checkout and both preserved original/baseline worktrees remain clean.

### Complete public BigInt decoder

`BigIntComplete.lean` proves `wp_bigInt_complete` for the actual translated
`Cursor.BigInt` on **every byte input at every valid cursor position**. Its
postcondition preserves the entire fractional input ownership and provides either:

- An independently specified mathematical integer, separately owned normalized
  `big.Int` storage, exact consumed length without machine-word wrapping, and the
  exact remaining suffix.
- A specified rejection, a nil integer result, the original cursor restored,
  and the owned error object with its exact offset, diagnostic and arguments.

`BigIntSpec.lean` defines the independent success and rejection relations. Every
input is classified into a supported consumed prefix or a rejection. Successful
prefixes are unique in value, bytes consumed and suffix. All unsigned and negative
integer head widths are covered, including the `-2^64` case; tags 2 and 3 accept
arbitrary-length byte-string magnitudes, empty payloads and leading zero bytes.
This matches the supported Juno integer contract, including non-preferred heads.

`BigIntOwned.lean` and `BigIntValid.lean` compose the actual allocation, SetUint64,
Neg, head, payload and complete magnitude-helper proofs. `BigIntRootFailure.lean`,
`BigIntPayloadFailure.lean` and `BigIntFailure.lean` cover every rejection. A wrong
payload type inside tag 2/3 becomes ErrSyntax at the **outer** start; other payload
errors retain the **inner** offset even though the cursor rolls back to the outer
start. Invalid/truncated heads, EOF, unsupported tags and truncated bodies are
all included, with owned variadic diagnostic arguments where applicable.

`BigIntErrorIs.lean` proves the actual DecodeError.Unwrap method and specializes
the translated `errors.is` loop to the decoder's pointer categories. The existing
pinned `errors.Is` model replaces reflection's comparability check by true; this
proof only uses targets whose dynamic type is `*errors.errorString`, which meets
that restriction. No new trusted model or semantics rule is introduced.

The local cursor initialization predicate now retains each concrete immutable
errorString object. `wp_cursor_new_error_owned` proves that resource from the
actual `errors.New` allocation. Distinct messages and heap agreement establish
that type errors have different pointer identities from truncation, syntax and
unsupported errors. Consequently the complete decoder theorem requires package
initialization and ordinary input/cursor ownership, **no helper-correctness or
category-distinctness premise**. Initialization is still established after the
same modeled runtime CPU startup boundary as before.

This is partial correctness/model safety on the pinned 64-bit target. It does
not prove termination, OOM handling, compiler/runtime implementation correctness
or fxamacker equivalence. It closes a library dependency, not a containing typed
record root. Header remains conditional on its unapproved generic-index repair;
felt-key maps, remaining generated readers and the performance gate remain open.
Production Go, translation selection and pinned trusted semantics are unchanged.

Full regeneration/check passes with **1070 audited theorems**, **361 matching
project hashes**, **28 matching standard-library hashes** and 2178 build jobs.
All 1045 previous theorem dependency lists and the transitive axiom union remain
unchanged. The 25 new results contain no admissions or native-evaluation axioms.
The full log is `check-bigint-public-complete-full.log`; the audit explicitly
records the BigInt scope and pinned `errors.Is` boundary. Its current caveat now
summarizes the actual remaining limits instead of accumulating obsolete statuses.

The entire cursor test package passes with `go test -short -race
-gcflags=all=-d=checkptr=2 ./utils/cbor/cursor` (3.838s). Saved ordinary and
race/checkptr frozen-corpus evidence was revalidated against both unchanged Go
source trees and all harness/corpus hashes; it was not rerun for these proof-only
changes. No performance timings were collected. The trusted Perennial checkout
and preserved original/baseline worktrees remain clean.

### Generated typed Header projections

The generator now emits all five manifested Header projections in
`core/cbor_projections_generated.go`: hash, global state root, transaction count,
nullable timestamp, and hash plus state root. `RegisterTypedCBORReaders` installs
these alongside Header. Other roots retain the planner. The checked entry manifest
now records six registered typed roots, with the projection proofs explicitly
pending; its fifteen-root and 185-case scope is unchanged.

`utils/cbor/cmd/cborgen/projections.go` checks the actual Go projection fields,
Header wire keys, discard-skeleton coverage and the intentional `*uint64`
timestamp representation. Unsupported shapes and tag options stop generation.
Only selected fields enter each static lookup table. A separate internal metadata
package, `core/internal/cborprojections`, keeps the projection tables and their
future initialization contracts separate from the existing Header/GasPrice tables.

Each generated reader uses direct typed stores and `NextWanted`, preserving
first-occurrence wins (including null/undefined), case-insensitive matching,
skipping of unwanted values, full input consumption and nesting behavior. It
also preserves the original projection tag policy: at most one enclosing tag,
with tags 0–3 and chained tags rejected. A nonzero destination is rejected
unchanged; parsing failures clear every selected field. The common public
`fastcbor.Unmarshal` wrapper rejects trailing bytes and clears the result.
The timestamp helper preserves absent/null versus an allocated pointer to zero.

Tests invoke the public fastcbor APIs. Sixty-four generated valid Headers compare
all five projections to FX and their independently selected expected values;
checks include original suffix aliasing, input preservation, independent output
ownership, trailing-byte cleanup and nonzero-destination rejection. Additional
cases cover reordered/case-folded keys, integer unknown keys, duplicate/null
selection, malformed ignored tails, all truncated prefixes and type rejection.
Distinct named projection types retain the planner route for exact value, suffix
and diagnostic parity, including special/unknown/wide/truncated/chained tags.
A 15-second differential fuzz run passed 301,467 executions with two workers.
The targeted Header/projection/skeleton tests passed under race/checkptr=2.

Goose and proofgen accept the actual generated projection types, readers, guards,
cleanup, metadata and both new helpers without trusted-model changes. Translation
is **not a complete projection proof**: independent projection relations, the
general `NextWanted` contract, metadata initialization and API composition remain
open. Header’s existing UTF-8 helper premise is unchanged. No performance result
is claimed for these new implementations; the frozen performance gate remains
mandatory before full-library proofs.

The full regeneration and audit pass with **1070 theorems**, **365 matching project
hashes**, **28 matching standard-library hashes**, and 2184 build jobs. No axioms
were added. Translating the five projection types and their two discard types
replaces 21 opaque type/underlying/zero declarations; 662 theorem axiom lists lose
those dependencies. This is a representation improvement, not a new decoder proof.
The Header validator/word-helper premise counts remain 51 and 2. The log is
`check-header-projections-full.log`.

Both ordinary and race/checkptr=2 frozen-corpus sweeps were **rerun** on the new
implementation and unchanged baseline. Each reports 177 compatible fast cases,
six removed projections and two zero-decoder controls, with no FX fallback,
unchanged input, owned output and exact suffix checks. The reports include the
new source hashes. The generator CLI rejects a nonempty discard marker; the
probe was reverted byte-for-byte. The pinned trusted checkout and preserved
original/baseline worktrees remain unchanged. Performance timings remain pending.

### Generated typed block transaction indexes

`core/cbor_indexes_generated.go` now implements and registers
`DecodeBlockTransactionsIndexesFirst` and supplies the full-input
`DecodeBlockTransactionsIndexes` API. The generator reads the actual integer
CBOR tags and `[]int` field types; it rejects duplicate keys, unsupported tags,
embedded/mixed fields and unmapped types. The emitted loop dispatches directly
on keys 1 and 2, with a seen-bit mask and typed field stores. It needs no runtime
field table, reflection, field offsets or allocation hooks.

The implementation retains the planner contract: the first occurrence of a
known integer key wins, including a null/undefined value; unknown unsigned keys,
negative integer keys and valid UTF-8 text keys are consumed and their values
skipped. Out-of-int64 keys and unsupported key types retain the planner's error
behavior. Unlike projections, an enclosing tag is rejected. Signed integer
arrays preserve nil versus empty, leave null/undefined elements zero, reject
out-of-range values on the pinned 64-bit target, and allocate independent storage.
The unsigned first attempt preserves the planner's common offset path and
its diagnostic behavior for signed/error cases.

Both public typed APIs enforce the zero-destination guard, preserve nonzero
destinations, clear partial results on parsing errors and return an exact borrowed
suffix on success. The full decoder additionally rejects trailing bytes and
clears the destination. The registered fastcbor route invokes the same typed
First implementation. This checkpoint preceded the receipt-family conversion below.

Public-API tests compare 128 random valid index records with FX and an unregistered
named type using the original planner. They cover all 625 combinations of map,
key, array and value head widths; signed int64 boundaries; nil/empty/null/undefined;
reordered, duplicate, text, negative and unknown keys; malformed tails, the
whole-record nesting limit after a decoded array, and every truncated prefix of
a populated record; unchanged input, independent output,
original suffix aliasing, nonzero rejection and cleanup. Race/checkptr=2 passes.
A 15-second differential fuzz run passed 258,162 executions with two workers,
comparing the typed direct API, registered route and planner values, errors and
suffixes. A deliberate duplicate-key source drift makes the generator CLI fail;
the source was restored byte-for-byte afterward.

Goose and proofgen accept the actual integer-key reader, integer-slice and key
helpers and concrete index-record type. This is implementation and translation
evidence, **not a complete index decoder proof**. Independent integer-slice and
index-record relations, complete helper contracts and API composition remain
open. Header's UTF-8 helper and trusted-repair approval remain unchanged, and no
performance gate result is claimed.

At the index checkpoint, the checked manifest recorded **seven registered typed roots** out of the same
fifteen destinations; its 140 shapes, 18 registry entries and 185 frozen cases
remain unchanged. Full regeneration/audit passes with **1070 theorems**, **367
matching project hashes**, **28 matching standard-library hashes** and 2184 build
jobs. No axioms were added. The concrete BlockTransactionsIndexes representation
removes its three opaque type/underlying/zero declarations from 662 theorem
dependency lists; all other dependencies are unchanged. Header's explicit
validator and word-helper premise counts remain 51 and 2. The log is
`check-typed-indexes-full.log`.

Targeted typed-decoder, block-serializer and transaction-hash regression tests
pass with race/checkptr=2 (4.247s); the additional nesting-boundary test also passes.
Both frozen-corpus sweeps were rerun after the final source change. Ordinary and
race/checkptr=2 each report 177 compatible fast cases, six removed projections
and two controls, with no FX fallback. Recorded source and corpus hashes match.
The trusted model and original/baseline worktrees remain unchanged. No performance
measurements or new complete record proof are claimed by this checkpoint.

### Generated typed receipt family

`core/cbor_receipts_generated.go` supplies the value and pointer receipt APIs,
`DecodeTransactionReceipt[Pointer][First]`. The registry also installs the two
receipt projections emitted in `core/cbor_receipt_projections_generated.go`.
At the receipt checkpoint, **eleven typed roots out of fifteen** were registered;
Transaction, transactionHashProjection, StateUpdate and ClassDefinition still used
the planner. The transaction checkpoint below converts the first two.
The inventory retains 140 shapes, 18 registry entries and 185 frozen cases, with
158 build-selected production packages after adding the receipt metadata package.

`utils/cbor/cmd/cborgen/receipts.go` reads the actual Go structs and emits all
receipt child records, including BuiltinInstanceCounter, DataAvailability,
GasConsumed, ExecutionResources, Event and both message types. It uses immutable
literal field tables in a separate `core/internal/cborreceipts` package. The
projection emitter now handles both Header and receipt skeletons, validating the
selected types and full discarded-field coverage before generating code. The
existing Header/GasPrice tables and their initialization proof are unchanged.

Helpers retain the original bool, byte-width FeeUnit, fixed 20-byte address and
felt-value-slice behavior. The generated event/message pointer slice readers use
one pointer array and bounded shared value storage, with individual allocations
after that storage fills, matching the planner's `min(count, remaining/32)` rule.
The execution resource pointer allocates a concrete block containing the parent,
DataAvailability and GasConsumed slots. Nullable children are published only after
successful decoding. No runtime reflection, field offsets or allocation hooks
are used in these readers.

Full records reject duplicate known keys; projections preserve first-wins and
one enclosing tag under the existing policy. The public entries enforce zero
initial destinations, preserve input and exact suffixes, and clear partially
built outputs on errors. Null/undefined, empty/nil slices, nil slice elements,
nullable records and nonzero pointer destinations keep the original behavior.

Public-API tests compare 128 generated valid receipts against FX and named
unregistered types using the planner. They exercise every receipt field and
nested shape, output ownership by overwriting input, suffix aliasing, all truncated
prefixes of a populated record, late failures, FeeUnit boundaries/wide heads,
address lengths, case folding, duplicate keys, projection tags, the whole-record
depth limit and both shared/fallback slice allocations. Targeted typed-reader
checks pass under race/checkptr=2. A 20-second differential fuzz run passed
360,504 executions with two workers, comparing all four roots with the planner
and both public direct receipt APIs with the registered route. A generator CLI
probe rejects changing FeeUnit from byte to uint16; the source was restored
byte-for-byte. The additional tag/guard cases pass under race/checkptr=2 too.

Goose/proofgen translate the concrete receipt layouts and reader implementations,
including the Ethereum address array and generic felt-value-slice call. This is
**not a complete receipt proof**. Independent receipt specifications, nested
allocation and slice contracts, receipt/projection initialization and complete
public API proofs remain open. Header still has its explicit UTF-8 premise and
the trusted indexing repair remains unapplied. No performance result is claimed.

Full regeneration and Lean checking pass with **1070 audited theorems**, **380
matching project hashes**, **28 matching standard-library hashes** and 2188 build
jobs (`check-typed-receipts-full.log`). No theorem gained an axiom. Making the
receipt layouts concrete removes 36 opaque type/underlying/zero declarations
from 662 theorem dependency lists; all remaining dependencies are unchanged.
Header's explicit validator and word-helper premise counts remain 51 and 2.

The full short test suite for `core` and `utils/cbor/...` passes. Both frozen
corpus sweeps were rerun after the final Go source change: each revision reports
177 compatible fast cases, six removed projections and two controls, in ordinary
and race/checkptr=2 modes. Saved source/corpus hashes match. These are correctness
checks, not timing measurements. The original worktree, frozen baseline and
pinned trusted semantics remain unchanged.

### Generated typed transaction family and hash projection

`core/cbor_transactions_generated.go` supplies `DecodeTransactionFirst` and
`DecodeTransaction`, with explicit dispatch to all five concrete transaction
readers. DeployAccount's embedded DeployTransaction fields are flattened into
its record schema and stored through typed field selectors. The generator rejects
ambiguous keys and unsupported embedding; the hash projection must match the
complete union of transaction wire fields, with only its value felt materialized.
Private literal field tables live in `core/internal/cbortransactions`.

The direct transaction API takes `CBORTransactionTags` by value. Registry startup
captures each actual tag from the same encoded type it already uses to configure
the planner, then installs a closure over a distinct-tag profile. There is no
hard-coded numeric tag allocation in the typed decoder and no FX call in its read
path. Registration, assignment of numeric tags, validation of the installed
profile and the routing closure remain outside the formal boundary. A future
transaction theorem must explicitly relate a supplied profile to tag dispatch;
it must not silently assume a fixed mapping or claim registry initialization is
proved. Custom-profile public tests exercise all five variants and all tag widths.

Additional helpers decode TransactionVersion as an allocated four-limb value,
uint32 data-availability modes, plain felt slices and resource-bounds maps. The
map uses ordinary typed Go allocation and assignment. Each map entry starts with
a fresh zero ResourceBounds value, so a duplicate key replaces the entire prior
value even when its new record is partial or null. Null keys are unsupported and
keys outside uint32 are rejected, matching the planner. No runtime map hooks,
reflection, field offsets or manually constructed interface headers remain in
these typed paths.

Full transaction records retain known-key duplicate rejection. The hash projection
retains first-wins selection and its one-tag policy; nil/absent hashes produce a
zero felt. Interface destinations must be nil: a typed nil pointer stored in the
interface is rejected unchanged. Outputs are published after successful payload
decoding, owned independently of the input, and cleared on parse/trailing-data
failure. First decoding preserves the exact original unread suffix.

Public-API tests compare 320 generated valid transactions (64 of each variant)
with FX and an unregistered interface using the original planner, plus the hash
projection. Tests cover all fields, nested maps/slices, nil/empty distinctions,
input/output ownership, all truncated prefixes of populated variants, typed-nil
and populated destination guards, repeated resource keys, map-key/mode widths and
overflow, duplicate known fields, unknown/chained/truncated tags, the whole-record
depth limit, and the explicit tag-profile API. Typed-reader tests pass under
race/checkptr=2. A 20-second differential fuzz run passed 233,061 executions with
two workers. A generator CLI probe rejects changing Resource from uint32 to
uint64; the source was restored byte-for-byte.

At the transaction checkpoint, **thirteen typed roots out of fifteen** were
registered. StateUpdate and ClassDefinition still used the planner; the state
checkpoint below converts StateUpdate. The inventory still has 140 shapes,
18 registry entries and 185 frozen cases, with 159 build-selected production
packages. These transaction implementations and their concrete layouts translate
through Goose/proofgen; their complete independent specifications and decoder
proofs remain open, including resource-map decoding, metadata initialization and
profile/variant correspondence. Header's UTF-8 premise and the unapplied trusted
repair are unchanged. No performance result is claimed.

The full regeneration/check passes with **1070 audited theorems**, **385 matching
project hashes**, **28 matching standard-library hashes**, and 2191 build jobs
(`check-typed-transactions-full.log`). No theorem gained an axiom. Concrete
transaction/variant/version/mode/projection layouts remove 24 opaque declarations
from 662 theorem dependency lists; all other dependencies are unchanged. Header's
explicit validator and word-helper premise counts remain 51 and 2.

The full short `core` and `utils/cbor/...` test suites pass. Both frozen-corpus
sweeps were rerun after the final source change and report 177 compatible fast
cases, six removed projections and two controls on each revision, in ordinary
and race/checkptr=2 modes. The source/corpus hashes match. The original worktree,
baseline and pinned trusted semantics remain unchanged. No timing measurements
or complete transaction decoder proof are claimed by this checkpoint.

### Generated typed state-update reader

`core/cbor_state_generated.go` supplies `DecodeStateUpdateFirst` and
`DecodeStateUpdate`, with source-derived StateUpdate and StateDiff field dispatch
and private literal tables in `core/internal/cborstate`. The registry installs the
value root. There are now **fourteen registered typed roots out of fifteen**;
ClassDefinition is the remaining planner destination. The inventory still records
140 shapes, 18 registry entries and 185 frozen cases, with 160 build-selected
production packages.

`core/cbor_state.go` implements ordinary typed felt-keyed maps, nested storage
diffs and migrated-class maps. The helpers preserve the planner's duplicate-map
policy: later entries replace the entire old value, including nil pointers and
nil/empty inner maps. Known record-field duplicates remain unsupported. Felt
keys are decoded by their four stored limbs; different wire widths and short
arrays can denote the same Go key. Migrated class keys and values use direct
fixed-array pointer conversions rather than the generic unsafe-pointer helper.

Felt-pointer map values share fresh arrays; storage maps share spare capacity
across their inner maps. The first version called `slices.Grow(nil, want)` exactly
as the planner does, but the pinned slices library exposes only its symbol and
has no translated Grow body. The final helper specializes the positive-count
nil-slice case to `append([]felt.Felt(nil), make([]felt.Felt, want)...)` and then
uses the whole returned capacity. This is the same growth expression used by
Go 1.27's Grow at zero initial length/capacity, retaining its capacity-rounding
policy without introducing an unproved library call or changing trusted sources.
The new helper body is translated with the decoder; its complete buffer-ownership
contract remains future work.

Public-API tests compare 128 generated valid updates with FX and an unregistered
named type using the original planner. They cover every map shape, pointer/slice
nilness, explicit empty containers, each limb of the map key, duplicate logical
keys with different byte encodings, whole-inner-map replacement, malformed keys
and values, every truncated prefix of a populated record, late errors, nonzero
destination preservation, original suffixes, read-only input, and failure cleanup.
A varying-inner-map workload exercises repeated shared-buffer growth; mutating one
decoded value leaves every other value unchanged. Depth checks follow nested maps
to verify restoration of the whole-record counter. A generator CLI probe rejects
an unsupported integer-key tag on StateUpdate.NewRoot and restores the source
byte-for-byte.

Targeted race/checkptr=2 tests pass, including a repeat after specializing Grow.
The full short `core` and `utils/cbor/...` suites pass after that change. The final
20-second differential fuzz run passed 253,676 executions with two workers,
comparing the direct API, registered route and original planner. An earlier
411,993-execution run covered the version before Grow specialization.

Goose/proofgen accept the state layouts, map assignments, typed pointer
conversions and explicit allocation helper. This is **not a complete state-update
proof**. Independent state/map specifications, value-buffer ownership, relevant
initialization and complete API contracts remain open. The already documented
array-value equality gap still blocks felt-key map operation proofs; no trusted
repair or assumed map-correctness contract has been added. Header's UTF-8 premise
also remains open. No performance result is claimed.

Full regeneration/check passes with **1070 audited theorems**, **390 matching
project hashes**, **28 matching standard-library hashes**, and 2194 build jobs
(`check-typed-state-full.log`). No theorem gained an axiom. Making StateUpdate and
StateDiff concrete removes their six opaque type/underlying/zero declarations
from 662 theorem dependency lists; all remaining dependencies are unchanged.
Header's explicit validator and word-helper premise counts remain 51 and 2.

Both frozen-corpus sweeps were rerun against the final append/make implementation:
177 compatible fast cases, six removed projections and two controls on each
revision, in ordinary and race/checkptr=2 modes. The saved source/corpus hashes
match. The original worktree, baseline and pinned trusted semantics remain
unchanged. These results do not establish performance or complete state decoding
proofs.

### Generated typed class definitions

The class generator emits both `ClassDefinition` variants, `DeprecatedCairoClass`
and `SierraClass`, with their eight concrete record layouts, CASM children,
entry-point arrays and recursive `SegmentLengths`. The implementation uses
ordinary typed slices and pointers, shared cursor primitives, static field tables
in `core/internal/cborclasses`, and an explicit `CBORClassTags` value. Registry
initialization captures the actual encoded tags in the existing registration
order, checks that the profile is distinct, and installs the typed closure.
Numeric tag assignment and the routing adapter remain outside decoder proof
scope. Arbitrary runtime destinations retain the planner.

All **15 of 15 manifested destinations** now have registered typed readers. The
inventory still records 140 shapes, 18 registry entries and the frozen 185-case
benchmark universe. This completes the planned root conversion, not its complete
specifications, proof obligations or performance gate.

Raw JSON fields are CBOR byte strings: the readers preserve arbitrary bytes,
including invalid JSON and UTF-8, rather than invoking JSON methods. The active
Go 1.27 `encoding/json.RawMessage` alias points to `encoding/json/jsontext.Value`.
Both declarations are translated from the build-selected standard-library sources,
and those sources are included in the audit. Nullable primes use the actual
`Cursor.BigInt` implementation, whose public primitive contract was proved in the
earlier checkpoint. Containing class contracts still need to compose that result.

Importing those JSON packages also exposes their unrelated opaque type declarations
through Goose's package-wide `Assumptions`. They appear in theorem dependencies
even though the CBOR readers do not call the JSON encoder, decoder or parser.
The audit lists them under `typed_classes.json_opaque_type_dependencies`; this
checkpoint does not claim that dependency sets are unchanged or that all JSON
layouts are proved. The actual `RawMessage` alias and `Value` slice are concrete,
and no JSON method-correctness premise or trusted model has been introduced.

Public-API tests compare 256 generated class values with the planner and FX,
including nil/empty arrays and pointers, opaque byte ownership, Unicode strings,
positive/negative/zero big integers and recursive segments. Additional cases
cover every truncation of populated classes, whole-record nesting boundaries,
null value-record elements, case-folded and duplicate fields, alternate tag
widths, unknown/chained tags, custom tag profiles, typed-nil rejection and cleanup
after late nested failures. A generator drift probe rejected an unsupported
`keyasint` tag on `SegmentLengths.Length`; the source was restored byte-for-byte.
The full short core/CBOR tests and targeted race/checkptr=2 tests pass. A two-worker
20-second differential fuzz run completed 212,397 executions without a mismatch.

The class layouts and readers pass Goose/proofgen translation and their Lean
module build. These are implementation and translation results, **not complete
class decoder proofs**. Independent class relations, recursive and slice ownership
contracts, metadata initialization and public API composition remain open. Header's
UTF-8 premise, the unapplied trusted indexing repair and the felt-key map model gap
are unchanged. No performance timings are claimed.

Both frozen-corpus smoke modes pass on baseline and candidate: 177 compatible
fast cases, six removed-projection cases and two zero-decoder controls each.
The ordinary and race/checkptr=2 reports retain all 185 IDs, input/output ownership
checks and negative checks for the FX-fallback instrumentation. These are
source-specific compatibility results; the instrumentation is not used for timing.

`ClassInterfaceFeasibility.lean` adds six checked language-feature contracts for
the actual class witness functions: boxing either concrete pointer, recognizing
either dynamic type including typed nil, and the nil/unknown interface branches.
Go and race/checkptr witness tests agree. These contracts neither access payloads
nor call `Hash`/`SierraVersion`, and do not establish CBOR tag correspondence or
complete class decoding.

Full regeneration, Lean checking and auditing pass with **1076 theorems**, **398
matching project source hashes**, **46 build-selected standard-library source
hashes**, and 2201 build jobs. The log is `check-typed-classes-full.log`. Compared
with the state-update checkpoint, 662 existing theorem dependency lists replace
27 opaque class declarations with the concrete layouts and add 105 opaque JSON
type/underlying/zero declarations from the package-wide assumptions. No added
dependency lies outside those JSON type declarations; the concrete `RawMessage`
and `Value` types are not among them. The six new interface results pass the same
audit. There are no admission or native-evaluation dependencies. Header's explicit
validator and word-helper premise counts remain 51 and 2. The original, baseline
and pinned trusted checkouts remain unchanged.

## Whole-library gates after the Header milestone

1. **Complete feasibility:** close the Header UTF-8 helper after the separately
   approved trusted indexing repair. Resource-map operations and transaction/class
   interface representation/type switches have checked witnesses; complete public
   BigInt decoding is proved. Felt-key map equality still exposes a model gap.
   Recursive SegmentLengths, class payloads, raw JSON and the remaining concrete
   payload shapes need their own contracts and initialization dependencies.
2. **Maintain the generator:** all 15 manifested roots now have typed readers,
   including class definitions, nested types and explicit registry tag profiles.
   Keep static metadata and the checked entry manifest synchronized. Arbitrary
   runtime types remain fallback. Complete projection proofs are still pending.
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
all cases now have source-specific compatibility classifications in
`results/compatibility-smoke.json`. **All 177 compatible decoder cases pass the performance gate**, as described below.

## Frozen performance gate passed

`bench/run.py` and `bench/frozen_bench_test.go.txt` now implement the timing port
for all 185 frozen IDs. The six removed projections remain explicitly excluded;
177 compatible decoder cases and two separately reported zero-decoder controls
are measured. Block framing, case selection and planner warmup stay outside timing.
The wrapper overlay preserves the successful fast path and panics on attempted
fallback. It does not contain compatibility-smoke counters. The unchanged baseline
and candidate both pass timing preflight for every present operation, direct FX
comparisons for records/projections/indexes, and the fallback-rejection check.
Six runner tests check pass/fail/inconclusive decisions, paired drift, preservation
of zero allocations, missing cases and rejection of unplanned looks.

The completed first run used candidate Go sources from `66d35a944` and the frozen baseline.
Artifacts are in `/tmp/juno-cbor-gate-66d35a944-v2`; the runner log is
`/tmp/cbor-gate-measure-v2.log`. Baseline and candidate binaries, overlays and raw
samples are hashed in `report.json`. The prior directory without the `-v2` suffix
was a preparation-only build, before moving block operation selection outside the
closure; it supplied no timing samples or acceptance decision.

The completed run used the shared historical lock, CPU 2, GOMAXPROCS=1 and 300 ms
samples, alternating baseline/candidate order per case and round. All comparable
cases must meet the 3% bound for all three metrics at the prespecified 5/10/20
looks. Student-t bounds are applied to paired log ratios with one-sided alpha/3;
zero allocation baselines remain zero. See [the runner documentation](bench/README.md)
for the assumptions, exclusions and resume rules. A partially collected run is
not a passed performance gate. No whole-library proof advancement is justified by
preflight success or incomplete measurements.

The first scheduled look, after five complete rounds (895 baseline/candidate
pairs), is **inconclusive: 163 decoder cases pass, zero fail, and 14 remain
inconclusive on ns/op**. All 177 decoder cases pass B/op and allocs/op. The 14
remaining cases have timing intervals crossing the 3% boundary, so this is neither
a whole-gate pass nor evidence of a confirmed regression. The unchanged run
continues to ten rounds under the same protocol. Its saved decision was recomputed
from the paired measurements. That check exposed only a presentation-order issue:
JSON key sorting changes classification iteration order, so the artifact checker
now compares decision cases by ID and rejects duplicate IDs. The runner and all
statistical decisions are unchanged. All 13 harness/checker tests pass.

The ten-round look (1790 pairs) is **inconclusive: 176 decoder cases pass, zero
fail, and one remains inconclusive**, `block/block-p100/oneReceipt`. Its paired
geometric mean time ratio is 1.0034716, with bounds 0.9730536–1.0348405 under the
prespecified one-sided alpha/3 procedure. The upper bound exceeds 1.03, so the
estimated +0.35% change cannot yet establish the required limit. Both allocation
ratios for that case are exactly 1; all 177 decoder cases still pass allocation
metrics. Both saved look decisions were recomputed from their paired samples.
The same unchanged run continues to the final planned twenty-round look; no
case has been dropped and no additional acceptance look has been introduced.

### Final twenty-round result

The final scheduled look **passes all 177 compatible decoder cases**, with zero
failed or inconclusive decoder cases. Both zero-decoder controls are retained in
the report, and the six removed bloom projections remain explicitly excluded.
The run collected **3580 baseline/candidate pairs and 7160 raw logs**. Every
comparable case passes all three agreed metrics. The largest ns/op upper ratio
bound is **1.0265001** (`record/transactionInvoke-p100`), below 1.03. Neither
allocation metric increases: their largest upper ratio is 1. The previously
unresolved single-receipt case finishes at ratio 0.9988720 with upper bound
1.0160566. These are empirical results under the recorded protocol, not universal
performance guarantees or a decoder correctness proof.

[The summary](results/performance-gate.json) includes each final case result.
[The evidence archive](results/performance-66d35a944.zip) preserves the complete
report, all raw logs, both source-derived overlays and both preflight logs. The
local original executable hashes were checked against the report; executable
binaries are excluded from the archive. Archive verification rechecks source,
corpus, overlay and raw-log hashes and recomputes all three scheduled decisions;
it explicitly reports that the executable hashes are recorded rather than
revalidated from archived binaries. `check.sh` runs this check before the proof
regeneration/audit, and `bootstrap.sh` installs the pinned statistics dependencies.

```sh
verification/fastcbor-proof/.tools/bench-venv/bin/python \
  verification/fastcbor-proof/bench/check.py --archive \
  verification/fastcbor-proof/results/performance-66d35a944.zip
```

This clears the frozen performance gate for the recorded candidate Go sources.
Changes to those sources or to the benchmark inputs invalidate the check and need
reviewed evidence before carrying the result forward. General Header is still
conditional on the unresolved UTF-8 word helper; the trusted indexing repair is
unapproved and unapplied. Felt-key map equality remains a separate model gap.
Independent specifications, allocation/recursive ownership, metadata initialization
and complete public API proofs for the remaining typed roots are still required.

The full check (`check-performance-gate-full.log`) passes with 2201 build jobs,
1076 audited theorems, 405 matching project/artifact hashes and 46 standard-library
source hashes. All theorem dependency sets are unchanged from the class checkpoint;
no admissions, native-evaluation dependencies or trusted semantics changes were
introduced by the performance tooling. The explicit UTF-8 premise counts remain
51 validator-dependent and two word-helper-dependent results.

## Complete signed-integer primitive

`Int64Spec.lean` defines the signed integer wire contract independently of the Go
reader and fxamacker. It allows both integer signs and every supported head width,
including nonpreferred widths. Both argument ranges are bounded by MaxInt64, so
the decoded interval includes -2^63 and 2^63-1. The checked classification covers
every byte sequence, with head faults/truncation taking precedence over type and
range checks. Successful prefixes have unique values and exact unique suffixes.

`Int64Read.lean` proves the actual translated `Cursor.Int64` branches, using the
already checked head parser. `Int64Complete.lean` composes these into
`wp_int64_complete` for every byte input at any valid cursor offset. Success gives
the specified mathematical signed value and exact nonwrapping consumption.
Failures return zero with an owned diagnostic, preserve the input and restore the
original cursor. The proof covers EOF, malformed/incomplete heads, wrong types,
unsupported tags and overflow; actual cursor initialization supplies error globals.
No unresolved helper contract is required, and no trusted semantics or Go source
was changed. The shared error representation has BigInt-prefixed names, but this
proof executes Int64 directly and does not use BigInt decoding as an oracle.

These fourteen results are a dependency for block-index decoding. Signed-array
proofs follow below, culminating in the complete helper contract. Integer-key
dispatch is now composed conditionally below; the complete BlockTransactionsIndexes
record/API proof still remains. Header's UTF-8 helper and felt-key map model gaps are unchanged.
The existing frozen performance evidence remains applicable to the unchanged Go
sources. The full regeneration and audit (`check-int64-full.log`) pass with
2204 build jobs, 1090 audited theorems, 408 project/artifact hashes and 46
standard-library hashes. All 1076 existing theorem dependency sets are unchanged;
there are no admissions or native-evaluation dependencies. The explicit UTF-8
premise counts remain 51 validator-dependent and two word-helper-dependent
results. Both saved compatibility reports and the archived performance decisions
were rechecked against the current sources.

## Signed-array success and outer failures

`IntsSpec.lean` defines valid index elements and nullable arrays independently of
the implementation. Signed values have their mathematical CBOR meaning; null and
undefined elements mean zero, while a null/undefined array means a nil slice.
`Ints.lean` proves the actual `cborReadInts` loop for arbitrary mixtures and counts,
including both signed endpoints and every supported nonpreferred head width.
The `wp_readInts_valid` contract preserves fractional ownership of the complete
input, returns the exact unread suffix and original depth, and owns the output
backing storage. It distinguishes nil from an allocated empty array. The old
slice header may be arbitrary; its former backing storage remains caller-owned.

The proof follows the actual unsigned-first attempt, including its temporary
error allocation before null and negative elements, and then the signed fallback.
Actual package initialization supplies the error globals. No correctness premise
for that fallback or any other helper is introduced. `IntsReject.lean` separately
checks array-opening errors, impossible counts and exhausted nesting depth: these
preserve the old destination and cursor and return exact owned diagnostics.

At that checkpoint the array contract was incomplete. Element failures after
arbitrary valid prefixes are now composed and classified in the next checkpoint. In particular, an unsigned
value above MaxInt64 consumes its head and returns the bare ErrType sentinel;
a signed-decoder failure rolls back to the element start and returns a DecodeError.
Both retain the entered array depth and leave the old destination unpublished.
The complete index record, integer-key handling and public API proofs remain open.

The Go implementation and pinned semantics are unchanged. Full regeneration,
axiom/source audit and the archived performance check are recorded in
`check-ints-full.log`: 2207 build jobs, 1099 audited theorems, 411 project/artifact
hashes and 46 standard-library hashes. All 1090 previous theorem dependency sets
and the transitive axiom union are unchanged, with no admissions or native
evaluation dependencies. The saved ordinary/race/checkptr compatibility evidence
was also checked against the same Go sources. Header's unresolved UTF-8 helper
is unchanged (51 validator-premise and two word-helper-premise results).

## Complete signed-array helper

`IntElementFailure.lean` classifies every input into an independently specified
valid element or failure. `IntsFailure.lean` proves the production loop's failures
after arbitrary valid prefixes, including mixed positive, negative and null values.
The result preserves the caller's old destination header and backing storage;
the partial new slice is not published. The cursor retains the entered array depth.
Unsigned overflow advances over the integer head and returns the bare ErrType
sentinel. All other element errors use the signed fallback, roll back to the failed
element's start, and return the exact owned diagnostic and argument storage.

`IntsFailureSpec.lean` extends this to every array input, with the actual null,
head, count, depth and element-error precedence. It also bounds consumption,
failed-element offsets and retained depth. `IntsComplete.lean` proves
`wp_readInts_complete` for **every byte input at every valid cursor offset and
supported depth**, combining these results with the checked successful reader.
The contract specifies exact mathematical integers, nil versus allocated empty,
output/error ownership, read-only fractional input, exact successful suffixes,
nonwrapping error positions, retained depth and old-destination preservation.
Actual schema/cursor initialization supplies all error categories; there is no
unresolved helper correctness or initializer WP premise.

This completes the signed-array helper under the documented partial-correctness
model. It does not yet prove the containing BlockTransactionsIndexes record:
the first-duplicate-wins record loop, zero guards, cleanup and public API
composition remain. Integer-key handling is now composed conditionally below. Header's UTF-8 helper and felt-map equality
model gaps are unchanged. No production Go or trusted semantics changed, so the
saved compatibility and performance evidence remains source-applicable.
Full regeneration and audit (`check-ints-complete-full.log`) pass with 2211 build
jobs, 1109 audited theorems, 415 project/artifact hashes and 46 standard-library
hashes. All 1099 existing theorem dependency sets and the transitive axiom union
are unchanged. No admissions or native-evaluation dependencies are present. The
saved ordinary/race/checkptr corpus checks and archived performance decisions
were revalidated against the unchanged sources. The explicit UTF-8 premise counts
remain 51 validator-dependent and two word-helper-dependent results.

## Integer-key dispatch composed conditionally

`IntegerKeySpec.lean` independently classifies every byte input for the actual
`cborReadIntegerKey` helper. Bounded unsigned keys return their numeric value and
select fields; valid negative integer and text keys are consumed as unknown.
Null/undefined are invalid keys. Unsigned overflow consumes the encoded head and
returns the bare ErrType sentinel. Other failures preserve the original cursor
and return an owned DecodeError; the unsigned parser and general key parser have
distinct diagnostic contracts. EOF includes the discarded Peek allocation and the
subsequent returned Key error.

`IntegerKey.lean` proves each actual dispatch branch. `KeyRejected.lean` composes
the existing public Key error proofs for all independently specified failures,
including arbitrary invalid UTF-8. `IntegerKeyComplete.lean` provides
`wp_readIntegerKey_complete` for every input at any valid cursor offset, with
exact values, unknown-key indication, successful suffixes, nonwrapping error
consumption, error ownership and unchanged fractional input. Actual package
initialization supplies the error categories.

**This complete composition remains conditional on Utf8ValidatorContract.** The
helper validates unknown text keys before the record skips their values, so it
inherits the same unresolved word-helper dependency as Header. The direct bounded
unsigned, unsigned-overflow, bounded-negative, malformed-unsigned-head and EOF
branches have no validator premise. No missing proof is replaced by a new axiom
or a new trusted-model change. The index record loop, duplicate policy, ownership,
zero guards, cleanup and public API composition still remain to be proved.

The Go sources, benchmark inputs and pinned trusted semantics are unchanged.
Full regeneration/audit is recorded in `check-integer-key-full.log`, including
revalidation of the archived performance decision. The saved compatibility
reports remain applicable to the unchanged sources. The full check passes with
2215 build jobs, 1124 audited theorems, 419 project/artifact hashes and 46
standard-library hashes. All 1109 prior theorem dependency sets and the transitive
axiom union are unchanged, with no admissions or native-evaluation dependencies.
Six new results explicitly carry the existing validator premise, bringing its
direct count to 57; the two direct word-helper-premise results are unchanged.
Both ordinary and race/checkptr saved corpus reports were rechecked.

## Index-record specification, ownership and guard foundation

`IndexesRecordSpec.lean` now defines the independent complete wire relation for
BlockTransactionsIndexes: nullable records, integer keys 1 and 2, signed nullable
arrays, unknown supported keys, duplicate-value skipping and the depth/count
limits. Every byte input is classified as a successful prefix with an exact
suffix or a specified failure with the last published record, consumed bytes,
diagnostic offset and retained depth. The specification covers all supported
widths and unknown UTF-8 text keys; it is not restricted to canonical encodings.
Consumption and diagnostic offset are bounded independently: unsigned overflow
can consume past its diagnostic start, while Skip can report beyond its restored
cursor. Pair-count and nesting bounds are checked.

`IndexesSeen.lean` connects the production two-bit mask to the independent visited
field set. It proves first-key-wins preservation across arbitrary successful
suffixes, including duplicates whose values need only satisfy Skip. These are
specification and mask-correspondence results; the generated record loop still
needs its own refinement proof.

`IndexesOwnership.lean` relates both concrete Go slices to signed mathematical
values, distinguishing nil from allocated empty slices. It proves field borrowing
and publication, then composes the actual signed-array helper at either generated
field for every specified success and failure. Failed replacements preserve both
previously published arrays even after consuming an element prefix. These helper
compositions require no UTF-8 premise.

`IndexesZero.lean` proves the actual zero guard, standalone cleanup function and
both public APIs' nonzero-destination rejection paths. The guard agrees with the
independent owned value and rejects allocated empty slices. These results do not
yet prove successful public decoding or parse/trailing-data cleanup composition.
The remaining record work is success/failure loop refinement, outcome
uniqueness/separation and the complete zero-destination API contracts. Those
contracts will retain the existing UTF-8 premise until the shared helper is proved.
Header's pending trusted repair and the felt-map model gap are unchanged.

Full regeneration and audit are recorded in
`check-indexes-record-foundation-full.log`: 2219 build jobs, 1151 audited theorems,
423 project/artifact hashes and 46 standard-library source hashes. The 27 new
results introduce no decoder-helper premises. All 1124 previous theorem dependency
sets and the transitive axiom union are unchanged; there are no admissions or
native-evaluation dependencies. Direct UTF-8 premise counts remain 57 validator
and two word-helper results. Production Go and trusted semantics are unchanged.
The full check revalidates both saved compatibility reports and all archived
performance decisions against the unchanged sources.

## Index-record successful loop and public API composition

`IndexesMapValid.lean` now proves the actual generated
`cborReadBlockTransactionsIndexes` loop against every successful pair sequence in
`IndexesPairsValid`. Its invariant connects remaining pairs, the actual seen-bit
mask, both owned arrays, nonwrapping cursor progress and the exact unread suffix.
The proof covers both selected numeric keys, every supported key width, duplicate
values and unknown unsigned/negative/text keys, including the generated `continue`
path. The record can start with any owned value; first occurrences replace their
selected field, while missing fields and skipped duplicates preserve it.

`IndexesValid.lean` adds null/undefined preservation and composes every successful
`IndexesRecordValid` input at any supported cursor offset and depth. It also proves
that cleanup restores the public zero ownership relation after any partial value.
`IndexesValidAPI.lean` composes successful decoding through both production public
entries: First returns exactly the original unread input slice; full decoding
accepts an empty suffix, and rejects a nonempty suffix after clearing both fields,
with an owned diagnostic containing the exact unread count. Input ownership and
nil/allocated-empty distinctions are preserved throughout.

These are **conditional successful-decoding contracts**, not an all-input API
milestone. All successful loop/record/API results explicitly carry the existing
`Utf8ValidatorContract`, because unknown text keys are validated. The remaining
index work is the generated failure loop, including errors after arbitrary valid
prefixes and partially published arrays; outcome uniqueness and success/failure separation;
and parse-error cleanup/API composition. No decoder-specific premise has been
added beyond the existing validator contract. Header's trusted-model repair and
the felt-map equality gap remain unchanged. Production Go, pinned semantics and
the performance candidate sources are unchanged.

Full regeneration and audit pass in `check-indexes-valid-api-full.log`: 2222 build
jobs, 1159 audited theorems, 426 project/artifact hashes and 46 standard-library
source hashes. Eight results were added; all 1151 prior theorem dependency sets
and the transitive axiom union are unchanged. The audit contains no admissions or
native-evaluation dependencies. Five new compositions explicitly carry the
validator premise, bringing its direct count to 62; the two word-helper-premise
results are unchanged. Saved ordinary/race/checkptr compatibility reports and the
archived 177-case passing performance decision were revalidated by the full check.

## Index-record failures and all-input API composition

`IndexesMapRejected.lean` proves every specified failure through the generated
record loop, after arbitrary valid prefixes and partially published arrays. The
invariant retains exact remaining pairs, seen fields, both owned arrays, cursor
consumption, diagnostic position and depth. Integer-key overflow consumes its
head and returns bare ErrType; other key failures roll back. Selected-array
failures retain their specified progress and private allocation behavior without
publishing the failed replacement. Unknown/duplicate-value Skip failures restore
the value-start cursor while retaining their exact diagnostic offset.

`IndexesError.lean` keeps bare sentinels and allocated errors distinct and derives
Skip/Felt error-category ownership from the existing initialized categories.
`IndexesOpen.lean` proves all opening errors, including invalid/missing/wrong-type
heads, count-before-depth precedence and original-record preservation.
`IndexesRejected.lean` composes the complete rejection relation; its bounds track
consumption and diagnostic offsets independently. `IndexesRejectedAPI.lean` proves
both actual public APIs clear every partial result and preserve the precise parse
error; First returns a nil suffix on failure. No parse error becomes a trailing
error.

`IndexesAPISpec.lean` defines independent First/full outcome relations, including
nonzero guard preservation, all-input classification, exact success suffixes,
parse/trailing cleanup and bounded diagnostics. `IndexesAPIComplete.lean` proves
`wp_decodeIndexesFirst_complete` and `wp_decodeIndexes_complete` for **every byte
input and every owned initial record** against those relations. Relevant actual
schema/cursor and cborerr initialization supplies all error categories; callers
supply no error-global or initialization WP assumptions. The same explicit
`Utf8ValidatorContract` remains required for text keys. These are conditional
partial-correctness/model-safety results, not an unconditional record milestone.

The independent record/API outcome uniqueness and success/rejection separation
proofs still remain. The shared UTF-8 word helper and unapproved trusted correction
are unchanged, as is the felt-map model gap. Production Go, baseline worktrees,
benchmark inputs and pinned semantics are unchanged.

Full regeneration and audit pass in `check-indexes-api-complete-full.log`: 2229
build jobs, 1181 audited theorems, 433 project/artifact hashes and 46 standard-library
source hashes. All 1159 previous theorem dependency sets and the transitive axiom
union are unchanged. The 22 added results introduce no new helper-contract kind;
six compositions explicitly retain the validator premise, bringing its direct
count to 68, with two direct word-helper results unchanged. The audit contains no
admissions or native-evaluation dependencies. Both saved compatibility reports
and the archived passing performance decision were revalidated by the full check.

## Index specification has exactly one outcome

`IntegerKeyDeterminism.lean` checks unique keys and byte boundaries, success/error
separation and exact rejected consumption/diagnostics. `Int64Determinism.lean`
and `IntElementDeterminism.lean` do the same for signed integers and nullable
array elements, including the consumed unsigned-overflow versus rolled-back
signed-fallback distinction.

`IntsDeterminism.lean`, `IntsDisjoint.lean` and `IntsFailureUnique.lean` lift these
results through arbitrary valid array prefixes. Successful arrays determine
exactly one value and suffix. Rejections determine retained consumption,
diagnostic offset, entered depth and reason. Success cannot also reject on
representable input. The input-length bound excludes natural counts wrapping
when the rejection relation encodes them as 64-bit words; it is already implied
by the production slice ownership preconditions.

`IndexesDeterminism.lean`, `IndexesDisjoint.lean` and
`IndexesFailureUnique.lean` compose the results through key dispatch, first-wins
duplicates, ignored values, record opening and arbitrary partial records.
`IndexesOutcomeUnique.lean` proves that the independent First and full API
specifications each assign **exactly one outcome to every representable input
and initial destination**. That outcome includes values and exact suffixes on
success, or the preserved/cleared destination and complete diagnostic on failure,
including trailing-data rejection.

These are independent specification proofs with no UTF-8 implementation premise.
They validate the relations used by the previously checked production API proofs;
they do not discharge those proofs' explicit validator premise or establish
execution termination. The Header word-helper repair, felt-map model gap and
remaining record implementation proofs are unchanged. Production Go and the
trusted semantics are unchanged.

Full regeneration and audit pass in `check-indexes-outcome-unique-full.log`:
2239 build jobs, **1217 audited theorems**, 443 project/artifact hashes and 46
standard-library source hashes. The 36 new results introduce no implementation
helper premises. All 1181 previous theorem dependency sets, the transitive axiom
union, standard-library hashes and unresolved-premise inventory are unchanged
(68 direct validator-premise results and two word-helper results). Every recorded
project/artifact hash was independently rechecked. The full check also revalidated
both saved compatibility reports and the archived passing performance decision.

## Projection scan specification and production branches

`WantedSpec.lean` specifies the shared `Cursor.NextWanted` scan independently of
its machine masks and lookup implementation. Folded schema names identify natural
field indices; the seen-field list makes first occurrence win. A selected key
leaves its value unread. Unknown and duplicate keys require a complete valid Skip
before the remaining-pair count decrements. A rejected key consumes nothing; a
rejected Skip retains the cursor at the value start and may diagnose a deeper
byte. Both failures retain the current pair count and leave the container open.
Successful exhaustion closes the container in the production state transition.

`WantedDeterminism.lean` and `WantedFailureUnique.lean` prove unique values,
selected pair counts, suffixes, rejected consumption/offsets/remaining counts and
reasons, plus success/rejection separation. Classification and uniqueness give
**exactly one independent scan outcome for every byte input**, for distinct folded
schema names and supported depth. The specification imposes no assumption about
UTF-8 implementation correctness and does not prove execution termination.

`WantedBoundary.lean` proves the actual translated `NextWanted` for exhausted
closed state, open-container closure, and every immediate Key rejection. Closure
restores one enclosing depth; key failure preserves the cursor, current pair
count and open state without accessing metadata. `WantedSelected.lean` proves
the actual fresh-field branch for every supported text-head width and folded
spelling. It consumes exactly the key, sets one seen bit, decrements one pair,
preserves the search hint/depth and leaves the value unread. Metadata and the
whole input retain fractional ownership. Key rejection and text selection retain
the explicit UTF-8 validator premise; the two exhaustion branches do not.

This is not yet a complete production scan or projection proof. Remaining work
is to compose unknown/duplicate/all-seen Skip paths and arbitrary scan prefixes,
connect masks to independent seen-field lists, establish actual projection
metadata initialization, prove projection-tag/pointer-scalar helpers, and compose
the generated records and APIs. Header's pending trusted repair and the felt-map
model gap are unchanged. Production Go, benchmark inputs and pinned semantics
are unchanged.

Full regeneration and audit pass in `check-wanted-spec-selected-full.log`:
2244 build jobs, **1232 audited theorems**, 448 project/artifact hashes and 46
standard-library source hashes. The 15 new results preserve all 1217 previous
theorem dependency sets and the transitive axiom union. The two new production
text-related contracts explicitly retain `Utf8ValidatorContract`, bringing its
direct count to 70; the two word-helper-premise results are unchanged. No new
premise kind, admission or native-evaluation dependency was introduced. Every
recorded project/artifact hash and the unchanged standard-library hashes were
independently checked. Both saved compatibility reports and the archived passing
performance decision were revalidated by the full check.

## Complete successful projection scans

`WantedSeen.lean` connects the independent seen-field list to the production
64-bit mask. Kernel-checked finite bit calculations cover all 64 possible field
positions and every schema size through 64, including the shift-by-64 all-mask
case. The results prove initialization, exact membership after marking, exclusion
of the all-seen shortcut for a fresh field, and rejection of the fresh-bit test
for unknown/duplicate keys. The actual name matcher determines the independent
schema index under distinct folded names. No native-evaluation axiom is used.

`WantedSkipRejected.lean` proves immediate Skip failure after every supported
integer key and every ignored text-key path: unknown names, duplicates and the
all-fields-seen shortcut. The cursor retains exactly the consumed key bytes;
the current pair count, seen bits, search hint and container depth are unchanged.
The original Skip diagnostic retains its independently bounded absolute offset,
which can be beyond the cursor restored by Skip. Input and metadata ownership
are preserved. The integer branch has no validator premise; the text branch
explicitly retains the existing validator contract.

`WantedValidImpl.lean` proves `wp_nextWanted_valid` through the **complete actual
successful NextWanted loop** for every independent `WantedValid` scan, arbitrary
supported field names, every valid key width/spelling, and any number of skipped
unknown/duplicate pairs. Its invariant tracks remaining pairs, exact remaining
bytes and cursor progress against the original successful prefix. Selection
marks one bit and leaves its value unread; full exhaustion closes an open
container once. The theorem preserves the complete fractional input and metadata
and establishes the exact unread suffix. The original pair count must fit the
pinned signed 64-bit representation, matching the production Struct invariant.
Only the existing UTF-8 validator premise remains for key decoding; no assumed
Key, match or Skip correctness contract was introduced.

The remaining scan obligation is failure propagation after arbitrary successful
skipped prefixes, followed by the unified all-input contract. Actual projection
metadata initialization, projection-tag and pointer-scalar helpers, and complete
record/public API composition remain. No unconditional Header or projection
milestone is claimed. Production Go, benchmark inputs and pinned semantics are
unchanged.

Full regeneration and audit pass in `check-wanted-valid-full.log`: 2247 build
jobs, **1243 audited theorems**, 451 project/artifact hashes and 46 standard-library
source hashes. All 1232 previous theorem dependency sets, the transitive axiom
union and standard-library hashes are unchanged. The 11 new results add two
explicit validator-dependent compositions, bringing the direct validator count
to 72; the two word-helper-premise results are unchanged. No new premise kind,
admission or native-evaluation dependency was introduced. Independent checks
verified every recorded project/artifact hash and the exact premise additions.
The full check revalidated both saved compatibility reports and the archived
passing performance decision.

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


### Complete projection scan implementation contract

`WantedRejectedImpl.lean` proves `wp_nextWanted_rejected` through the actual
`NextWanted` loop after arbitrary valid skipped prefixes. The cases include
integer keys, unknown text keys, duplicate selected names and the all-fields-seen
shortcut. A failing key leaves its bytes unread; a failing ignored value retains
the cursor at the value start. Both retain the failing pair count and the original
container-open state. The proof returns the actual owned diagnostic, preserving
input and metadata, with separate exact nonwrapping consumption and error offsets.

`WantedComplete.lean` combines successful and rejected scans into
`wp_nextWanted_complete`, which accepts **every byte input at a valid cursor**
under explicit schema/seen-mask correspondence and supported depth/count bounds.
Actual cursor initialization supplies the error-category globals. The result
contains the independent `WantedSpec` outcome and `WantedScanOwned`: concrete
cursor/struct ownership, the selected index or Done, the untouched selected value
and exact suffix on success, or the retained state and diagnostic on failure.
Exhaustion closes an open container exactly once. This composes the checked
implementation branches; it does not assume a valid input or a chosen scan result.

The only helper correctness premise remains `Utf8ValidatorContract`. The pure
specification's already-checked uniqueness theorem applies to these outcomes.
This is a complete conditional **scan helper**, not yet a projection record/API
proof or a termination proof. Projection metadata initialization, tag handling,
nullable scalar helpers, record specifications and public API composition remain
open. Header's unapplied trusted indexing correction and the felt-map equality
model gap are unchanged. Production Go and trusted semantics are unchanged.

Full regeneration and audit pass in `check-wanted-complete-full.log`: 2249
build jobs, 1245 audited theorems, 453 project/artifact hashes and 46 directly
translated standard-library hashes. Both added implementation results explicitly
require the existing validator premise (74 direct validator-premise results;
2 word-helper-premise results unchanged). Every previous theorem's dependency
set and the overall axiom union are unchanged, with no admission or native
execution axioms. Independent checks verified all recorded project hashes and
the exact premise additions. Saved compatibility, race/checkptr and archived
performance evidence was revalidated against the unchanged Go sources.


### Projection metadata initialization and caller contracts

`ProjectionState.lean` lists the five Header and two receipt projection schemas
independently of the generated factory expressions. Checked finite facts establish
that each schema has at most 64 distinct folded field names and that every cached
wire key, word, mask, field index and selection bit agrees with those names.
`ProjectionFields.lean` checks all seven actual generated allocation factories
and derives persistent ownership of their exact backing slices and masks.

`ProjectionInitialize.lean` checks the **complete actual cborprojections package
initializer**: all seven globals, all seven factory calls, their stores and the
existing proved cursor initialization dependency. It publishes immutable table
ownership and exposes it through the package initialization predicate. The runtime
CPU-startup precondition is the same explicit boundary as Header initialization;
no projection initializer correctness premise is supplied by callers.

`ProjectionOpen.lean` checks all seven actual opening wrappers on arbitrary bytes,
deriving the appropriate published table and canonical errors from initialization.
The returned `StructOpenOutcome` covers null/undefined, bounded maps and every
specified opening error, with exact cursor/depth and diagnostic ownership. This
opening step has no UTF-8 premise and excludes the preceding projection tag helper.

`ProjectionScan.lean` specializes the all-input `NextWanted` contract to any of
those initialized tables. Agreement of immutable global pointers ties the current
`Struct` to the correct table. Its names, mask, size and folded-name uniqueness
come from the checked metadata rather than caller assumptions. It retains the
existing explicit UTF-8 validator premise and input/state ownership contract.

This closes projection table initialization and the opening/scanning connection,
not complete projection record/API correctness. Remaining work includes the tag
policy, nullable timestamp scalar helper, independent record specifications,
field-value loops and public API composition. The Header UTF-8 word helper and
felt-key map model gap remain unchanged. No production Go or trusted semantics
was modified.

Full regeneration and audit pass in `check-projection-initialize-full.log`: 2254
build jobs, 1268 audited theorems, 458 project/artifact hashes and 46 directly
translated standard-library hashes. The 23 new results preserve every previous
theorem's dependency set and the overall axiom union, with no admissions or native
evaluation axioms. Only `wp_nextWanted_projection` adds a direct occurrence of the
existing validator premise (75 direct validator results; 2 word-helper results
unchanged). Independent checks verified the exact additions and all recorded
project hashes. Saved compatibility, race/checkptr and archived performance
evidence was revalidated against the unchanged Go sources.


### Complete projection tag policy

`ProjectionTagSpec.lean` defines the independent prelude relation for every byte
input: leave untagged input alone; consume one allowed tag; reject a malformed tag
head without consumption; or consume the first head and reject a reserved tag
number (0–3) or a tag chain. This prelude deliberately defers EOF and validation
of the content item to the record reader. In particular, an allowed tag followed
by EOF succeeds here, while a reserved tag followed by EOF is unsupported.
`ProjectionTagUnique.lean` proves that consumption, unread suffix and status are
unique, including the exact malformed-head reason; success and rejection cannot
overlap. These are specification results, not an execution-termination claim.

`ProjectionTagRead.lean` proves actual Tag decoding through its shared expectation
implementation for every supported width and all specified failures. It also
checks the actual untagged/EOF and malformed-tag prelude branches.
`ProjectionTagChecked.lean` handles both post-tag Peek outcomes, reserved-number
checks and tag-chain rejection. Failed Peek allocations are discarded just as in
the production helper; the returned unsupported error is the bare canonical
sentinel, while malformed heads return owned structured diagnostics.

`ProjectionTagComplete.lean` composes the actual `cborReadProjectionTag` contract
for **every byte input at a valid cursor**. It derives canonical error categories
from actual cursor initialization and returns the independent relation with exact
nonwrapping consumption, exact suffix, unchanged depth, preserved fractional input
ownership and concrete error ownership. There is **no UTF-8 or other unproved
helper-correctness premise** in this tag-prelude theorem.

Complete projection record/API proofs and the nullable timestamp scalar helper
remain open. The existing conditional scan contract still depends on UTF-8;
Header's unapproved trusted-indexing repair and the felt-map equality gap are
unchanged. No production Go or trusted semantics changed in this checkpoint.

Full regeneration and audit pass in `check-projection-tag-full.log`: 2259 build
jobs, 1279 audited theorems, 463 project/artifact hashes and 46 directly translated
standard-library hashes. The 11 new results add no helper premises or axioms.
Independent checks verified every previous theorem's dependency set, the unchanged
overall axiom union, exact theorem additions and all project hashes. The direct
UTF-8 premise counts remain 75 validator results and 2 word-helper results. Saved
compatibility, race/checkptr and archived passing performance evidence was
revalidated against the unchanged Go sources.
