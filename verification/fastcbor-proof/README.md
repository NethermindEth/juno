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
felt depth rejection, later-limb errors for every array-head width, ignored-tail atomic/EOF errors,
and malformed-head propagation through unsigned/felt/Skip readers,
general recursive Skip success and atomic rejection/EOF contracts,
definite payload success/truncation and public byte/string propagation contracts,
general ASCII folding and two-pass name matching, field-bit marking and duplicate
rejection, integer-key classification/overflow, key type/tag rejection and text-body
truncation through public Key and alternate field lookup, the complete batched ASCII
scan and short-ASCII-key field matching/duplicate handling, byte-prefix equivalence
for the word cache, production cached-hit and general field-EOF contracts, general
table search for arbitrary input lengths with the Header/GasPrice cache shape,
general public table-hit and ASCII field-reading contracts, exact search-index
progress, public integer-key classification/range rejection, key type/tag rejection
and text-body truncation, malformed/incomplete key-head propagation, and bounded public Header examples. The reproducible audit records the current lemma count.
The count is not a coverage percentage. The complete Header theorem is absent.

| Order | Work | Exit criterion | Current status |
| --- | --- | --- | --- |
| 1 | Formalize the complete Header application contract | Independent definitions of accepted encodings, decoded values, rejection cases and ownership, covering all 15 fields and GasPrice; the checklist above is represented formally | Head-argument, recursive Skip and felt value relations exist; full Header relation pending |
| 2 | Finish primitive error contracts | Reserved/invalid heads, wrong types, truncation, excessive counts and depth have the specified result, cursor state and error information | Head classification, scalar type/tag and extended-head truncation failures, and container count/depth failures proved; general EOF and propagation of all head errors through remaining callers pending |
| 3 | General felt and UTF-8 contracts | Fast and slow felt paths agree on limb contents, consumption and failure behavior; text validation and conversion have proved contracts | General felt success (null/undefined, short arrays and arbitrary valid extras) and depth rejection proved through the typed reader; payload extraction and truncation propagation proved; later-limb type/tag/truncation/EOF errors proved for all array-header widths, including both inline-four shortcut rejection paths; ignored-tail atomic/EOF errors proved after arbitrary valid skipped contents; malformed outer/limb/ignored heads also proved; outer type/tag/truncation/EOF and incomplete ignored heads now proved; recursive Skip failures, rejection/converse completeness and UTF-8 pending (generic slice-index fix proposed below) |
| 4 | General field lookup and Skip | Every supported field order, width, case variant, unknown key/value, duplicate and nested skip is covered, including rollback/error behavior | Static table factories, general two-pass name matching, folded-name uniqueness and duplicate marking proved; integer keys, range/type/tag errors and text-body truncation compose through Key and alternate field lookup; short ASCII keys compose through public Key, matching and marking with all three field outcomes; cached comparisons, the production cached-hit branch and general EOF are proved; short-buffer search is proved for arbitrary ordered tables; word-based search is proved for the Header/GasPrice cached short-key shape; all public encoded-table hits and ASCII misses compose through actual field reading for arbitrary input lengths; a unified public ASCII contract covers field identity, new/duplicate/unknown results, exact suffix and search-index progress, with actual Header/GasPrice field-name checks; public integer keys, integer range/type/tag errors and text-body truncation now compose through the cache/search wrapper; all malformed/incomplete key-head errors now compose through public Key and NextField, with original offsets and owned diagnostics; wider/non-ASCII keys remain pending. General recursive Skip success, atomic rejection/EOF and malformed/incomplete heads proved; arbitrary supported tag prefixes propagate malformed/incomplete/atomic failures, tagged EOF and chained-tag depth errors through public Skip; array/map recursive rejection remains pending |
| 5 | Nested outputs and GasPrice | Arbitrary signature lengths and contents, nil/empty distinctions, disjoint owned elements in shared allocations, and partial-construction errors are covered | Null-field behavior proved; general non-null outputs pending |
| 6 | Initialization and immutable table sharing | Relevant actual initialization establishes the table contents and shareable ownership needed by independent decoder calls | Factories proved; matcher and duplicate marking accept fractional metadata; initialization wiring and shared ownership establishment pending |
| 7 | Header loop and public APIs | Compose the preceding contracts into general theorems for cborReadHeader, DecodeHeaderFirst and DecodeHeader, including arbitrary supported records and all specified rejection cases | Restricted null/empty/Number examples and one late-error family proved |
| 8 | Reproduction and validation | Regenerate from the current Go source, Lean-check the full dependency chain, audit axioms/assumptions, and pass the applicable Go regression/property/corpus checks | Harness works; final complete-Header run pending |

This is dependency order, not eight isolated phases: nested GasPrice uses field
lookup, and successful felt decoding now composes the recursive Skip theorem.
The next primitive work is array/map Skip rejection and complete felt rejection/converse, alongside UTF-8;
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
  Malformed and incomplete heads are now covered below; recursive rejection remains unproved.
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
depth and old output pointer. Recursive Skip rejection and complete felt
rejection/converse remain separate obligations.

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
scratch state. They do not yet prove recursive array/map rejection or a complete
arbitrary-input Skip classification/converse.

`TestSkipTaggedErrors` checks mixed head widths and 0–34 tags at enclosing depths
0, 1, 15, 31 and 32, ending in EOF, malformed/incomplete heads and atomic failures.
It checks precedence when a tagged child would exceed depth before its head is
read, exact inner offsets, repeated-read rollback and unchanged input.
`TestTypedHeaderTaggedErrorsClearPartialResult` checks both Header APIs after a
Number field, with tagged failures in an unknown field or an ignored felt value;
the result clears, First returns nil and the inner offset survives.

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
