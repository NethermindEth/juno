#!/usr/bin/env python3
"""Record the exact pilot inputs and reject admitted pilot theorems.

Generated modules can contain unused declarations with sorry; this checks each
exported pilot theorem's transitive axiom list, not just build success. Native
evaluation axioms are also rejected. Opaque
Goose type axioms are reported, not silently treated as proved definitions.
"""
import hashlib
import json
import re
import subprocess
import sys
import zipfile
from pathlib import Path

here = Path(__file__).resolve().parent
repo = here.parent.parent
log = Path(sys.argv[1]).read_text()
proofs = sorted((here / "ProofCheck").glob("*.lean"))
expected = {name for proof in proofs for name in re.findall(r"^theorem (\w+)", proof.read_text(), re.M)}
found = {}
for name, body in re.findall(r"'Perennial\.github_com\.NethermindEth\.juno\.core\.(\w+)' depends on axioms: \[(.*?)\]", log, re.S):
    found[name] = [x.strip() for x in body.split(",") if x.strip()]
for name in re.findall(r"'Perennial\.github_com\.NethermindEth\.juno\.core\.(\w+)' does not depend on any axioms", log):
    found[name] = []
if set(found) != expected:
    sys.exit(f"missing/unexpected axiom reports: {set(found) ^ expected}")
if any("sorryAx" in ax for xs in found.values() for ax in xs):
    sys.exit("a pilot theorem depends on sorryAx")
if any("._native." in ax or ax in {"Lean.ofReduceBool", "Lean.trustCompiler"}
       for xs in found.values() for ax in xs):
    sys.exit("a pilot theorem depends on a native evaluation axiom")
if any(re.search(r"^\s*(axiom|opaque)\b|\b(sorry|admit)\b", proof.read_text(), re.M) for proof in proofs):
    sys.exit("a handwritten proof contains an opaque declaration or admission")
rev = subprocess.check_output(["git", "-C", str(here / ".tools/perennial"), "rev-parse", "HEAD"], text=True).strip()
if rev != (here / "perennial.rev").read_text().strip():
    sys.exit("Perennial differs from perennial.rev")
dirty = subprocess.check_output(["git", "-C", str(here / ".tools/perennial"), "status", "--porcelain", "--untracked-files=no"], text=True)
if dirty:
    sys.exit("the pinned Perennial checkout has tracked modifications")
inputs = [repo / "go.mod", repo / "go.sum", *proofs, here / "ProofCheck.lean", here / "lean-toolchain", here / "lake-manifest.json", here / "lakefile.toml", here / "check.sh", here / "bootstrap.sh", here / "audit.py"]
for rel in ["core", "core/internal/cborschema", "core/internal/cborprojections", "core/internal/cborreceipts", "core/internal/cbortransactions", "core/internal/cborstate", "core/internal/cborclasses", "l1/eth", "core/felt", "utils/cbor/cursor", "utils/cbor/cborerr", "utils/cbor/fastcbor", "utils/cbor/registry", "utils/cbor/cmd/cborgen"]:
    inputs += [p for p in (repo / rel).glob("*.go") if not p.name.endswith("_test.go")]
inputs += list((here / "config").rglob("*.toml"))
inputs += [here / "cmd/entrymanifest/main.go", here / "results/entry-manifest.json"]
entry_manifest = json.loads((here / "results/entry-manifest.json").read_text())
inputs += [here / "smoke/run.py", here / "smoke/frozen_test.go.txt", here / "results/compatibility-smoke.json"]
compatibility_smoke = json.loads((here / "results/compatibility-smoke.json").read_text())
inputs += [here / "results/compatibility-smoke-race-checkptr.json"]
race_checkptr_smoke = json.loads((here / "results/compatibility-smoke-race-checkptr.json").read_text())
inputs += list((here / "bench").glob("*.py"))
inputs += [here / "bench/frozen_bench_test.go.txt", here / "results/performance-gate.json", here / "results/performance-66d35a944.zip"]
performance_gate = json.loads((here / "results/performance-gate.json").read_text())
if performance_gate["archive_sha256"] != hashlib.sha256((here / "results" / performance_gate["archive"]).read_bytes()).hexdigest():
    sys.exit("performance archive hash differs from its summary")
with zipfile.ZipFile(here / "results" / performance_gate["archive"]) as archive:
    measured = json.loads(archive.read("report.json"))
if (performance_gate["status"] != measured["status"]
    or performance_gate["rounds"] != measured["looks"][-1]["rounds"]
    or performance_gate["pairs"] != len(measured["samples"])
    or performance_gate["raw_logs"] != 2 * len(measured["samples"])
    or performance_gate["cases"] != measured["looks"][-1]["cases"]
    or performance_gate["candidate_go_revision"] != measured["candidate"]["revision"]
    or performance_gate["baseline"] != measured["baseline"]["revision"]):
    sys.exit("performance summary differs from its measured report")
inputs += list((here / "feasibility/maps").glob("*.go"))
inputs += list((here / "feasibility/interfaces").glob("*.go"))
inputs += [here / "repros/FeltMap.lean", here / "repros/MapKeyEquality.lean"]
# Record the build-selected Go sources for standard-library packages translated
# directly by this harness. Other standard-library models are pinned by Perennial.
goroot = Path(subprocess.check_output(["go", "env", "GOROOT"], text=True).strip())
stdlib_sources = {}
for package in ["unicode/utf8", "math/big", "internal/cpu", "encoding/json", "encoding/json/jsontext"]:
    metadata = json.loads(subprocess.check_output(["go", "list", "-json", package], text=True))
    for filename in metadata["GoFiles"]:
        source = Path(metadata["Dir"]) / filename
        stdlib_sources[str(source.relative_to(goroot))] = hashlib.sha256(source.read_bytes()).hexdigest()

# These composition theorems have a checked proof body but explicitly require
# the still-unproved production validator contract. A clean axiom audit does
# not discharge a theorem's hypotheses, so report this boundary separately.
# Match typed argument binders, not contract names in the theorem's result: the
# word-to-validator implication produces the latter without assuming it.
utf8_conditional_theorems = sorted(
    name
    for proof in proofs
    for name, signature in re.findall(r"^theorem (\w+)((?:(?!^theorem |^def |^end ).)*?) := (?:by|rfl)\b", proof.read_text(), re.M | re.S)
    if re.search(r"\(\s*\w+\s*:\s*Utf8ValidatorContract\b", signature)
)

# The actual validator loop is now checked conditional on the smaller generic
# eight-byte word helper. Keep this unresolved premise separate from the whole
# validator premise used by the existing decoder composition chain.
utf8_word_conditional_theorems = sorted(
    name
    for proof in proofs
    for name, signature in re.findall(r"^theorem (\w+)((?:(?!^theorem |^def |^end ).)*?) := (?:by|rfl)\b", proof.read_text(), re.M | re.S)
    if re.search(r"\(\s*\w+\s*:\s*Utf8WordWindowContract\b", signature)
)

axiom_sets = []
theorem_sets = {}
for name, axioms in found.items():
    if axioms not in axiom_sets:
        axiom_sets.append(axioms)
    theorem_sets[name] = axiom_sets.index(axioms)
report = {
    "status": "full decoder correctness remains pending; frozen performance gate passes for the recorded sources",
    "perennial": rev,
    "go": subprocess.check_output(["go", "version"], text=True).strip(),
    "go_target": subprocess.check_output(["go", "env", "GOOS", "GOARCH", "GOAMD64"], text=True).splitlines(),
    "map_feasibility": {
        "go_witness": "verification/fastcbor-proof/feasibility/maps/witness.go",
        "checked_theorem": "wp_resourceMap_witness",
        "proved": "actual ResourceEntries translation: allocation, two inserts with arbitrary collisions, lookup including absence, exact returned map ownership; pointer pointees are not owned by this contract",
        "blocked_shapes": ["map[felt.Felt]*felt.Felt", "map[felt.Felt]map[felt.Felt]*felt.Felt", "map[felt.SierraClassHash]felt.CasmClassHash"],
        "blocker": "pinned ArraySemantics checks array comparability but supplies no array-value GoEquals step; felt and SierraClassHash resolve to [4]uint64",
        "excluded_attempts": ["repros/FeltMap.lean", "repros/MapKeyEquality.lean"],
        "scope": "language-feature witnesses only; no map decoder, wire specification, generator extension or whole-library proof is claimed; trusted semantics unchanged",
    },
    "transaction_interface_feasibility": {
        "go_witness": "verification/fastcbor-proof/feasibility/interfaces/witness.go",
        "proof": "verification/fastcbor-proof/ProofCheck/InterfaceFeasibility.lean",
        "proved": "actual pointer-to-Transaction boxing for all five registered transaction variants; concrete type-switch classification for arbitrary pointers including typed nil; nil-interface and unrecognized dynamic-type branches",
        "theorems": 12,
        "boundary": "generated Transaction interface representation and concrete pointer type identities; transaction payload layouts are now concrete, but these witnesses do not access or allocate payloads",
        "excluded": "CBOR tag parsing and numeric tag-to-type mapping, payload decoding/ownership, dynamic method calls, registration/routing/fallback and class interfaces",
        "scope": "language-feature witnesses only; local classification numbers are not wire tags; no new trusted semantics or helper correctness premise",
    },
    "class_interface_feasibility": {
        "go_witness": "verification/fastcbor-proof/feasibility/interfaces/witness.go",
        "proof": "verification/fastcbor-proof/ProofCheck/ClassInterfaceFeasibility.lean",
        "proved": "actual pointer-to-ClassDefinition boxing for both registered variants; concrete type-switch classification for arbitrary pointers including typed nil; nil-interface and unrecognized dynamic-type branches",
        "theorems": 6,
        "boundary": "generated ClassDefinition interface representation and concrete pointer type identities; concrete class layouts are translated, but these witnesses do not access or allocate payloads",
        "excluded": "CBOR tag parsing and numeric tag-to-type mapping, payload decoding/ownership, recursive segments, dynamic method calls and registration/routing/fallback",
        "scope": "language-feature witnesses only; local classification numbers are not wire tags; no new trusted semantics or helper correctness premise",
    },
    "bigint_magnitude_foundation": {
        "specification": "verification/fastcbor-proof/ProofCheck/BigIntMagnitudeSpec.lean",
        "word_dependency": "verification/fastcbor-proof/ProofCheck/BigIntMagnitudeWord.lean",
        "proved": "arbitrary-length base-256 magnitude agrees with 64-bit word grouping; exact allocation counts and signed-int bounds; spare-zero carry adds exactly one; short-word shift/OR is exact base-256 accumulation; actual BigEndian.Uint64 load agrees with the numeric specification and preserves whole fractional input ownership",
        "implementation": "verification/fastcbor-proof/ProofCheck/BigIntMagnitude.lean",
        "implementation_theorem": "wp_bigMagnitude",
        "implementation_proved": "actual bigIntMagnitude for every owned byte slice and both negative/increment flags: count arithmetic, zeroed allocation, full-word and partial-byte packing loops, carry range loop and early break, actual SetBits normalization and in-place Neg; exact unbounded integer, canonical zero/sign, separately owned output and unchanged fractional input; no helper correctness premise",
        "pending": "typed containing readers and their complete library contracts; public Cursor.BigInt is now proved separately",
        "scope": "complete production magnitude helper under the pinned 64-bit Go model; package initialization is required; excludes compiler/runtime correctness, OOM and termination",
    },
    "bigint_public_decoder": {
        "specification": "verification/fastcbor-proof/ProofCheck/BigIntSpec.lean",
        "implementation": "verification/fastcbor-proof/ProofCheck/BigIntComplete.lean",
        "theorem": "wp_bigInt_complete",
        "proved": "every byte input at every valid cursor position; unsigned and negative uint64 heads including -2^64; arbitrary-length tags 2/3, all supported head widths and leading/empty magnitudes; exact mathematical value and consumed suffix; separately owned normalized output; unchanged fractional input; all head/type/tag/payload failures, outer rollback, inner diagnostic offsets and type-to-syntax rewriting",
        "initialization": "actual cursor initialization now retains concrete immutable errorString objects; pointer category distinctions follow from distinct messages and heap agreement, not caller assumptions",
        "errors_is_boundary": "specialized checked DecodeError.Unwrap and errors.is loop; pinned errors.Is model replaces reflection comparable check by true; all targets here have dynamic type *errors.errorString and satisfy that pointer-comparability restriction",
        "unproved_helper_premises": [],
        "scope": "partial correctness and model safety on pinned 64-bit target; compiler/runtime correctness, OOM and termination excluded; independent Juno integer wire contract, not fxamacker equivalence; containing typed roots remain open; frozen whole-library performance gate passes for the recorded sources",
    },
    "int64_public_decoder": {
        "specification": "verification/fastcbor-proof/ProofCheck/Int64Spec.lean",
        "implementation": "verification/fastcbor-proof/ProofCheck/Int64Complete.lean",
        "theorem": "wp_int64_complete",
        "proved": "every byte input at every valid cursor position; both signs, inclusive -2^63 and 2^63-1 endpoints, all supported head widths; exact signed value and suffix, unchanged fractional input; head faults, truncation, EOF, wrong types, unsupported tags and range overflow; owned exact errors and complete cursor rollback",
        "initialization": "actual cursor initialization supplies immutable error-category objects through the existing checked initialization result",
        "unproved_helper_premises": [],
        "specification_audit": "Int64Determinism.lean checks rejection diagnostic uniqueness and success/rejection separation without helper premises",
        "pending": "the containing index APIs retain the shared UTF-8 premise; their independent specification now has unique outcomes and success/rejection separation",
        "scope": "partial correctness and model safety under pinned 64-bit Go semantics; no fxamacker equivalence, compiler/runtime correctness, OOM or termination claim; production Go and trusted semantics unchanged",
    },
    "signed_array_decoder": {
        "specification": ["verification/fastcbor-proof/ProofCheck/IntsSpec.lean", "verification/fastcbor-proof/ProofCheck/IntElementFailure.lean", "verification/fastcbor-proof/ProofCheck/IntsFailureSpec.lean"],
        "implementation": "verification/fastcbor-proof/ProofCheck/IntsComplete.lean",
        "theorem": "wp_readInts_complete",
        "proved": "actual cborReadInts for every byte input at any valid cursor offset and supported depth; arbitrary mixed signed/unsigned/null elements and head widths; exact mathematical values, nil versus allocated-empty distinction, owned output, unchanged fractional input and exact suffix; all array opening/count/depth and element failures after arbitrary valid prefixes; failed destination preserved; exact nonwrapping progress and retained depth; unsigned overflow consumes its head and returns bare ErrType, signed fallback errors roll back to the element start and return owned diagnostics",
        "initialization": "actual schema/cursor initialization supplies all immutable error-category objects, including the depth category; no caller-supplied helper or initialization WP premise",
        "unproved_helper_premises": [],
        "specification_audit": "IntElementDeterminism, IntsDeterminism, IntsDisjoint and IntsFailureUnique check unique values/suffixes, exact failure progress/offset/depth/reason and success/rejection separation on representable input; no helper premise",
        "pending": "the containing index APIs retain the shared UTF-8 premise; their independent outcome uniqueness/separation audit is complete",
        "scope": "complete signed-array helper under pinned 64-bit Go semantics, not a complete record decoder; compiler/runtime correctness, OOM and termination excluded; trusted semantics and production Go unchanged",
    },
    "integer_key_decoder": {
        "specification": "verification/fastcbor-proof/ProofCheck/IntegerKeySpec.lean",
        "implementation": "verification/fastcbor-proof/ProofCheck/IntegerKeyComplete.lean",
        "theorem": "wp_readIntegerKey_complete",
        "checked_conditionally": "every byte input at every valid cursor offset; bounded unsigned keys return their actual value; supported negative/text keys are consumed as unknown; head/EOF/type/tag/truncated-text/invalid-UTF8/negative-overflow errors preserve cursor and own diagnostics; oversized unsigned keys consume the complete head and return bare ErrType; exact successful suffix and nonwrapping error consumption; input preserved",
        "unconditional_branches": "bounded unsigned keys, unsigned overflow, bounded negative keys, malformed unsigned heads and EOF; only text/general Key rejection composition needs the validator premise",
        "initialization": "actual schema/cursor initialization supplies immutable error-category objects; no initializer WP premise",
        "unproved_contract_premises": ["Utf8ValidatorContract"],
        "specification_audit": "IntegerKeyDeterminism.lean checks unique successful keys and boundaries, exact rejected consumption/diagnostics and success/rejection separation without the UTF-8 implementation premise",
        "pending": "discharge the shared UTF-8 word-helper dependency after separately approved trusted repair; both all-input index APIs compose conditionally and their independent outcomes are unique",
        "scope": "conditional partial correctness and model safety under pinned 64-bit semantics; not an unconditional helper or complete record milestone; production Go and trusted semantics unchanged",
    },
    "projection_scanner": {
        "implementation": "utils/cbor/cursor/fields.go: Cursor.NextWanted",
        "specification": ["verification/fastcbor-proof/ProofCheck/WantedSpec.lean", "verification/fastcbor-proof/ProofCheck/WantedDeterminism.lean", "verification/fastcbor-proof/ProofCheck/WantedFailureUnique.lean"],
        "specification_proved": "all-input classification and exactly one scan outcome at supported depth with distinct folded names; selected values remain unread; unknown/duplicate pairs skipped with first occurrence winning; exact suffix and selected remaining-pair count; exact failure consumption, independent diagnostic offset and retained pair count; success/rejection disjoint; no UTF-8 implementation premise for pure specification results",
        "implementation_proofs": ["verification/fastcbor-proof/ProofCheck/WantedBoundary.lean", "verification/fastcbor-proof/ProofCheck/WantedSelected.lean", "verification/fastcbor-proof/ProofCheck/WantedSkipRejected.lean", "verification/fastcbor-proof/ProofCheck/WantedValidImpl.lean", "verification/fastcbor-proof/ProofCheck/WantedRejectedImpl.lean", "verification/fastcbor-proof/ProofCheck/WantedComplete.lean"],
        "proved_branches": "actual NextWanted with exhausted closed state; open-container closure restores enclosing depth; every immediate Key rejection preserves current pair count, cursor and open state; every supported width/spelling of a fresh selected text key marks only its bit, decrements exactly one pair and leaves value unread; fractional metadata/input ownership preserved",
        "unproved_contract_premises": ["Utf8ValidatorContract"],
        "conditional_theorems": ["wp_nextWanted_key_rejected", "wp_nextWanted_selected", "wp_nextWanted_text_skip_rejected", "wp_nextWanted_valid", "wp_nextWanted_rejected", "wp_nextWanted_complete", "wp_nextWanted_projection", "wp_nextWanted_projection_valid", "wp_nextWanted_projection_rejected"],
        "mask_correspondence": "WantedSeen.lean checks all 64 field bits, the all-fields mask including count 64, initial/updated membership, fresh-field exclusion of the all-seen shortcut and unknown/duplicate lookup exclusion",
        "successful_loop": "wp_nextWanted_valid realizes every WantedValid scan through the actual loop with arbitrary unknown/duplicate/all-seen prefixes, exact consumed suffix, selected bit/remaining count, preserved metadata/input and container closure on exhaustion; complete key and immediate Skip failures checked separately",
        "rejected_loop": "wp_nextWanted_rejected realizes every WantedRejected scan through arbitrary ignored prefixes; preserves current pair/open state, input and metadata, with exact nonwrapping cursor consumption and independent diagnostic offset",
        "all_input_contract": "wp_nextWanted_complete classifies every byte input, derives actual error-category globals from cursor initialization, and produces WantedSpec with matching concrete cursor/struct/error ownership; no assumed well-formedness or selected outcome; explicit schema names/mask and seen-state correspondence",
        "initialized_specialization": "wp_nextWanted_projection derives actual immutable names/mask for any of the seven initialized projection tables; pointer agreement ties the Struct to its published table; only the existing validator premise remains",
        "pending": "receipt projection records/APIs and remaining record families; discharge shared UTF-8 word-helper premise after separately approved trusted repair",
        "scope": "complete independent scan specification and conditional all-input production NextWanted contract; Header projection records and First APIs compose conditionally while receipt projections remain open; partial correctness and model safety under pinned 64-bit semantics; production Go and trusted model unchanged",
    },
    "header_projection_records": {
        "shapes": ["headerHashProjection", "headerGlobalStateRootProjection", "headerTransactionCountProjection", "headerTimestampProjection", "headerHashAndStateRootProjection"],
        "specification_proved": "complete independent all-input record classification and unique outcomes, including optional tag policy, null/undefined, first-occurrence selection, ignored pairs and exact retained errors; canonical abstract slots and rejection position bounds; successful and rejected pair-to-scan decomposition and scan-to-pair composition; rejected-step equivalence preserves the independent pair relation",
        "ownership_and_fields": "actual five Go layouts with empty embedded skeletons; owned felt and nullable integer children; exact concrete/abstract zero equivalence; all generated zero guards and clear functions; all six concrete field slots and uniform all-input field decoding, preserving unrelated ownership and input with exact diagnostics",
        "record_boundaries": "actual generated readers on malformed tag heads, reserved/chained tag rejection with initialized non-nil sentinel, and opening errors or null/undefined after any accepted optional tag; preserved output, exact consumed prefix and retained depth as specified",
        "successful_maps": ["wp_readHeaderHashProjection_map_valid", "wp_readHeaderGlobalStateRootProjection_map_valid", "wp_readHeaderTransactionCountProjection_map_valid", "wp_readHeaderTimestampProjection_map_valid", "wp_readHeaderHashAndStateRootProjection_map_valid"],
        "successful_map_scope": "all actual generated map loops on arbitrary supported valid pair sequences and arbitrary owned initial projection values; all supported definite map heads and field orderings, unknown/duplicate fields, nested ignored contents and exact suffix; input preserved, output owned, enclosing cursor depth restored; initialized tables and error globals",
        "rejected_maps": ["wp_readHeaderHashProjection_map_rejected", "wp_readHeaderGlobalStateRootProjection_map_rejected", "wp_readHeaderTransactionCountProjection_map_rejected", "wp_readHeaderTimestampProjection_map_rejected", "wp_readHeaderHashAndStateRootProjection_map_rejected"],
        "rejected_map_scope": "all actual generated map loops after a successful definite-map opening, arbitrary owned initial projection values and rejected pair sequences; key/ignored-value scan errors, selected-field errors and later rejection after successful updates; earlier published fields remain owned, failed field replacements unpublished, input unchanged, map depth retained; exact nonwrapping retained cursor and separately positioned owned diagnostic; actual initialized tables and error-category globals",
        "unproved_contract_premises": ["Utf8ValidatorContract"],
        "tag_composition": "all ten successful/rejected map proofs have checked _after_tag generalizations; prior untagged theorem signatures remain intact as corollaries; tag prelude correctness and initialized non-nil unsupported category are proved",
        "record_theorems": ["wp_readHeaderProjection_record_valid", "wp_readHeaderProjection_record_body_rejected", "wp_readHeaderProjection_record_spec", "wp_readHeaderProjection_record_complete"],
        "all_input_record_scope": "all five actual generated internal record readers on every byte input and arbitrary owned initial projection value at supported depth; independent HeaderProjectionRecordSpec determines every success/failure; input preserved, final/retained fields owned, exact nonwrapping consumption and successful suffix, enclosing depth restored on success and entered map retained on rejection; allocated diagnostics versus bare unsupported sentinel; actual initialization supplies shared tables and error categories",
        "first_specification": "HeaderProjectionFirstSpec independently specifies zero-destination guarding, unchanged nonzero rejection, exact successful suffix and clearing of every parser failure; all-input classification, unique outcome, cleanup, success and diagnostic-bound properties are proved",
        "first_api_theorems": ["wp_decodeHeaderProjectionFirst_nonzero", "wp_decodeHeaderProjectionFirst_zero", "wp_decodeHeaderProjectionFirst_complete"],
        "first_api_entries": ["decodeHeaderHashProjectionFirst", "decodeHeaderGlobalStateRootProjectionFirst", "decodeHeaderTransactionCountProjectionFirst", "decodeHeaderTimestampProjectionFirst", "decodeHeaderHashAndStateRootProjectionFirst"],
        "first_api_scope": "all five actual registered typed First entry functions on every byte input and owned initial destination; guard diagnostics tied to initialized entry categories, nonzero destinations preserved, all parser failures invoke actual whole-struct cleanup and return nil remaining slice, successful suffix aliases original input with exact backing pointer/capacity and length; unchanged fractional input ownership; retained allocated diagnostics or bare tag sentinel with nonwrapping error offset; package initialization derived, existing UTF-8 premise explicit",
        "pending": "discharge shared UTF-8 premise; receipt projections and remaining record families are separate",
        "scope": "conditional partial correctness and model safety for complete Header projection records and all five registered First entries; routing/registration itself remains outside scope; no Go or trusted semantics changes",
    },
    "projection_initialization": {
        "proofs": ["verification/fastcbor-proof/ProofCheck/ProjectionState.lean", "verification/fastcbor-proof/ProofCheck/ProjectionFields.lean", "verification/fastcbor-proof/ProofCheck/ProjectionInitialize.lean", "verification/fastcbor-proof/ProofCheck/ProjectionOpen.lean", "verification/fastcbor-proof/ProofCheck/ProjectionScan.lean"],
        "tables": ["headerHash", "headerGlobalStateRoot", "headerTransactionCount", "headerTimestamp", "headerHashAndStateRoot", "receiptExecutionStatus", "receiptEvents"],
        "proved": "all seven actual allocation factories, exact immutable names/order/masks, cached-key consistency, bounded distinct folded names, actual global stores and complete package initializer through the checked cursor dependency; initialized table access; all seven actual Open wrappers against the complete null/map/rejection contract; initialized-table NextWanted specialization",
        "boundary": "explicit modeled runtime CPU startup as for Header; actual immutable table publication, no initializer WP premise; opening wrappers require no UTF-8 premise, scan retains existing Utf8ValidatorContract; opening proof excludes preceding projection-tag policy",
        "pending": "receipt projection record/API composition; complete Header projection internal records and First APIs now checked conditionally",
    },
    "projection_tag_policy": {
        "implementation": "core/cbor_readers.go: cborReadProjectionTag",
        "specification": "verification/fastcbor-proof/ProofCheck/ProjectionTagSpec.lean",
        "implementation_proofs": ["verification/fastcbor-proof/ProofCheck/ProjectionTagRead.lean", "verification/fastcbor-proof/ProofCheck/ProjectionTagChecked.lean", "verification/fastcbor-proof/ProofCheck/ProjectionTagComplete.lean"],
        "uniqueness": "verification/fastcbor-proof/ProofCheck/ProjectionTagUnique.lean",
        "theorem": "wp_projectionTag_complete",
        "proved": "every byte input at a valid cursor; exact consumed prefix and suffix; unchanged depth and input; untagged/EOF deferred to record reader; every supported tag width including nonpreferred encodings; malformed tag head rolls back with owned expectation diagnostic; reserved numbers 0-3 or a following tag consume first head and return canonical bare ErrUnsupported; allowed tag with absent content defers EOF to record reader; actual initialization supplies category globals",
        "specification_proved": "all-input classification, exact suffix decomposition and unique consumption/suffix/status including malformed reason; success and rejection disjoint",
        "unproved_helper_premises": [],
        "scope": "complete projection tag-prelude contract under pinned 64-bit partial-correctness model, not the following record/API decoder; trusted semantics and production Go unchanged",
    },
    "typed_felt_values": {
        "implementation": "utils/cbor/cursor/felt.go: LimbSlice[felt.Felt], Cursor.readLimbs; core/cbor_readers.go: cborReadFeltValues, cborReadPlainFeltValues",
        "specification": ["verification/fastcbor-proof/ProofCheck/LimbItemsSpec.lean", "verification/fastcbor-proof/ProofCheck/FeltValuesSpec.lean", "verification/fastcbor-proof/ProofCheck/LimbItemsUnique.lean", "verification/fastcbor-proof/ProofCheck/FeltValuesUnique.lean"],
        "implementation_proofs": ["verification/fastcbor-proof/ProofCheck/FeltValuesBoundary.lean", "verification/fastcbor-proof/ProofCheck/FeltValuesEmpty.lean", "verification/fastcbor-proof/ProofCheck/FeltValuesReaders.lean", "verification/fastcbor-proof/ProofCheck/FeltValuesView.lean", "verification/fastcbor-proof/ProofCheck/FeltValuesNonempty.lean", "verification/fastcbor-proof/ProofCheck/FeltValuesRejected.lean", "verification/fastcbor-proof/ProofCheck/FeltValuesOwnership.lean", "verification/fastcbor-proof/ProofCheck/FeltValuesValid.lean", "verification/fastcbor-proof/ProofCheck/FeltValuesFailure.lean", "verification/fastcbor-proof/ProofCheck/FeltValuesComplete.lean", "verification/fastcbor-proof/ProofCheck/FeltValuesTyped.lean"],
        "specification_proved": "every supported-depth input has exactly one complete slice outcome, including values/suffix or diagnostic offset/reason; nonnullable sequence classification, success/rejection separation, unique successful boundary/value and unique rejected prefix writes/consumption/diagnostic; count guard depends on the entire unread suffix and precedes outer/element depth; successful prefixes nonempty and failure offsets bounded",
        "proved": "actual generic LimbSlice[felt.Felt] and both typed field wrappers have complete all-input contracts at permitted depths and valid owned input cursors; null/undefined versus allocated empty/nonempty slices, exact values and nonwrapping suffix, unchanged input/depth, owned backing/capacity, all outer/count/depth/element errors with exact owned arguments and offsets; complete cursor rollback on every error; typed wrappers publish only on success and preserve arbitrary old field values on failure, with old backing ownership frameable; actual initialization supplies error categories; no assumed decoder/allocator/unsafe-view helper contract",
        "loop_proofs": ["verification/fastcbor-proof/ProofCheck/LimbWideGuard.lean", "verification/fastcbor-proof/ProofCheck/LimbWideSpec.lean", "verification/fastcbor-proof/ProofCheck/LimbWideStep.lean", "verification/fastcbor-proof/ProofCheck/LimbItemsWire.lean", "verification/fastcbor-proof/ProofCheck/ReadLimbsFailure.lean", "verification/fastcbor-proof/ProofCheck/ReadLimbsValid.lean", "verification/fastcbor-proof/ProofCheck/ReadLimbsRejected.lean", "verification/fastcbor-proof/ProofCheck/LimbItemsFailureSpec.lean", "verification/fastcbor-proof/ProofCheck/ReadLimbsComplete.lean"],
        "loop_proved": "actual readLimbs success and rejection for every byte input at a permitted felt depth and arbitrary old owned output storage; arbitrary valid starting offset, mixed fast/slow encodings, exact nonwrapping cursor/diagnostic offsets and success suffix; rejected prefix retains exactly the successful writes and preserves every remaining slot; complete outcome classification uses proved independent sequence relations and owned error globals, without assumed helper WP; guard and fast-step expressions match the translated production loop; invariant distinguishes local input window from lagging stored cursor",
        "nonempty_slice_proved": "typed allocation, first-element address, unsafe pointer conversions and unsafe.Slice call compose into both production paths; owned arrays derive positive pointers and 32-byte element bounds, owned nonempty slice derives full allocation bounds, recapping preserves ownership; no trusted change or pointer-validity assumption",
        "theorems": ["wp_limbSlice_spec", "wp_limbSlice_complete", "wp_readFeltValues_complete", "wp_readPlainFeltValues_complete", "feltValuesSpec_unique", "feltValuesSpec_total_unique"],
        "unproved_helper_premises": [],
        "pending": "Event pointer-slice ownership/allocation/implementation composition; containing receipt/projection contracts",
        "scope": "complete felt-value slice helper and both typed field readers against the independent Juno application relation, including initialization and outcome uniqueness; not a complete Event or receipt decoder contract; unchanged production Go and trusted semantics; pinned 64-bit partial correctness, excluding compiler/runtime correctness, OOM and termination",
    },
    "receipt_initialization": {
        "proofs": ["verification/fastcbor-proof/ProofCheck/ReceiptState.lean", "verification/fastcbor-proof/ProofCheck/ReceiptFields.lean", "verification/fastcbor-proof/ProofCheck/ReceiptInitialize.lean", "verification/fastcbor-proof/ProofCheck/ReceiptOpen.lean"],
        "tables": ["builtinInstanceCounter", "dataAvailability", "gasConsumed", "executionResources", "event", "l1ToL2Message", "l2ToL1Message", "transactionReceipt"],
        "proved": "all eight actual factories establish exact names/order/masks and immutable backing storage; bounded distinct folded names and valid entries; actual global allocations, cursor dependency startup, stores and immutable publication compose the complete receipt-package initializer; every actual opening wrapper borrows its initialized table and canonical error globals, realizing complete null/undefined/map/rejection behavior with exact cursor/depth/input ownership",
        "theorems": ["wp_receipt_initialize", "receipt_initialized_metadata", "receipt_initialized_table", "wp_openReceipt_complete"],
        "boundary": "explicit modeled runtime CPU startup as for Header; no assumed initializer, allocation or opening-helper WP; no UTF-8 premise for initialization or opening; subsequent record loops and public API composition remain separate",
        "unproved_helper_premises": [],
    },
    "typed_event": {
        "implementation": "core/cbor_receipts_generated.go: cborReadEvent and cborReadEventPointers; core/internal/cborreceipts/fields_generated.go: OpenEvent and immutable field table",
        "specification": ["verification/fastcbor-proof/ProofCheck/EventValues.lean", "verification/fastcbor-proof/ProofCheck/EventKeys.lean", "verification/fastcbor-proof/ProofCheck/EventPairs.lean", "verification/fastcbor-proof/ProofCheck/EventRecordSpec.lean", "verification/fastcbor-proof/ProofCheck/EventValueUnique.lean", "verification/fastcbor-proof/ProofCheck/EventPairsUnique.lean", "verification/fastcbor-proof/ProofCheck/EventRecordUnique.lean"],
        "implementation_proofs": ["verification/fastcbor-proof/ProofCheck/EventOwnership.lean", "verification/fastcbor-proof/ProofCheck/EventFieldComplete.lean", "verification/fastcbor-proof/ProofCheck/EventTable.lean", "verification/fastcbor-proof/ProofCheck/EventKeyRead.lean", "verification/fastcbor-proof/ProofCheck/EventSeen.lean", "verification/fastcbor-proof/ProofCheck/EventFieldRefine.lean", "verification/fastcbor-proof/ProofCheck/EventMapValid.lean", "verification/fastcbor-proof/ProofCheck/EventMapRejected.lean", "verification/fastcbor-proof/ProofCheck/EventBoundary.lean", "verification/fastcbor-proof/ProofCheck/EventRecordComplete.lean"],
        "specification_proved": "independent From/Keys/Data field identities and values; total/unique ASCII-folded name interpretation; all-input field classification with unique successful value/boundary, success/rejection separation, unique error and bounded offsets; selected and ignored values, arbitrary-order pairs with known-field duplicate rejection, unknown repeats, and complete direct Event record all-input classification; pair count and failure position bounds; direct record null/undefined preserves prior Event; later map errors retain partial Event and entered depth; actual unread suffix is explicit in successful field/pair/record relations because felt-value slice acceptance uses a contextual count guard; independent pair/record success has unique consumed prefix, final value and suffix; success and rejection are disjoint; rejected pairs/records uniquely fix retained partial value, consumption, diagnostic offset, depth entry and reason; EventRecordSpec has exactly one outcome for every supported-depth input without implementation helper premises",
        "proved": "concrete Event struct and child ownership, zero-child state, separate From and Keys/Data field access/update reconstruction; actual cborReadFelt call on From and cborReadFeltValues calls on either slice field have all-input contracts with complete resulting Event ownership, exact values/suffixes, original input, cursor rollback on failure and owned bounded diagnostics; unchanged fields and child allocations remain owned; actual schema/cursor initialization supplies error categories; actual receipt-package initialization/opening proved separately; exact Event metadata agrees with independent field identities, with ordered/disjoint encoded keys, correct cached words and inline indexes; actual NextField handles every supported fresh/unknown key, known duplicate and independently rejected key with exact cursor/search/seen/count state, preserved input/table ownership and owned diagnostics, conditional on the existing UTF-8 validator; seen-mask invariant agrees with independent field identities; field success/rejection refinement is checked; actual generated Event success and rejection loops cover arbitrary prior owned records, key orders, nested values, suffixes and partial-result failures; null/undefined and opening rejection compose into all-input direct record contracts, with initialized tables/categories, restored success depth, retained failure depth and distinct bounded consumption/diagnostic positions",
        "theorems": ["wp_event_sender_complete", "wp_event_values_complete", "eventRecord_classify", "wp_nextEventKey_valid", "wp_nextEventKey_duplicate_valid", "wp_nextEventKey_rejected", "wp_readEvent_map_valid", "wp_readEvent_map_rejected", "wp_readEvent_record_valid", "wp_readEvent_record_rejected", "wp_readEvent_record_complete", "eventRecordSpec_unique", "eventRecordSpec_total_unique"],
        "unproved_helper_premises": ["Utf8ValidatorContract"],
        "pending": "full receipt contracts; both receipt projection APIs now have conditional contracts",
        "scope": "all-input/all-owned-initial-value direct Event record implementation contracts against independent classified relations, conditional on the separately reported UTF-8 validator; independent pair/record outcome uniqueness and success/rejection separation are checked; the enclosing Event pointer-slice now has a checked conditional all-input contract, with private partial-allocation ownership proved in its rejected loop; no production Go or trusted semantics change; pinned 64-bit partial correctness, excluding compiler/runtime correctness, OOM and termination",
    },
    "typed_event_pointer_slice": {
        "implementation": "core/cbor_receipts_generated.go: cborReadEventPointers",
        "specification": ["verification/fastcbor-proof/ProofCheck/EventPointerItemsSpec.lean", "verification/fastcbor-proof/ProofCheck/EventPointerItemsUnique.lean", "verification/fastcbor-proof/ProofCheck/EventPointersSpec.lean", "verification/fastcbor-proof/ProofCheck/EventPointersUnique.lean"],
        "specification_proved": "independent pointer element, fixed-length sequence and complete nullable array relations; nil element versus nonnil zero Event; nil versus allocated empty slice; actual later fields/suffix retained in record slice guards; all-input classification and unique outcomes at supported depths; exact prefix/value/suffix uniqueness and success/rejection separation; rejected sequences uniquely retain private successful prefix, failed partial Event, consumed and diagnostic offsets, entered map depth and reason; count-before-depth array preflight, no blanket element-depth precheck, bounded consumed/diagnostic offsets and retained depth; outer errors roll back, inner failures retain outer array entry and record consumption/depth",
        "theorems": ["eventPointerItems_classify", "eventPointerItemsRejected_unique", "eventPointers_classify", "eventPointers_failure_bounds", "eventPointers_failure_depth", "eventPointersSpec_unique", "eventPointersSpec_total_unique", "eventStorage_take", "wp_readEventPointers_null", "wp_readEventPointers_outer_failure", "wp_readEventPointers_count", "wp_readEventPointers_depth", "wp_readEventPointers_array", "wp_readEventPointers_valid", "wp_readEventPointers_array_rejected", "wp_readEventPointers_rejected", "wp_readEventPointers_complete"],
        "implementation_proofs": ["verification/fastcbor-proof/ProofCheck/EventPointersOwnership.lean", "verification/fastcbor-proof/ProofCheck/EventPointersBoundary.lean", "verification/fastcbor-proof/ProofCheck/EventPointersValid.lean", "verification/fastcbor-proof/ProofCheck/EventPointersReadValid.lean", "verification/fastcbor-proof/ProofCheck/EventPointersRejected.lean", "verification/fastcbor-proof/ProofCheck/EventPointersReadRejected.lean", "verification/fastcbor-proof/ProofCheck/EventPointersComplete.lean"],
        "implementation_proved": "nullable pointer/result ownership; disjoint published records and unused shared-slot inventory; transfer of each shared slot into an owned zero Event; actual null/undefined and outer expected/count/depth failure branches preserve the specified destination/cursor/depth; actual successful array loop includes signed minimum and remaining-bytes/32 shared allocation, nil-element bypass, shared slots, fallback individual allocations, per-element decoding and pointer publication, final Leave and destination publication; every independent successful nullable-array outcome has exact owned contents, consumed length, suffix, unchanged input and restored depth, conditional on the existing UTF-8 validator; actual rejected loop handles arbitrary successful prefixes, nil elements, shared-slot and individual failed allocations, preserving the original destination while retaining the private successful records and failed partial Event; exact distinct consumption/diagnostic positions and entered depth; actual initialization supplies diagnostic categories; complete all-input/all-old-slice-header contract returns the independently specified unique outcome, exact owned success or original destination on failure, unchanged input and bounded cursor/depth",
        "unproved_helper_premises": ["Utf8ValidatorContract"],
        "pending": "enclosing full receipt contracts; receipt-events projection now has a conditional First API contract",
        "scope": "complete conditional all-input implementation contract at supported depths for arbitrary prior slice headers against independent classified unique outcomes; rejected-loop theorem retains exact private partial allocations, while caller-facing composition may forget unreachable temporary allocations in the garbage-collected model; retains the direct Event UTF-8 dependency; no whole-receipt/library or unconditional Header proof; pinned 64-bit partial correctness, excluding compiler/runtime correctness, OOM and termination; no production Go or trusted semantics change",
    },
    "typed_fee_unit": {
        "implementation": "core/cbor_readers.go: cborReadFeeUnit",
        "specification": "verification/fastcbor-proof/ProofCheck/FeeUnitSpec.lean",
        "implementation_proofs": [
            "verification/fastcbor-proof/ProofCheck/FeeUnitRead.lean",
            "verification/fastcbor-proof/ProofCheck/FeeUnitComplete.lean"
        ],
        "theorems": [
            "feeUnitSpec_total_unique",
            "feeUnitRejected_bound",
            "wp_readFeeUnit_valid",
            "wp_readFeeUnit_expected",
            "wp_readFeeUnit_overflow",
            "cursor_type_nonNil",
            "wp_readFeeUnit_rejected",
            "wp_readFeeUnit_complete"
        ],
        "specification_proved": "every input and prior byte classified with unique outcome; null/undefined preserve the old byte; all unsigned wire widths and values 0 through 255 accepted, including unnamed enum values; above-255 values rejected after consuming the complete integer; malformed/type/tag errors roll back; unique success prefix/value/suffix and failure reason/consumption; success/rejection disjoint and failure consumption bounded",
        "implementation_proved": "actual byte conversion/store after width check, nullish preservation, unsigned parser failure propagation, bare ErrType return after overflow without assignment; exact nonwrapping consumption/suffix and unchanged input/depth; cursor startup establishes canonical non-nil type sentinel and owned initialized diagnostics; all-input contract returns independent unique outcome and owned destination",
        "unproved_helper_premises": [],
        "scope": "complete typed helper under pinned 64-bit partial correctness/model safety; no UTF-8 or other helper-correctness premise, new trusted semantics or Go change; containing receipt remains separate; compiler/runtime correctness, OOM and termination excluded"
    },
    "bytes_into": {
        "implementation": "utils/cbor/cursor/values.go: Cursor.BytesInto",
        "specification": "verification/fastcbor-proof/ProofCheck/BytesIntoSpec.lean",
        "implementation_proofs": [
            "verification/fastcbor-proof/ProofCheck/BytesIntoRead.lean",
            "verification/fastcbor-proof/ProofCheck/BytesIntoMismatch.lean",
            "verification/fastcbor-proof/ProofCheck/BytesIntoComplete.lean"
        ],
        "theorems": [
            "bytesIntoSpec_total_unique",
            "wp_bytesInto_encoded",
            "wp_bytesInto_expected",
            "wp_bytesInto_truncated",
            "wp_bytesInto_size",
            "wp_bytesInto_valid",
            "wp_bytesInto_rejected",
            "wp_bytesInto_complete"
        ],
        "specification_proved": "every destination length and byte input classified with unique outcome; exactly one complete definite byte-string payload matching the destination length; all wire widths and arbitrary byte contents; nullish inputs rejected by this public primitive; expected-head, truncated-payload and complete-length-mismatch precedence; success prefix/value/suffix uniqueness, rejection reason uniqueness including exact actual length, and disjoint success/rejection",
        "implementation_proved": "actual payload read and length check followed by modeled copy for separately owned destination/input; successful copy replaces exactly destination bytes and restores full input ownership, exact nonwrapping consumption/suffix and cursor depth; every failure preserves destination bytes and restores cursor; length mismatch owns actual length diagnostic arguments; real initialized error categories; complete public primitive contract returns independently unique outcome",
        "unproved_helper_premises": [],
        "remaining": "full receipt; typed address and containing message readers are separately proved",
        "scope": "complete public BytesInto contract for every input and separately owned destination; relies on unchanged pinned Go copy semantics and its checked proof, not a new copy assumption; no UTF-8 premise; compiler/runtime correctness, OOM and termination excluded; typed address wrapper is proved separately"
    },
    "transaction_hash_foundation": {
        "implementation": "core/cbor_readers.go: cborReadFeltValue; core/cbor_transactions_generated.go: hash field access, cborZeroTransactionHashProjection, cborClearTransactionHashProjection and nonzero guard of decodeTransactionHashProjectionFirst; cursor.NextWanted with initialized transaction-package metadata",
        "theorems": [
            "wp_readFeltValue_complete",
            "wp_readFeltValue_valid",
            "wp_readFeltValue_rejected",
            "feltValue_classify",
            "feltValueValid_nonempty",
            "feltValueValid_prefix_unique",
            "feltValueValid_not_failure",
            "feltValueSpec_unique",
            "feltValueSpec_total_unique",
            "wp_decodeTransactionHashFirst_nonzero",
            "wp_transactionHashField_complete",
            "wp_transactionHashField_valid",
            "wp_transactionHashField_rejected",
            "transactionHashLayout_covers",
            "transactionHashLayout_zero",
            "transactionHashField_acc",
            "transactionHashOwned_length",
            "feltValue_four_words",
            "wp_transactionHashWord",
            "wp_zeroTransactionHash_owned",
            "wp_clearTransactionHash_owned",
            "wp_nextWanted_transaction_rejected",
            "wp_nextWanted_transaction_valid"
        ],
        "specification_proved": "independent by-value four-limb acceptance, null/undefined preservation of prior value, array replacement with zero-fill and supported skipped extras; all-input classification, unique value and exact suffix or rejection offset/reason, success/rejection separation; no protocol arithmetic or hash validity condition",
        "implementation_proved": "complete actual cborReadFeltValue and selected transaction-hash field contracts for every byte input and arbitrary owned initial value, exact nonwrapping consumption/suffix or complete cursor/destination rollback and initialized owned diagnostic; actual typed pointer cast and existing Limbs implementation; entire projection layout covers every concrete initial struct including nineteen empty discarded fields; checked field borrow/reconstruction, ownership-derived four-word length, actual individual guard loads, zero guard, whole-struct cleanup, and public nonzero rejection with nil suffix and exact entry diagnostic; successful/rejected NextWanted wrappers use actual initialized transaction-package tables and preserve exact scanner state/input/errors",
        "unproved_helper_premises": [
            "Utf8ValidatorContract (only the two NextWanted transaction-package wrappers; by-value felt, selected field, zero guard, cleanup and nonzero public guard proofs have no helper-correctness premise)"
        ],
        "remaining": "complete independent transactionHashProjection pair/record specification and uniqueness; actual optional-tag/open-map/NextWanted loop composition and all-input First API; shared UTF-8 word helper/model repair; felt-key map equality and state-update/class contracts",
        "scope": "complete by-value felt helper and projection dependencies, not yet a complete transaction hash projection decoder proof; unchanged production Go and trusted semantics; no workaround for pinned slice-index or array-equality gaps; partial correctness/model safety on pinned 64-bit target excludes compiler/runtime correctness, OOM and termination; no FX-equivalence, protocol-validity or strict ignored-CBOR-validity claim"
    },
    "transaction_dispatch": {
        "implementation": "core/cbor_transactions_generated.go: cborReadTransaction and CBORTransactionTags",
        "theorems": [
            "transactionVariantTag_injective",
            "transactionTagMeaning_total",
            "transactionTagMeaning_unique",
            "transactionPayload_classify",
            "transactionPayloadSpec_unique",
            "transactionDispatchSpec_total_unique",
            "wp_transaction_nullish",
            "wp_transaction_tag_rejected",
            "wp_transaction_unknown_tag",
            "wp_transaction_declare_valid",
            "wp_transaction_declare_rejected",
            "wp_transaction_deploy_valid",
            "wp_transaction_deploy_rejected",
            "wp_transaction_invoke_valid",
            "wp_transaction_invoke_rejected",
            "wp_transaction_l1Handler_valid",
            "wp_transaction_l1Handler_rejected",
            "wp_transaction_deployAccount_valid",
            "wp_transaction_deployAccount_rejected"
        ],
        "specification_proved": "independent five-variant tag profile and tag-match relation; all-input dispatch classification at supported depth, unique complete outcome under the API-documented distinct-tag caller requirement; untagged null/undefined, malformed/absent tag, unknown tag with exact tag consumption/rest, or recognized native payload outcome from its zero record; recognized payload outcomes retain complete success or partial failure values, positions and depth; tags do not independently enter cursor depth",
        "implementation_proved": "actual five tag-switch success branches allocate the appropriate zero struct, run the proved record reader and publish the correctly typed interface with exact suffix and depth; actual five rejection branches preserve old interface and retain owned private partial record/error with exact tag-plus-payload consumption and diagnostic boundary; nullish clears interface before tag lookup, malformed tags roll back without allocation, unknown tags consume only tag bytes and return the initialized nonnil bare unsupported sentinel; input ownership preserved",
        "caller_requirement": "TransactionTagsDistinct is the documented profile requirement, not an assumed decoder-helper contract; tag registration/validation remains outside proof scope; nullish/tag-rejection/unknown branch proofs do not require distinctness",
        "unproved_helper_premises": [
            "Utf8ValidatorContract (the ten recognized payload success/rejection branch contracts through record decoding)"
        ],
        "remaining": "unified internal and public First/full contracts now checked in transaction_public_decoder; transaction hash projection; shared UTF-8 word-helper/model repair; separate state-update/class work",
        "scope": "exhaustive independent specification plus actual dispatch branch contracts; the combined internal/public decoder proofs are recorded in transaction_public_decoder; no production Go or trusted-semantics changes; partial correctness/model safety on pinned 64-bit target excludes compiler/runtime correctness, OOM and termination; no protocol validity, FX-equivalence or strict ignored-CBOR-validity claim"
    },
    "transaction_public_decoder": {
        "implementation": "core/cbor_transactions_generated.go: cborReadTransaction, cborClearTransaction, DecodeTransactionFirst and DecodeTransaction",
        "theorems": [
            "wp_decodeTransactionFirst_complete",
            "wp_decodeTransaction_complete",
            "wp_decodeTransactionFirst_nonzero",
            "wp_clearTransaction",
            "transactionAPIErrorOwned_nonNil",
            "transactionFirst_classify",
            "transactionFirst_unique",
            "transactionFirst_cleanup",
            "transactionFirst_success",
            "transactionFull_classify",
            "transactionFull_cleanup",
            "transactionFull_success",
            "transactionFull_unique",
            "wp_decodeTransactionFirst_zero",
            "wp_readTransaction_spec",
            "wp_readTransaction_complete",
            "transactionErrorOwned_nonNil",
            "transactionSpec_unique",
            "transactionSpec_total_unique"
        ],
        "specification_proved": "independent normalized transaction values and complete failure details, all-input classification and unique results under distinct tags; public First and full specifications for arbitrary initial interfaces; successful decoding requires initially nil interface and exact independent transaction value/suffix; nonnil guard preserves initial interface, parser failures clear to nil, full decoding rejects nonempty suffix and clears result; success/error separation and unique public outcomes",
        "implementation_proved": "unified complete cborReadTransaction WP composes every dispatch branch and returns independently unique outcome, exact consumption/diagnostic offsets, actual cursor depth and preserved input ownership; success owns correctly typed concrete record; failures preserve old interface and own private partial records and actual errors; public First/full APIs checked for every input and owned initial interface, including typed-nil guard rejection, initialized guard/trailing diagnostics, actual cleanup, exact original-input suffix backing/capacity and nonwrapping lengths/offsets",
        "caller_requirement": "TransactionTagsDistinct is the documented pairwise-distinct tag-profile requirement; package initialization and separately owned destination/input are memory preconditions; arbitrary nonnil interfaces are rejected without dereferencing payload; tag registry/routing/fallback remain outside proof scope",
        "unproved_helper_premises": [
            "Utf8ValidatorContract (unified internal/public decoder composition through transaction record keys)"
        ],
        "remaining": "transaction hash projection; shared UTF-8 word-helper and separately approved trusted model repair; felt-key map equality and state-update/class contracts; no unconditional Header or whole-library completion",
        "scope": "partial correctness/model safety for actual translated production functions on pinned 64-bit target; generator is not trusted; no Go or trusted semantics changes; compiler/runtime correctness, OOM and termination excluded; no protocol-validity, fxamacker-equivalence or strict ignored-CBOR-validity claim"
    },
    "deploy_account_transaction": {
        "implementation": "core/cbor_transactions_generated.go: cborReadDeployAccountTransaction; core/transaction.go: DeployAccountTransaction and embedded DeployTransaction",
        "theorems": [
            "deployAccountField_classify",
            "deployAccount_embedded_acc",
            "deployAccount_hash_acc",
            "deployAccountChildren_zero",
            "wp_deployAccount_field_complete",
            "deployAccountTable_names",
            "wp_nextDeployAccountKey_valid",
            "deployAccountRecordSpec_total_unique",
            "wp_readDeployAccount_map_valid",
            "wp_readDeployAccount_map_rejected",
            "wp_readDeployAccount_record_complete"
        ],
        "specification_proved": "independent fourteen-field flat wire schema: seven nullable felt/version fields, three nullable felt-value slices, ResourceBounds map, uint64 Tip and two uint32 DA modes; field/pair/record total classification and unique success/value/suffix or rejection/partial-record/consumption/offset/depth outcomes; contextual count guards, nil versus allocated-empty values, arbitrary field order, ASCII folding, known duplicate rejection, ignored-value Skip and direct nullish old-record preservation",
        "implementation_proved": "actual nine-field outer struct with six wire fields inside embedded DeployTransaction; borrow/reconstruct outer embedded cell and each inner cell while preserving all siblings and owned children; actual nested StructFieldRef selection and all fourteen successful/rejected helper calls; initialized flattened schema metadata, cache/order/seen masks; actual generated success/rejection loops and all-input/all-owned-prior direct record composition; unchanged input, exact suffix/restored depth on success, partial prior updates and exact errors/retained cursor depth on failure; resource-map old-field preservation and bare consumed DA-mode overflow",
        "unproved_helper_premises": [
            "Utf8ValidatorContract (record-key lookup and ResourceBounds map field; no additional helper-correctness premise)"
        ],
        "remaining": "all five variant records are conditionally complete; dispatch specification and individual branches now check separately, but unified all-input dispatch and public transaction APIs remain; transaction hash projection; shared UTF-8 word-helper/model repair; separate state-update/class work",
        "scope": "all five direct transaction variants now have complete conditional record contracts, but the enclosing interface/public APIs remain unproved; actual embedded layout is proved without changing Go or trusted semantics; storage decoding only, no protocol-version, hash/address derivation, signature, DA-enum-membership or cross-field validity claim; pinned 64-bit partial correctness/model safety excludes compiler/runtime correctness, OOM and termination; no FX-equivalence or strict ignored-CBOR-validity claim"
    },
    "invoke_transaction": {
        "implementation": "core/cbor_transactions_generated.go: cborReadInvokeTransaction; core/transaction.go: InvokeTransaction",
        "theorems": [
            "invokeField_classify",
            "invokeChildren_zero",
            "wp_readPlainFeltValues_valid",
            "wp_readPlainFeltValues_rejected",
            "wp_invoke_field_complete",
            "invokeTable_names",
            "wp_nextInvokeKey_valid",
            "invokeRecordSpec_total_unique",
            "wp_readInvoke_map_valid",
            "wp_readInvoke_map_rejected",
            "wp_readInvoke_record_complete"
        ],
        "specification_proved": "independent identities and values for all sixteen fields in actual declaration order: seven nullable felt/version fields, four named felt-value slices plus plain ProofFacts slice, ResourceBounds map, uint64 Tip and two uint32 DA modes; field/pair/record total classification, unique values/suffixes and failure/partial-record/consumption/offset/depth outcomes; contextual nested count guards, nil versus allocated-empty slices/maps, arbitrary key order, ASCII folding, known duplicate rejection, ignored-value Skip and direct nullish old-record preservation",
        "implementation_proved": "actual sixteen-field ownership and cell reconstruction; exact typed helper selection including cborReadPlainFeltValues for ProofFacts and cborReadFeltValues for the four named slices; every successful/rejected call, resource-map nested depth and old-field preservation, bare consumed DA-mode overflow; metadata names/indices/order/cache/seen bits from actual initialization; actual generated success/rejection loops and complete direct record on every byte input and owned prior value; exact success suffix/restored depth, partial record retained on failure, owned errors and unchanged input; reusable plain-slice refinement adapters derived from the prior complete actual reader",
        "unproved_helper_premises": [
            "Utf8ValidatorContract (record-key lookup and ResourceBounds map field; no additional helper-correctness premise)"
        ],
        "remaining": "all five variant records are conditionally complete; dispatch specification and individual branches now check separately, but unified all-input dispatch and public transaction APIs remain; transaction hash projection; shared UTF-8 word-helper/model repair; separate state-update/class work",
        "scope": "complete conditional direct InvokeTransaction record, not enclosing Transaction interface/public API; stored values only, no proof verification, signature verification, protocol-version, hash/address derivation, DA-enum-membership or cross-field validity claim; no production Go or trusted-semantics changes; pinned 64-bit partial correctness/model safety excludes compiler/runtime correctness, OOM and termination; no FX-equivalence or strict ignored-CBOR-validity claim"
    },
    "declare_transaction": {
        "implementation": "core/cbor_transactions_generated.go: cborReadDeclareTransaction; core/transaction.go: DeclareTransaction",
        "theorems": [
            "declareField_classify",
            "declareChildren_zero",
            "wp_readFeltValues_valid",
            "wp_readFeltValues_rejected",
            "wp_declare_field_complete",
            "declareTable_names",
            "wp_nextDeclareKey_valid",
            "declareRecordSpec_total_unique",
            "wp_readDeclare_map_valid",
            "wp_readDeclare_map_rejected",
            "wp_readDeclare_record_complete"
        ],
        "specification_proved": "independent identities and values for all fourteen declared fields: seven nullable four-limb/version fields, three nullable felt-value slices, ResourceBounds map, uint64 Tip and two uint32 DA modes; field/pair/record total classification, unique values/suffixes and rejection/partial-record/consumed/diagnostic/depth outcomes; contextual nested count guards, null versus empty, arbitrary order, ASCII-folded keys, known duplicate rejection, ignored-value Skip and direct nullish old-record preservation",
        "implementation_proved": "actual fourteen-field ownership and cell borrowing/reconstruction; all successful/rejected field calls including resource-map composition, exact nested consumption/depth and bare DA-mode overflow; map duplicate keys replace the whole bounds value, and failures keep the old map/record field; actual initialized transaction metadata, ordered/cache lookup and seen masks; actual generated successful and rejected loops and all-input/all-owned-prior direct record composition; exact success suffix/restored depth, retained partial record on failure and unchanged input; two reusable typed felt-slice refinement adapters derived from the prior complete reader",
        "unproved_helper_premises": [
            "Utf8ValidatorContract (record-key lookup and the ResourceBounds map field; no additional helper-correctness premise)"
        ],
        "remaining": "all five variant records are conditionally complete; dispatch specification and individual branches now check separately, but unified all-input dispatch and public transaction APIs remain; transaction hash projection; shared UTF-8 word-helper/model repair; separate state-update/class work",
        "scope": "complete conditional direct DeclareTransaction record, not enclosing Transaction interface/public API; stored field decoding only, no protocol-version, hash, address, signature, DA-enum-membership or cross-field validity claim; no production Go or trusted-semantics changes; pinned 64-bit partial correctness/model safety excludes compiler/runtime correctness, OOM and termination; no FX-equivalence or strict ignored-CBOR-validity claim"
    },
    "deploy_transaction": {
        "implementation": "core/cbor_transactions_generated.go: cborReadDeployTransaction; core/transaction.go: DeployTransaction",
        "theorems": [
            "deployField_classify",
            "deployChildren_zero",
            "wp_deploy_field_complete",
            "deployTable_names",
            "wp_nextDeployKey_valid",
            "deployRecordSpec_total_unique",
            "wp_readDeploy_map_valid",
            "wp_readDeploy_map_rejected",
            "wp_readDeploy_record_complete"
        ],
        "specification_proved": "independent identities and values for TransactionHash, ContractAddressSalt, ContractAddress, ClassHash, ConstructorCallData and Version in actual declaration order; five nullable four-limb fields and nullable felt-value slice with contextual count guard; version storage is decoded without protocol-version validation; all-input field/pair/record classification, unique success/value/suffix and rejection/partial-record/offset/depth outcomes and disjointness; arbitrary field order, ASCII case folding, unknown-value Skip, known duplicate rejection and direct nullish preservation",
        "implementation_proved": "actual six-field ownership, borrowed field cells and reconstruction preserving every unrelated child; actual felt, named-version and felt-value-slice calls; initialized transaction schema matches independent field names, encoded order/cache and seen mask; complete successful and rejected generated loops and direct record composition on every input and owned prior record; unchanged input, exact suffix/restored depth on success and exact partial state/owned diagnostic/retained map depth on failure",
        "unproved_helper_premises": [
            "Utf8ValidatorContract (field-key/record composition only; value helpers have no correctness premise)"
        ],
        "remaining": "all five variant records are conditionally complete; dispatch specification and individual branches now check separately, but unified all-input dispatch and public transaction APIs remain; transaction hash projection; shared UTF-8 word-helper/model repair; separate state-update/class work",
        "scope": "complete conditional direct DeployTransaction record, not DeployAccountTransaction or the enclosing interface API; no hash/address derivation or protocol-validity claim; no new helper premise, production Go edit or trusted-semantics change; pinned 64-bit partial correctness/model safety excludes compiler/runtime correctness, OOM and termination; no FX-equivalence or strict ignored-CBOR-validity claim"
    },
    "l1_handler_transaction": {
        "implementation": "core/cbor_transactions_generated.go: cborReadL1HandlerTransaction; core/transaction.go: L1HandlerTransaction",
        "theorems": [
            "l1HandlerField_classify",
            "wp_readTransactionVersion_felt_outcome",
            "l1HandlerChildren_zero",
            "wp_l1Handler_field_complete",
            "l1HandlerTable_names",
            "wp_nextL1HandlerKey_valid",
            "l1HandlerRecordSpec_total_unique",
            "wp_readL1Handler_map_valid",
            "wp_readL1Handler_map_rejected",
            "wp_readL1Handler_record_complete"
        ],
        "specification_proved": "independent identities and values for TransactionHash, ContractAddress, EntryPointSelector, Nonce, CallData and Version; five nullable four-limb fields and one nullable value slice; named version has the stored felt contract without added protocol-version validation; field/pair/record classification, unique success/value/suffix and rejection/partial-record/offset/depth outcomes, disjoint success/failure, arbitrary field order, ASCII case folding, ignored values and known duplicate rejection; direct nullish record preserves prior contents",
        "implementation_proved": "actual six-field ownership/access/reconstruction and every field call with rollback and old-child preservation on failure; actual transaction-version reader satisfies the common felt outcome through its previously checked allocation/cast/limb calls; initialized six-field transaction table matches independent names, encoded order/cache and seen bits; complete actual generated success/rejection loops and direct record boundary/composition on all inputs and all owned prior records; unchanged input, exact suffix/restored depth on success, partial prior updates plus owned diagnostic and retained map depth on failure",
        "unproved_helper_premises": [
            "Utf8ValidatorContract (field-key/record composition only; value helpers have no correctness premise)"
        ],
        "remaining": "all five variant records are conditionally complete; dispatch specification and individual branches now check separately, but unified all-input dispatch and public transaction APIs remain; transaction hash projection; shared UTF-8 word-helper/model repair; separate state-update/class work",
        "scope": "complete conditional direct L1HandlerTransaction record, not the enclosing interface API; no new helper premise or trusted semantics; output/version/hash contents are decoded storage values, not protocol validity or hash recomputation; pinned 64-bit partial correctness/model safety excludes compiler/runtime correctness, OOM and termination; no Go edits, FX-equivalence or strict ignored-CBOR-validity claim"
    },
    "resource_bounds_map": {
        "implementation": "core/cbor_readers.go: cborReadResourceBoundsMap",
        "theorems": [
            "resourceMapKey_classify",
            "resourceMapEntry_classify",
            "resourceMapEntries_classify",
            "resourceMapSpec_total_unique",
            "resourceMap_failure_bounds",
            "resourceMap_failure_depth",
            "resourceMapOfEntries_last",
            "resourceMapChildren_insert",
            "resourceMapOwned_nonnull",
            "wp_readResourceMap_null",
            "wp_readResourceMap_outer_failure",
            "wp_readResourceMap_count",
            "wp_readResourceMap_depth",
            "wp_readResourceMap_map",
            "resourceMapEntryError_nonNil",
            "resourceMapFailureOwned_nonNil",
            "wp_readResourceMap_map_rejected",
            "wp_readResourceMap_valid",
            "wp_readResourceMap_rejected",
            "wp_readResourceMap_complete"
        ],
        "specification_proved": "independent total unique outcome for every input at supported depth; nil versus allocated empty map; unsigned uint32 keys including unnamed enum values and non-preferred widths; null/undefined keys consume one byte and reject; overwide unsigned keys consume their complete encoding and reject; parser failures restore key position; each value starts from zero; arbitrary duplicate keys replace whole records in wire order; all outer count/depth and nested record errors have exact consumed/offset/depth bounds; completed private entries and failed private record are uniquely determined",
        "ownership_proved": "concrete and abstract maps have matching key domains and separately owned nullable felt children; insertion handles fresh or duplicate keys without a distinctness premise, dropping replaced child ownership under the existing affine model; allocated map ownership excludes nil, including for an empty map; temporary record cell and price child transfer are separated for the Go load and insertion",
        "implementation_proved": "actual null/undefined branch and outer malformed-head/count/depth rejection; successful allocation/insertion loop for arbitrary accepted entry sequences; rejected loop after arbitrary successful prefixes including nullish keys, unsigned parser errors, width overflow and failed nested records; exact private completed map and partial value ownership, no publication on failure; complete all-input contract with independently unique map/suffix or non-nil error and consumed/offset/depth outcome, unchanged input and old destination preservation on failure",
        "unproved_helper_premises": [
            "Utf8ValidatorContract (nested ResourceBounds record decoding)"
        ],
        "remaining": "enclosing transaction variant records, tag dispatch and public APIs; shared UTF-8 word-helper proof/model repair",
        "scope": "complete conditional all-input production ResourceBounds map contract; arbitrary old map pointers are preserved on every failure, and caller-owned old map contents frame through because they are never accessed; initialized bare key sentinels remain distinct from allocated diagnostics; no new helper-correctness premise beyond existing UTF-8 dependency; no production Go or trusted semantics edits; pinned 64-bit partial correctness/model safety excludes compiler/runtime correctness, OOM and termination; no FX-equivalence or blanket RFC-validity claim"
    },
    "resource_bounds_record": {
        "implementation": "core/cbor_transactions_generated.go: cborReadResourceBounds; core/transaction.go: ResourceBounds",
        "theorems": [
            "resourceBoundsField_classify",
            "resourceBoundsField_rejected_unique",
            "resourceBoundsOwned_zero",
            "wp_resourceBounds_field_complete",
            "resourceBoundsTable_names",
            "wp_nextResourceBoundsKey_valid",
            "resourceBoundsPairs_classify",
            "resourceBoundsRecordSpec_total_unique",
            "wp_readResourceBounds_map_valid",
            "wp_readResourceBounds_map_rejected",
            "wp_readResourceBounds_record_complete"
        ],
        "specification_proved": "independent MaxAmount uint64 and nullable MaxPricePerUnit four-limb value; scalar nullish preserves old amount while price nullish clears pointer; direct record nullish preserves both; field and record classification, success/failure uniqueness and disjointness, arbitrary field order and owned prior values, unknown-value Skip and known duplicate rejection, exact suffixes, partial record contents and separate consumed/diagnostic positions with retained outer-map level",
        "implementation_proved": "actual field ownership/access and both helper calls; initialized two-field table agrees with independent names, encoded order/caches and seen masks; complete NextField cases; actual generated success/rejection loops, ignored values, map closing, nullish/opening branches and complete direct-record contract on every input at supported depth; exact unchanged input, restored depth on success and owned partial output/errors on failure",
        "unproved_helper_premises": [
            "Utf8ValidatorContract"
        ],
        "remaining": "transaction variant and public API composition; enclosing ResourceBounds map now separately has a complete conditional all-input contract",
        "scope": "complete conditional direct ResourceBounds record; price contract reflects stored felt limbs without imposing an extra uint128 validation absent from this decoder; field calls have no helper-correctness premise; pinned 64-bit partial correctness/model safety, compiler/runtime correctness/OOM/termination excluded; no production Go or trusted-model changes; no FX-equivalence or strict ignored-CBOR-validity claim"
    },
    "transaction_primitives": {
        "implementation": "core/cbor_readers.go: cborReadDataAvailabilityMode and cborReadTransactionVersion",
        "theorems": [
            "daModeSpec_total_unique",
            "daModeError_nonNil",
            "wp_readDAMode_valid",
            "wp_readDAMode_expected",
            "wp_readDAMode_overflow",
            "wp_readDAMode_rejected",
            "wp_readDAMode_complete",
            "wp_readTransactionVersion_valid",
            "wp_readTransactionVersion_rejected",
            "transactionVersionSpec_total_unique",
            "wp_readTransactionVersion_complete"
        ],
        "specification_proved": "DataAvailabilityMode accepts every uint32 unsigned value across all CBOR widths, not only named enum constants; nullish preserves old scalar; unsigned parse errors roll back and width overflow consumes value before returning bare ErrType without assignment; transaction version shares independent four-limb felt encoding, including nullish clearing, zero-padded short arrays and recursively skipped long-array extras; all inputs classified with unique values/suffixes or rejection positions/reasons",
        "implementation_proved": "actual uint32 guard/cast/store, null handling and bare overflow error; actual TransactionVersion allocation, typed pointer cast and Limbs call; publication only on success, old pointer and full cursor preserved on failure; exact owned output, unchanged input, nonwrapping consumption/suffix and initialized owned diagnostics; complete contracts at supported cursor depth with no helper-correctness premise",
        "unproved_helper_premises": [],
        "remaining": "transaction fields, record loops, ResourceBounds map, tag dispatch, transaction-hash projection and public API composition",
        "scope": "byte-decoding correctness, not semantic validation of named DA modes or Starknet transaction versions; pinned 64-bit partial correctness/model safety; no production Go or trusted-model change; compiler/runtime correctness, OOM and termination excluded"
    },
    "transaction_metadata": {
        "implementation": "core/internal/cbortransactions/fields_generated.go: all seven factories, globals, initialize and Open wrappers",
        "schemas": [
            "ResourceBounds",
            "DeclareTransaction",
            "DeployTransaction",
            "InvokeTransaction",
            "L1HandlerTransaction",
            "DeployAccountTransaction",
            "TransactionHashProjection"
        ],
        "theorems": [
            "transactionKeys_valid",
            "transactionNames_small",
            "transactionNames_distinct",
            "wp_transaction_initialize",
            "transaction_initialized_metadata",
            "transaction_initialized_table",
            "wp_openTransaction_complete"
        ],
        "metadata_proved": "literal generated field names, cached words/masks, encodings, indexes and selection bits for all tables; entry encodings agree with recorded names and ASCII-folded names are distinct; actual table and slice allocation followed by immutable publication; actual initializer with checked cursor dependency establishes all seven tables; real opening wrappers obtain their own tables and initialized error categories and satisfy complete null/map/rejection contract for every input",
        "initialization_boundary": "modeled runtime CPU startup remains a precondition, as in Header and receipt initialization; unrelated core/router startup excluded",
        "unproved_helper_premises": [],
        "remaining": "independent transaction application field relations and table-specific key/seen/sort proofs, followed by field/record/map/interface/public API composition; this foundation does not prove payload decoding or the generator",
        "scope": "all seven actual transaction metadata factories and opening wrappers, no initializer/helper correctness premise; pinned 64-bit partial correctness/model safety; no production Go or trusted-semantics change"
    },
    "transaction_receipt_public_apis": {
        "implementation": "core/cbor_receipts_generated.go: cborZeroTransactionReceipt, cborClearTransactionReceipt, cborClearTransactionReceiptPointer, DecodeTransactionReceiptFirst, DecodeTransactionReceipt, DecodeTransactionReceiptPointerFirst, DecodeTransactionReceiptPointer",
        "theorems": [
            "receiptFirst_classify",
            "receiptFirst_unique",
            "receiptFull_classify",
            "receiptFull_unique",
            "receiptFull_cleanup",
            "receiptFull_success",
            "receiptPointerFirst_classify",
            "receiptPointerFirst_unique",
            "receiptPointerFull_classify",
            "receiptPointerFull_unique",
            "receiptPointerFull_cleanup",
            "receiptPointerFull_success",
            "receiptChildren_zero_iff",
            "wp_zeroReceipt_owned",
            "wp_clearReceipt_owned",
            "wp_clearReceiptPointer_owned",
            "wp_decodeReceiptFirst_complete",
            "wp_decodeReceipt_complete",
            "wp_decodeReceiptPointerFirst_complete",
            "wp_decodeReceiptPointer_complete"
        ],
        "specification_proved": "independent First and full caller relations for value and pointer destinations; every initial value and byte input classified with unique outcomes; nonzero destination guard preserves prior value; successful First returns exact unread suffix; parse failure clears to zero value or nil pointer; full success consumes all bytes and nonempty suffix causes cleanup plus exact extraneous-byte count; parser error offsets bounded including consumed-value boundary for bare FeeUnit overflow",
        "implementation_proved": "actual nine-field zero guard linked to independent values and nil/allocated-empty distinctions; actual whole-record and pointer cleanup; real cursor construction, complete direct/pointer reader calls and Remaining; both First and both full APIs for every input and owned initial destination, exact original suffix backing pointer/capacity, unchanged input, initialized entry diagnostics, allocated parser errors or bare overflow sentinel propagated unchanged; full trailing-input cleanup and exact Extra field",
        "ownership_scope": "ReceiptOwned includes exact child ownership and the existing supplied-slot invariant for resource children; ReceiptPointerDestination owns the pointer cell and optional receipt, with nonnil owned record distinct from nil; rejected nonzero guards preserve concrete cells and children; cleanup releases unreachable children under the unchanged garbage-collected model",
        "unproved_helper_premises": [
            "Utf8ValidatorContract"
        ],
        "remaining": "shared UTF-8 word-helper/model obligation; transaction, state-update and class public APIs and dependencies remain outside these receipt results, with separate felt-key map model gap; whole-library gate remains open",
        "scope": "complete conditional public value/pointer API contracts on pinned 64-bit partial-correctness/model-safety semantics; no production Go change or trusted-semantics repair; excludes compiler/runtime correctness, OOM, termination, FX equivalence and strict ignored-CBOR validity"
    },
    "transaction_receipt": {
        "implementation": "core/cbor_receipts_generated.go: cborReadTransactionReceipt and cborReadTransactionReceiptPointer",
        "theorems": [
            "receiptPairs_classify",
            "receiptPairsRejected_depth",
            "receiptRecordSpec_total_unique",
            "receiptRecordRejected_depth",
            "receiptFailureOwned_nonNil",
            "wp_readReceipt_map_valid",
            "wp_readReceipt_map_rejected",
            "wp_readReceipt_record_complete",
            "receiptPointerSpec_total_unique",
            "wp_readReceiptPointer_complete"
        ],
        "specification_proved": "independent ordered pair, record and pointer relations; every input classified with unique outcome at supported depth; arbitrary owned initial receipt, field order, unknown skipped values and known duplicate rejection; nullish direct-record preservation versus nullish pointer clearing; nil versus allocated zero record; exact suffixes, partial contents, consumed/error positions and retained nested depth",
        "implementation_proved": "actual generated successful/rejected map loops, nullish/opening branches, all nine field calls and initialized metadata; complete direct-record contract for every input and owned initial receipt; actual pointer allocation from zero and publication only on success, with original pointer preserved on failure and private partial receipt retained; exact input ownership, cursor/depth and error propagation",
        "error_boundary": "actual GoInterface errors preserved throughout loops and pointer wrapper; FeeUnit overflow returns the initialized bare ErrType sentinel, with numeric offset denoting only consumed-value boundary; other errors retain allocated diagnostics, positions and arguments; all represented errors proved non-nil",
        "unproved_helper_premises": [
            "Utf8ValidatorContract"
        ],
        "remaining": "public value/pointer APIs now composed separately; shared Header UTF-8 helper remains unproved",
        "scope": "complete conditional internal-record and pointer-wrapper contracts under pinned 64-bit partial correctness/model safety; no production Go or trusted-semantics changes; excludes compiler/runtime correctness, OOM, termination, FX equivalence and strict ignored-CBOR validity"
    },
    "transaction_receipt_fields": {
        "implementation": "core/cbor_receipts_generated.go: all nine calls in cborReadTransactionReceipt; core/receipt.go: TransactionReceipt",
        "fields": ["Fee", "FeeUnit", "Events", "ExecutionResources", "L1ToL2Message", "L2ToL1Message", "TransactionHash", "Reverted", "RevertReason"],
        "theorems": ["receiptFieldSpec_total_unique", "receiptField_rejected_bounds", "receiptField_rejected_depth", "receiptOwned_zero", "receiptFieldError_nonNil", "feeUnitError_boundary", "wp_receipt_field_valid", "wp_receipt_field_rejected", "wp_receipt_field_complete", "receiptTable_names", "wp_nextReceiptKey_valid", "wp_nextReceiptKey_duplicate_valid", "wp_nextReceiptKey_rejected"],
        "specification_proved": "independent complete nine-field receipt values and per-field accepted/rejected encodings; classification and unique outcomes for every input at supported depth; contextual nested count guards; nil/empty distinctions; retained private resource/message values, consumed bytes and nested depth on failure; all failed fields preserve the caller-visible receipt",
        "implementation_proved": "actual struct access/reconstruction and child ownership, including shared resource slots and pointer slices; arbitrary owned prior receipt values; all nine generated field calls realize independently unique complete outcomes with unchanged input, exact suffixes/cursor/depth, full old receipt ownership on failure and owned error/private pointer-record state; initialized nine-field metadata, folded names, encoded order/caches, seen masks and actual NextField valid/duplicate/rejected cases",
        "error_boundary": "ReceiptFieldError preserves the actual GoInterface, not an assumed DecodeError representation; FeeUnit overflow consumes the unsigned value and returns the initialized bare ErrType sentinel with no allocated diagnostic; its numeric offset denotes only the consumed-value boundary, while other failures retain their structured diagnostic position; all represented failures are proved non-nil",
        "unproved_helper_premises": ["Utf8ValidatorContract"],
        "remaining": "shared UTF-8 obligation; complete internal record, pointer wrapper and public APIs proved separately",
        "scope": "complete field-call and key-lookup foundation; internal record, pointer wrapper and public APIs proved separately; pinned 64-bit partial correctness/model safety, excluding compiler/runtime correctness, OOM and termination; production Go and trusted semantics unchanged"
    },
    "execution_resources": {
        "implementation": "core/cbor_receipts_generated.go: cborReadExecutionResources and cborReadExecutionResourcesPointer",
        "theorems": ["executionPairs_classify", "executionPairsRejected_depth", "executionRecordSpec_total_unique", "executionRecordRejected_depth", "wp_readExecution_map_valid", "wp_readExecution_map_rejected", "wp_readExecution_record_complete", "executionPointerSpec_total_unique", "wp_readExecutionPointer_complete"],
        "specification_proved": "independent selected/ignored values, arbitrary ordered pair sequences, full records and pointer outcomes; every input classified and uniquely determined at supported depth; known duplicate rejection and ignored unknown fields; partial record/slot values, separate consumed and diagnostic offsets, all retained nested cursor levels, null/undefined and nil/non-nil empty-record distinctions",
        "implementation_proved": "actual successful and rejected generated map loops, initialized table lookup and seen masks, real field calls, ignored-value Skip and map closing; nullish preservation and opening failure compose into complete direct-record contracts; actual pointer wrapper clears nullish destinations or allocates the zero shared block, publishes its Value address only on success, and retains the old destination pointer and private partial block on failure; exact unchanged input, owned results/diagnostics, cursor/depth and success suffix",
        "ownership_scope": "direct record uses the documented supplied-slot layout invariant; the actual pointer wrapper establishes that invariant from the fresh block and supports every owned old destination pointer, without assuming child allocation/disjointness or helper correctness beyond the shared UTF-8 premise",
        "unproved_helper_premises": ["Utf8ValidatorContract"],
        "remaining": "full TransactionReceipt composition, cleanup and public entries; shared Header UTF-8 helper remains unproved",
        "scope": "complete conditional record and pointer-wrapper contracts under pinned 64-bit partial correctness/model safety; no production Go or trusted-semantics change; excludes compiler/runtime correctness, OOM and termination, FX equivalence and strict ignored-CBOR validity"
    },
    "execution_resources_fields": {
        "implementation": "core/cbor_receipts_generated.go: all five calls in cborReadExecutionResources and the cborExecutionResourcesBlock layout",
        "fields": ["BuiltinInstanceCounter", "MemoryHoles", "Steps", "DataAvailability", "TotalGasConsumed"],
        "theorems": ["executionFieldSpec_total_unique", "executionField_rejected_depth", "executionOwned_zero", "executionBlock_zero", "wp_execution_field_valid", "wp_execution_field_rejected", "wp_execution_field_complete", "executionTable_names", "wp_nextExecutionKey_valid", "wp_nextExecutionKey_duplicate_valid", "wp_nextExecutionKey_rejected"],
        "specification_proved": "independent field values and exact partial failures; explicit presence bits and supplied child storage distinguish null clearing from preserved storage and unpublished partial mutations; every input classified with unique field outcome at supported depth, exact consumption/suffix and diagnostic offsets, bounded retained cursor levels; independent five-name schema and folded-key meaning",
        "implementation_proved": "actual struct access and reconstruction; shared block separates into record and supplied slots without extra allocation/disjointness assumptions; all five actual field calls realize independent success/rejection and unique complete outcomes, with exact storage ownership, unchanged input and cursor/diagnostic state; initialized metadata, encoded names/order/caches, seen bits and actual NextField valid/duplicate/rejected cases agree with independent schema",
        "ownership_scope": "record pointers are nil or refer to their respective supplied slots; each slot is owned exactly once even when published or cleared; this includes every intermediate state reachable from the public pointer wrapper's fresh zero block, and arbitrary counters/presence within that layout; arbitrary external child-pointer layouts are not claimed",
        "unproved_helper_premises": ["Utf8ValidatorContract"],
        "remaining": "full receipt and public entries; complete ExecutionResources record and shared-block pointer wrapper proved separately",
        "scope": "field-call and key-lookup foundation only, not a complete ExecutionResources reader proof; pinned 64-bit partial correctness/model safety, excluding compiler/runtime correctness, OOM and termination; production Go and trusted semantics unchanged"
    },
    "scalar_receipt_resources": {
        "implementation": "core/cbor_receipts_generated.go: cborReadDataAvailability, cborReadGasConsumed, cborReadBuiltinInstanceCounter, cborReadDataAvailabilitySlot and cborReadGasConsumedSlot",
        "schemas": {
            "DataAvailability": {"module_prefix": "Availability", "fields": ["L1Gas", "L1DataGas"]},
            "GasConsumed": {"module_prefix": "GasUsed", "fields": ["L1Gas", "L1DataGas", "L2Gas"]},
            "BuiltinInstanceCounter": {"module_prefix": "Builtins", "fields": ["Pedersen", "RangeCheck", "Bitwise", "Output", "Ecsda", "EcOp", "Keccak", "Poseidon", "SegmentArena", "AddMod", "MulMod", "RangeCheck96"]}
        },
        "theorems": ["availabilityRecordSpec_total_unique", "gasUsedRecordSpec_total_unique", "builtinsRecordSpec_total_unique", "wp_availability_field_complete", "wp_gasUsed_field_complete", "wp_builtins_field_complete", "availabilityTable_names", "gasUsedTable_names", "builtinsTable_names", "wp_readAvailability_record_complete", "wp_readGasUsed_record_complete", "wp_readBuiltins_record_complete", "availabilitySlotSpec_total_unique", "gasUsedSlotSpec_total_unique", "wp_readAvailabilitySlot_complete", "wp_readGasUsedSlot_complete"],
        "specification_proved": "independent two-, three- and twelve-counter values; every unsigned width accepted, nullish fields preserve their prior value; actual legacy Ecsda spelling retained; classified unique field/pair/record outcomes at supported depth; arbitrary old counters, field order and suffix; ASCII-folded keys, known duplicate rejection and skipped unknown values; exact retained partial counters, consumed/diagnostic offsets and entered-map flag",
        "implementation_proved": "concrete struct ownership and exact field access/update, preserving every other counter; all scalar calls use checked unsigned contracts and initialized errors without UTF-8 premises; each initialized receipt-package table agrees with independent names, cached words, indexes and seen masks; actual NextField valid/duplicate/rejected cases and generated valid/rejected map loops; nullish/opening branches compose into complete direct-record contracts for every input and owned initial counters, with unchanged input, exact owned output/partial state, diagnostic and cursor/depth",
        "unproved_helper_premises": ["Utf8ValidatorContract"],
        "slot_wrappers": "complete all-input contracts for arbitrary initial supplied storage and old destination pointer; null/undefined clears the pointer and leaves storage unchanged; nonnull success publishes the supplied address and exact decoded storage; failures leave the old pointer cell unchanged and retain exact partial supplied storage, cursor/depth and owned diagnostic; old pointer may alias supplied storage, so pointee preservation is not claimed; independent outcomes are total and unique at supported depth",
        "remaining": "full receipt/public entries; enclosing ExecutionResources record and shared allocation wrapper proved separately",
        "scope": "complete conditional direct-record contracts under pinned 64-bit partial-correctness/model-safety semantics; existing UTF-8 premise appears in key/record and slot-wrapper composition; no production Go change, new helper assumption or trusted-model repair; compiler/runtime correctness, OOM and termination excluded"
    },
    "l1_to_l2_message": {
        "implementation": "core/cbor_receipts_generated.go: cborReadL1ToL2Message and cborReadL1ToL2MessagePointer; core/transaction.go: L1ToL2Message",
        "specification": [
            "verification/fastcbor-proof/ProofCheck/L1MessageValues.lean",
            "verification/fastcbor-proof/ProofCheck/L1MessageKeys.lean",
            "verification/fastcbor-proof/ProofCheck/L1MessagePairs.lean",
            "verification/fastcbor-proof/ProofCheck/L1MessageRecordSpec.lean",
            "verification/fastcbor-proof/ProofCheck/L1MessageRecordUnique.lean",
            "verification/fastcbor-proof/ProofCheck/L1MessagePointerSpec.lean"
        ],
        "theorems": ["l1MessageField_classify", "l1MessageRecordSpec_total_unique", "l1MessageChildren_zero", "wp_l1Message_field_complete", "l1MessageTable_names", "l1MessageInlineFieldTable", "wp_nextL1MessageKey_valid", "wp_readL1Message_map_valid", "wp_readL1Message_map_rejected", "wp_readL1Message_record_complete", "l1MessagePointerSpec_total_unique", "wp_readL1MessagePointer_valid", "wp_readL1MessagePointer_rejected", "wp_readL1MessagePointer_complete"],
        "specification_proved": "independent inline sender address, nonce/selector/recipient felt pointers and nullable felt-value payload; contextual nested slice guards; ASCII folded key identities; arbitrary field order and prior values; unknown values skipped and repeated known fields rejected; independently classified unique record and pointer outcomes, exact suffixes, partial values, consumed/diagnostic offsets and entered-map state; pointer nullish clears while direct-record nullish preserves contents, nonnil pointer decoding starts from zero",
        "implementation_proved": "actual access/update and whole struct/child ownership for all five fields; all five field calls complete without UTF-8 premises; actual initialized five-field metadata and exact seen bits; NextField valid/duplicate/rejected cases; generated valid/rejected record loops, nullish/opening branches and complete all-input direct contract for arbitrary owned initial values; actual pointer wrapper clears on nullish, otherwise allocates a zero concrete record and publishes only after success; failed allocation remains privately owned and prior destination pointer unchanged, with exact initialized diagnostic and cursor/depth; complete pointer contract returns independently unique outcome",
        "unproved_helper_premises": ["Utf8ValidatorContract"],
        "remaining": "enclosing full TransactionReceipt composition",
        "scope": "complete conditional direct-record and pointer-wrapper contracts under pinned 64-bit partial-correctness/model-safety semantics; shared UTF-8 premise inherited only through key/record composition; no new helper assumption, production Go change or trusted semantics repair; compiler/runtime correctness, OOM and termination excluded"
    },
    "l2_to_l1_message_pointers": {
        "implementation": "core/cbor_receipts_generated.go: cborReadL2ToL1MessagePointers",
        "specification": [
            "verification/fastcbor-proof/ProofCheck/L2MessagePointerItemsSpec.lean",
            "verification/fastcbor-proof/ProofCheck/L2MessagePointerItemsUnique.lean",
            "verification/fastcbor-proof/ProofCheck/L2MessagePointersSpec.lean",
            "verification/fastcbor-proof/ProofCheck/L2MessagePointersUnique.lean"
        ],
        "theorems": ["l2MessagePointerItems_classify", "l2MessagePointersSpec_total_unique", "l2MessageStorage_take", "wp_readL2MessagePointers_null", "wp_readL2MessagePointers_array", "wp_readL2MessagePointers_array_rejected", "wp_readL2MessagePointers_rejected", "wp_readL2MessagePointers_complete"],
        "specification_proved": "every input at supported depth has an independently unique nullable-array outcome; nil versus allocated empty slice and nil versus nonnil zero record remain distinct; contextual nested counts retain all later bytes; exact count-before-depth precedence, successful suffix, rejected consumption/diagnostic offsets and retained array/map depth",
        "implementation_proved": "actual nullish and outer expected/count/depth branches; exact signed min(count, remaining-bytes/32) shared storage; nil elements bypass record allocation; nonnil elements consume disjoint shared zero slots or allocate individual records, with actual record decoding and pointer publication; complete successful array owns exact records and children and restores outer depth; every rejected loop preserves original destination while owning private completed records, failed partial record and pointer table; caller contract retains exact error and cursor/depth, preserves input and publishes only after all elements succeed; complete all-input contract returns independently unique outcome for arbitrary prior slice headers",
        "unproved_helper_premises": ["Utf8ValidatorContract"],
        "remaining": "enclosing full TransactionReceipt composition",
        "scope": "pinned 64-bit partial correctness/model safety; shared premise inherited from message record key decoding; no new semantic or allocation assumption, production Go change or trusted-model repair; temporary failed allocations may be forgotten by callers under the garbage-collected model; compiler/runtime correctness, OOM and termination excluded"
    },
    "l2_to_l1_message": {
        "implementation": "core/cbor_receipts_generated.go: cborReadL2ToL1Message; core/transaction.go: L2ToL1Message",
        "specification": [
            "verification/fastcbor-proof/ProofCheck/L2MessageValues.lean",
            "verification/fastcbor-proof/ProofCheck/L2MessageKeys.lean",
            "verification/fastcbor-proof/ProofCheck/L2MessagePairs.lean",
            "verification/fastcbor-proof/ProofCheck/L2MessageRecordSpec.lean",
            "verification/fastcbor-proof/ProofCheck/L2MessageRecordUnique.lean"
        ],
        "theorems": ["l2MessageField_classify", "l2MessageRecordSpec_total_unique", "l2MessageChildren_zero", "wp_l2Message_field_complete", "l2MessageTable_names", "l2MessageInlineFieldTable", "wp_nextL2MessageKey_valid", "wp_readL2Message_map_valid", "wp_readL2Message_map_rejected", "wp_readL2Message_record_valid", "wp_readL2Message_record_rejected", "wp_readL2Message_record_complete"],
        "specification_proved": "independent sender felt, nullable felt-value payload and inline address values; nullish address preserves prior array; contextual nested slice count guards; ASCII folded key identities; arbitrary field order and prior contents, unknown values skipped, duplicate known fields rejected; all-input field/pair/record classification with unique success and rejection outcomes, exact suffixes, retained partial record, consumed/diagnostic offsets and entered-map flag",
        "implementation_proved": "actual field cell access and full struct/child ownership restoration; all three field calls return specified outcomes without UTF-8 premises; actual initialized receipt table matches independent names, encoded key order, cached words and inline indexes; seen bits track exact field identities; actual NextField success/duplicate/key failures; actual generated successful and rejected loops with exact partial values, consumption/diagnostic offsets and depth; actual nullish and opening failures; complete direct record contract for every input at supported depth and every owned initial value, preserving input and owning all published output/diagnostics",
        "unproved_helper_premises": ["Utf8ValidatorContract"],
        "remaining": "enclosing full receipt; pointer-slice contract now separately composed; L1ToL2Message is separate",
        "scope": "complete conditional direct-record contract under pinned 64-bit partial-correctness/model-safety semantics; field calls independently complete without the UTF-8 premise, key/record composition retains it; original Go and trusted semantics unchanged; compiler/runtime correctness, OOM and termination excluded"
    },
    "typed_address": {
        "implementation": "core/cbor_readers.go: cborReadAddress; l1/eth/address.go: Address [20]byte",
        "specification": "verification/fastcbor-proof/ProofCheck/AddressSpec.lean",
        "implementation_proofs": [
            "verification/fastcbor-proof/ProofCheck/AddressOwnership.lean",
            "verification/fastcbor-proof/ProofCheck/AddressRead.lean",
            "verification/fastcbor-proof/ProofCheck/AddressComplete.lean"
        ],
        "theorems": ["addressSpec_total_unique", "address_slice_owned", "address_slice_step", "wp_readAddress_valid", "wp_readAddress_rejected", "wp_readAddress_complete"],
        "specification_proved": "every byte input and prior array has an independently classified unique result; null/undefined preserve old value; otherwise exactly twenty bytes in a definite byte string, with all supported head widths and arbitrary contents; success prefix/value/suffix uniqueness and disjoint rejection; failure reasons inherit precise BytesInto precedence",
        "implementation_proved": "actual Null branch and named-array full slicing followed by checked BytesInto; array/slice ownership equivalence for the same cells, actual owned-array length supplies twenty-byte destination size; all successes return exact owned output, nonwrapping consumption/suffix and unchanged input/depth; all errors preserve entire old array and cursor with initialized category and owned diagnostic; complete all-input contract returns independently unique outcome",
        "unproved_helper_premises": [],
        "remaining": "full receipt; both message records and their pointer/pointer-slice wrappers are now conditionally proved",
        "scope": "pinned 64-bit partial correctness/model safety; named-array slice dispatch is derived from existing array and underlying-type rules with a proof-local tagged adapter, not a new semantic assumption or trusted-model change; no UTF-8 premise; compiler/runtime correctness, OOM and termination excluded"
    },
    "typed_boolean": {
        "implementation": "utils/cbor/cursor/values.go: Cursor.Bool; core/cbor_readers.go: cborReadBool",
        "specification": "verification/fastcbor-proof/ProofCheck/BoolSpec.lean",
        "implementation_proofs": ["verification/fastcbor-proof/ProofCheck/BoolRead.lean", "verification/fastcbor-proof/ProofCheck/BoolComplete.lean"],
        "theorems": ["wp_bool_valid", "wp_bool_rejected", "wp_readBool_valid", "wp_readBool_rejected", "wp_readBool_complete", "wp_readBool_initialized"],
        "proved": "every byte input at every valid cursor and arbitrary prior Boolean; false/true bytes assign, null/undefined preserve prior value; successful consumption is exactly one nonwrapping byte with exact suffix; errors preserve field/cursor/input and own the actual original-offset diagnostic; Peek EOF is truncated, tag-major mismatches are unsupported even for incomplete tags, all other mismatches are type errors; actual initialization supplies category objects",
        "specification_proved": "all-input classification, one-byte valid length, unique successful value/prefix/suffix, success/rejection separation and unique rejection reason",
        "unproved_helper_premises": [],
        "scope": "complete Boolean helper under pinned 64-bit partial-correctness semantics; receipt record/projection composition remains; compiler/runtime correctness, OOM and termination excluded; production Go and trusted semantics unchanged",
    },
    "nullable_uint64": {
        "implementation": "core/cbor_readers.go: cborReadUint64Pointer",
        "specification": "verification/fastcbor-proof/ProofCheck/Uint64PointerSpec.lean",
        "implementation_proofs": ["verification/fastcbor-proof/ProofCheck/Uint64PointerRead.lean", "verification/fastcbor-proof/ProofCheck/Uint64PointerComplete.lean"],
        "theorem": "wp_readUint64Pointer_complete",
        "proved": "every byte input at every valid cursor and arbitrary prior destination pointer; null/undefined clear to nil, all unsigned widths including nonpreferred encodings allocate an owned nonnil word, allocated zero distinct from nil; exact successful suffix and nonwrapping consumption; unchanged depth/input; errors retain old destination pointer and cursor with owned exact diagnostic; actual initialization supplies categories; old pointee ownership can be framed across all outcomes",
        "specification_proved": "all-input classification, unique successful value/boundary, success/rejection separation, unique full outcome and malformed diagnostic; concrete nil guard iff abstract value is absent",
        "unproved_helper_premises": [],
        "scope": "complete nullable scalar helper under pinned 64-bit partial-correctness semantics, not the containing timestamp projection; compiler/runtime correctness, OOM and termination excluded; production Go and trusted semantics unchanged",
    },
    "typed_header_projections": {
        "generator": "utils/cbor/cmd/cborgen/projections.go",
        "implementation": "core/cbor_projections_generated.go",
        "metadata": "core/internal/cborprojections/fields_generated.go",
        "entries": ["headerHashProjection", "headerGlobalStateRootProjection", "headerTransactionCountProjection", "headerTimestampProjection", "headerHashAndStateRootProjection"],
        "behavior": "typed field stores and zero guards; selected-field static lookup tables; NextWanted first-duplicate-wins; original one-tag projection policy; nullable timestamp pointer; owned output, exact suffix and failure cleanup",
        "proof_status": "complete independent all-input record and First specifications with unique outcomes; actual table initialization, concrete field ownership, successful/rejected map loops, optional tags, all internal record outcomes and all five First APIs including guard/cleanup/suffix compose conditional on UTF-8",
        "performance": "frozen whole-library gate passes for the source hashes in performance-gate.json; no universal performance claim",
    },
    "header_projection_record_specification": {
        "proofs": ["verification/fastcbor-proof/ProofCheck/HeaderProjectionValues.lean", "verification/fastcbor-proof/ProofCheck/HeaderProjectionPairs.lean", "verification/fastcbor-proof/ProofCheck/HeaderProjectionRecord.lean", "verification/fastcbor-proof/ProofCheck/HeaderProjectionCanonical.lean"],
        "shapes": ["headerHashProjection", "headerGlobalStateRootProjection", "headerTransactionCountProjection", "headerTimestampProjection", "headerHashAndStateRootProjection"],
        "proved": "metadata index-to-field-name agreement; complete selected-field classification and unique successful boundaries; independent wire-order pairs with first-key-wins and unknown/duplicate Skip semantics; all-input map/body and optional-tag record classification at supported depth; exact retained value, consumed position, diagnostic offset and entered-map state specified; absent abstract slots remain canonical through success and rejection",
        "theorems": ["headerProjectionRecord_classify", "headerProjectionRecord_canonical"],
        "boundary": "pure independent specification results, no production-decoder or UTF-8 correctness premise; bare tagUnsupported offset denotes rejection position rather than an allocated diagnostic; representation constraints for unused slots are explicit",
        "pending": "shared UTF-8 implementation premise for production refinements; pure specification results need no helper correctness; receipt projections have separate payload contracts",
    },
    "typed_classes": {
        "generator": "utils/cbor/cmd/cborgen/classes.go",
        "implementation": "core/cbor_classes_generated.go",
        "helpers": "core/cbor_classes.go",
        "metadata": "core/internal/cborclasses/fields_generated.go",
        "entries": ["ClassDefinition"],
        "variants": ["DeprecatedCairoClass", "SierraClass"],
        "behavior": "explicit tag dispatch; typed class, CASM and entry-point records; recursive SegmentLengths; value-record and string slices; felt pointers and values; opaque owned JSON byte strings; nullable arbitrary-precision integers; zero-interface guard including typed nil, exact suffix, owned output and cleanup",
        "tag_boundary": "CBORClassTags is an explicit by-value input to direct decoding; registry initialization captures actual encoded tags and installs a closure over a distinct profile; numeric tag assignment and routing adapter remain outside the proof boundary",
        "json_boundary": "the active Go 1.27 encoding/json.RawMessage alias and encoding/json/jsontext.Value byte-slice type are translated from build-selected sources; CBOR decoding treats these as bytes and does not call JSON parsing or validation methods",
        "json_opaque_type_dependencies": sorted({ax for xs in found.values() for ax in xs if ax.startswith("Perennial.encoding.json.")}),
        "json_opaque_boundary": "Goose package Assumptions also mention unrelated JSON package types, so their generated opaque type/underlying/zero declarations occur in theorem dependencies; they are reported explicitly, not treated as proved layouts. The RawMessage alias and Value slice used by the decoder are concrete. No JSON method implementation or correctness contract is assumed",
        "proof_status": "translated concrete layouts and implementations; complete independent class specifications, recursive and slice ownership contracts, metadata initialization and public API proofs remain open",
        "performance": "frozen whole-library gate passes for the source hashes in performance-gate.json; no universal performance claim",
    },
    "typed_state_update": {
        "generator": "utils/cbor/cmd/cborgen/state.go",
        "implementation": "core/cbor_state_generated.go",
        "helpers": "core/cbor_state.go",
        "metadata": "core/internal/cborstate/fields_generated.go",
        "entries": ["StateUpdate"],
        "behavior": "typed StateUpdate/StateDiff field dispatch; ordinary Go felt-keyed maps; last duplicate map key replaces the complete prior value; nil/empty/nested map and pointer distinctions; shared value arrays across inner storage maps with a typed append/make specialization of the planner's slices.Grow(nil, n) capacity policy; fixed-array pointer conversions for migrated class hashes; zero guard, exact suffix, owned output and cleanup",
        "growth_boundary": "pinned slices library has no translated Grow body; the production helper instead uses its equivalent positive-count nil-slice append/make path, directly translated with the decoder; no trusted library body or assumed Grow contract was added",
        "proof_status": "translated implementations and concrete layouts; complete independent state/map specifications, buffer ownership/growth, metadata initialization and public APIs remain open; the pinned array-value equality gap still blocks felt-key map proofs",
        "performance": "frozen whole-library gate passes for the source hashes in performance-gate.json; no universal performance claim",
    },
    "typed_transactions": {
        "generator": "utils/cbor/cmd/cborgen/transactions.go",
        "implementation": "core/cbor_transactions_generated.go",
        "metadata": "core/internal/cbortransactions/fields_generated.go",
        "entries": ["Transaction", "transactionHashProjection"],
        "variants": ["DeclareTransaction", "DeployTransaction", "InvokeTransaction", "L1HandlerTransaction", "DeployAccountTransaction"],
        "behavior": "explicit tag dispatch and typed pointer-to-interface boxing; flattened value embedding; named transaction versions and uint32 modes; ordinary Go resource maps with complete replacement on duplicate keys; value-felt hash projection with first-wins selection; zero guards including typed-nil interface rejection, exact suffix, owned output and cleanup",
        "tag_boundary": "CBORTransactionTags is an explicit by-value input to the direct API; registry initialization captures actual encoded tags and installs a closure over the validated distinct profile; registry/tag assignment and routing adapter are outside proof scope; independent transaction specification and the connection to the application tag profile remain open",
        "proof_status": "translated concrete layouts and implementations; complete independent transaction/map/projection specifications, helpers, initialization and API contracts remain open",
        "performance": "frozen whole-library gate passes for the source hashes in performance-gate.json; no universal performance claim",
    },
    "receipt_execution_status_projection": {
        "implementation": "core/cbor_receipt_projections_generated.go: cborReadReceiptExecutionStatusProjection",
        "specification": ["verification/fastcbor-proof/ProofCheck/ReceiptStatusValues.lean", "verification/fastcbor-proof/ProofCheck/ReceiptStatusPairs.lean", "verification/fastcbor-proof/ProofCheck/ReceiptStatusRecord.lean"],
        "ownership": "verification/fastcbor-proof/ProofCheck/ReceiptStatusOwnership.lean",
        "field_contracts": "verification/fastcbor-proof/ProofCheck/ReceiptStatusFieldComplete.lean",
        "map_contracts": ["verification/fastcbor-proof/ProofCheck/ReceiptStatusMap.lean", "verification/fastcbor-proof/ProofCheck/ReceiptStatusMapRejected.lean"],
        "record_contract": "verification/fastcbor-proof/ProofCheck/ReceiptStatusRecordComplete.lean: wp_readReceiptStatus_record_complete",
        "first_api_contract": "verification/fastcbor-proof/ProofCheck/ReceiptStatusAPIComplete.lean: wp_decodeReceiptStatusFirst_complete",
        "specification_proved": "all-input field/pair/body/record/First classification and unique full outcomes; successful prefix/value/suffix uniqueness and success/rejection separation; bounded error/consumption positions; exact successful/rejected NextWanted scan decomposition and composition with independent per-pair interpretation; First guard preservation, cleanup and diagnostic bounds",
        "implementation_proved": "actual layout covers every initial destination including arbitrary old string bytes; owned field calls, map loops, optional tag and opening branches compose into all-input internal record and registered First contracts; nonzero initial destination preserved/rejected, parser errors clear all partial results, success returns exact original-input suffix with pointer/capacity; input ownership preserved; initialized projection metadata and error globals supply concrete objects; allocated diagnostics and non-nil unsupported sentinel both drive actual cleanup",
        "unproved_contract_premises": ["Utf8ValidatorContract"],
        "remaining": "discharge the shared UTF-8 word-helper/model premise; full receipt and its other child records remain separate work",
        "scope": "complete conditional internal record and First API contracts under pinned 64-bit partial correctness/model safety; routing/registration, compiler/runtime correctness, OOM and termination excluded; no new helper premise, production Go change, trusted semantics change or broader receipt-family proof",
    },
    "receipt_events_projection": {
        "implementation": "core/cbor_receipt_projections_generated.go: cborZeroReceiptEventsProjection, cborReadReceiptEventsProjection, cborClearReceiptEventsProjection and decodeReceiptEventsProjectionFirst",
        "specification": [
            "verification/fastcbor-proof/ProofCheck/ReceiptEventsValues.lean",
            "verification/fastcbor-proof/ProofCheck/ReceiptEventsPairs.lean",
            "verification/fastcbor-proof/ProofCheck/ReceiptEventsRecord.lean",
            "verification/fastcbor-proof/ProofCheck/ReceiptEventsBounds.lean",
            "verification/fastcbor-proof/ProofCheck/ReceiptEventsItemUnique.lean",
            "verification/fastcbor-proof/ProofCheck/ReceiptEventsPairsUnique.lean",
            "verification/fastcbor-proof/ProofCheck/ReceiptEventsBodyUnique.lean",
            "verification/fastcbor-proof/ProofCheck/ReceiptEventsRecordUnique.lean",
            "verification/fastcbor-proof/ProofCheck/ReceiptEventsScanSpec.lean",
            "verification/fastcbor-proof/ProofCheck/ReceiptEventsScanDecompose.lean",
            "verification/fastcbor-proof/ProofCheck/ReceiptEventsRejectDecompose.lean",
            "verification/fastcbor-proof/ProofCheck/ReceiptEventsAPISpec.lean"
        ],
        "implementation_proofs": [
            "verification/fastcbor-proof/ProofCheck/ReceiptEventsOwnership.lean",
            "verification/fastcbor-proof/ProofCheck/ReceiptEventsFieldRead.lean",
            "verification/fastcbor-proof/ProofCheck/ReceiptEventsFieldComplete.lean",
            "verification/fastcbor-proof/ProofCheck/ReceiptEventsAPIComplete.lean",
            "verification/fastcbor-proof/ProofCheck/ReceiptEventsAPIGuard.lean",
            "verification/fastcbor-proof/ProofCheck/ReceiptEventsAPIOutcome.lean",
            "verification/fastcbor-proof/ProofCheck/ReceiptEventsAPIZero.lean",
            "verification/fastcbor-proof/ProofCheck/ReceiptEventsBoundary.lean",
            "verification/fastcbor-proof/ProofCheck/ReceiptEventsMap.lean",
            "verification/fastcbor-proof/ProofCheck/ReceiptEventsMapRejected.lean",
            "verification/fastcbor-proof/ProofCheck/ReceiptEventsPairError.lean",
            "verification/fastcbor-proof/ProofCheck/ReceiptEventsRecordComplete.lean",
            "verification/fastcbor-proof/ProofCheck/ReceiptEventsRecordError.lean",
            "verification/fastcbor-proof/ProofCheck/ReceiptEventsRecordRejected.lean",
            "verification/fastcbor-proof/ProofCheck/ReceiptEventsRecordValid.lean",
            "verification/fastcbor-proof/ProofCheck/ReceiptEventsTagBoundary.lean"
        ],
        "specification_proved": "independent Events/TransactionHash field identities, nullable values, selected/ignored items and wire-order first-key-wins pairs; contextual unread suffix retained for nested count guards; all-input classification and unique outcomes at supported depths for the complete optional-tag record; success/rejection separation and unique prefix/value/suffix; failures uniquely fix retained published fields, consumed bytes, diagnostic offset, remaining nesting depth and reason; consumed/diagnostic offsets and retained depth bounded; ignored/hash errors roll back their nested cursor while Event slice failures may retain consumption and additional array/map levels; exact two-way connection between batched NextWanted scans and independent per-pair interpretation, including ignored duplicate prefixes; independent all-input/all-initial-value First specification with unique outcomes, guard preservation, cleanup and diagnostic bounds",
        "implementation_proved": "owned projection fields and child allocations, exact nil-versus-allocated-empty zero guard, actual whole-struct cleanup, borrowing/reconstruction for either field; both actual selected-field helper calls have all-input contracts for arbitrary prior owned projections, preserving the other field and unchanged input, exact successful output/suffix, and original published fields on failure with initialized owned diagnostics and bounded nonwrapping cursor/depth; failed private Event allocation is not published; actual successful/rejected generated map loops, optional-tag/opening/null branches and internal record composition now checked for every byte input at supported depth and every owned initial projection; success restores enclosing depth; errors retain exact partial published fields and any nested Event-array depth/consumption; initialized projection/receipt/schema tables and categories; complete actual registered First API preserves nonzero destinations, clears all parser failures (including bare unsupported tags), and returns exact original-input suffix pointer/capacity on success",
        "theorems": [
            "receiptEventsRecord_total_unique",
            "receiptEventsRecord_failure_bounds",
            "receiptEventsRecord_failure_depth",
            "wp_zeroReceiptEvents_owned",
            "wp_clearReceiptEvents_owned",
            "wp_receiptEventsField_valid",
            "wp_receiptEventsField_rejected",
            "wp_receiptEventsField_complete",
            "receiptEventsPairs_scan",
            "receiptEventsPairs_rejected_scan",
            "wp_readReceiptEventsProjection_map_valid_after_tag",
            "wp_readReceiptEventsProjection_map_rejected_after_tag",
            "wp_readReceiptEvents_record_spec",
            "wp_readReceiptEvents_record_complete",
            "receiptEventsFirst_unique",
            "wp_decodeReceiptEventsFirst_complete"
        ],
        "unproved_contract_premises": [
            "Utf8ValidatorContract"
        ],
        "remaining": "discharge shared UTF-8 word-helper/model premise; full receipt and its other child records remain separate work",
        "scope": "complete conditional internal record and First API contracts against independent classified unique outcomes; no assumed scanner/loop/allocation/initialization correctness; guard and cleanup checked without UTF-8 premise; no production Go or trusted semantics change; pinned 64-bit partial correctness/model safety, excluding routing/registration, compiler/runtime correctness, OOM and termination",
        "record_contract": "verification/fastcbor-proof/ProofCheck/ReceiptEventsRecordComplete.lean: wp_readReceiptEvents_record_complete",
        "first_api_contract": "verification/fastcbor-proof/ProofCheck/ReceiptEventsAPIComplete.lean: wp_decodeReceiptEventsFirst_complete"
    },
    "typed_receipts": {
        "generator": "utils/cbor/cmd/cborgen/receipts.go",
        "implementation": "core/cbor_receipts_generated.go",
        "projections": "core/cbor_receipt_projections_generated.go",
        "metadata": ["core/internal/cborreceipts/fields_generated.go", "core/internal/cborprojections/receipts_generated.go"],
        "entries": ["TransactionReceipt", "*TransactionReceipt", "receiptExecutionStatusProjection", "receiptEventsProjection"],
        "behavior": "typed nested records, felt-value slices, bool/string/byte-backed FeeUnit and fixed byte-array addresses; bounded shared event/message slice storage; execution resource children allocated with their parent; original full-record duplicate rejection and projection first-wins policy; zero guards, exact suffix, owned output and failure cleanup",
        "proof_status": "translated implementations and concrete layouts; execution-status projection has complete independent record/First specifications with unique outcomes and all-input/all-owned-destination implementation contracts conditional on UTF-8, including initialized projection metadata, guards, cleanup and exact suffix; other receipt specifications, nested slice/record contracts, public API proofs remain open; all eight receipt-record table initializers and opening wrappers are checked; the direct Event record now has conditional all-input/all-owned-initial-value implementation contracts, with independent pair/record outcome uniqueness and success/rejection separation checked; enclosing Event pointer-slice now has a conditional all-input implementation contract; receipt-events projection now also has independently total/unique record and First specifications and complete conditional all-input/all-owned-destination internal record and First API contracts, including nested Event failure depth/consumption, initialized categories, guards, cleanup and exact suffix",
        "performance": "frozen whole-library gate passes for the source hashes in performance-gate.json; no universal performance claim",
    },
    "typed_block_indexes": {
        "generator": "utils/cbor/cmd/cborgen/integer_records.go",
        "implementation": "core/cbor_indexes_generated.go",
        "helpers": "core/cbor_readers.go: cborReadIntegerKey and cborReadInts",
        "behavior": "source-derived integer keys and direct typed field dispatch; first occurrence wins; unknown unsigned, negative and UTF-8 text keys skipped; signed []int with null/undefined elements zeroed; nil versus empty slices; exact suffix, owned output, zero guard and parse/trailing-byte cleanup",
        "target": "64-bit int, matching pinned linux/amd64",
        "specification": "verification/fastcbor-proof/ProofCheck/IndexesRecordSpec.lean",
        "ownership": "verification/fastcbor-proof/ProofCheck/IndexesOwnership.lean",
        "duplicate_policy": "verification/fastcbor-proof/ProofCheck/IndexesSeen.lean",
        "guard_and_cleanup": "verification/fastcbor-proof/ProofCheck/IndexesZero.lean",
        "successful_loop": "verification/fastcbor-proof/ProofCheck/IndexesMapValid.lean",
        "successful_record": "verification/fastcbor-proof/ProofCheck/IndexesValid.lean",
        "successful_public_apis": "verification/fastcbor-proof/ProofCheck/IndexesValidAPI.lean",
        "failure_loop": "verification/fastcbor-proof/ProofCheck/IndexesMapRejected.lean",
        "public_specification": "verification/fastcbor-proof/ProofCheck/IndexesAPISpec.lean",
        "complete_public_apis": "verification/fastcbor-proof/ProofCheck/IndexesAPIComplete.lean",
        "theorems": ["wp_decodeIndexesFirst_complete", "wp_decodeIndexes_complete"],
        "unproved_contract_premises": ["Utf8ValidatorContract"],
        "proof_status": "independent all-input record classification, pair-count/progress/depth bounds, first-key-wins preservation and production-mask correspondence checked; actual signed-array writes and failures compose at both owned fields; nil/empty-aware zero guard, standalone cleanup and both nonzero-destination API branches checked; complete success/failure record loops and both all-input/all-owned-destination public APIs compose against independent IndexesFirstSpec/IndexesFullSpec conditional on the explicit UTF-8 validator premise; actual initialization supplies error categories; bare unsigned-overflow sentinel and owned diagnostics remain distinct; exact suffixes, input ownership, partial output, depth/progress and cleanup are checked",
        "specification_audit": "IndexesDeterminism, IndexesDisjoint, IndexesFailureUnique and IndexesOutcomeUnique check unique successful values/suffixes, exact partial output/progress/offset/depth/diagnostics, success/rejection separation and exactly one specified First/full outcome for every representable byte input and initial destination; these pure results have no helper correctness premise",
        "pending": "arbitrary text keys retain the existing UTF-8 implementation dependency, so no unconditional index milestone is claimed",
        "performance": "frozen whole-library gate passes for the source hashes in performance-gate.json; no universal performance claim",
    },
    "stdlib_go_source_sha256": dict(sorted(stdlib_sources.items())),
    "initialization_boundary": {
        "cursor": "proved after runtime CPU startup; concrete immutable category objects retained, so distinct error messages establish distinct category pointers; no initializer WP premise",
        "schema": "complete core/internal/cborschema startup proved, including both factories and global stores; exact immutable Header/GasPrice tables retained in package initialization",
        "entry_errors": "complete cborerr startup proved with both canonical sentinels and errors/fmt dependency initialization",
        "decoder_preconditions": "schema initialization supplies cursor/tables/primitive errors; cborerr initialization supplies public-API errors where read; no core or fastcbor initialization premise",
        "runtime_precondition": "internal/cpu is marked initialized in the modeled package state, which retains arbitrary immutable HasADX and HasBMI2 flag values",
        "excluded": "hardware probing, runtime creation of that initial state, unrelated core/planner startup, routing/registration and diagnostic formatting",
        "pending": "initialization for future whole-library extensions; no unproved initializer WP premise remains for the Header decoder dependencies",
        "sentinel_boundary": "exported sentinel variables are read-only; fastcbor reexports their initial canonical values and aliases DecodeError; mutation of those variables is outside the contract",
        "pinned_fmt_type_placeholders": ["fmt.pp", "fmt.readRune"],
    },
    "unproved_contract_premises": {
        "Utf8ValidatorContract": {
            "status": "unproved; the checked complete validator loop reduces this to the explicit Utf8WordWindowContract",
            "definition": "verification/fastcbor-proof/ProofCheck/Utf8Contract.lean",
            "checked_reduction": "verification/fastcbor-proof/ProofCheck/Utf8FromWord.lean",
            "direct_theorems": utf8_conditional_theorems,
            "claim": "conditional composition only; no closed general UTF8, Key, string, GasPrice or Header correctness claim",
        },
        "Utf8WordWindowContract": {
            "status": "unproved; the actual generic eight-byte helper is blocked by the documented pinned-semantics defect",
            "definition": "verification/fastcbor-proof/ProofCheck/Utf8WordContract.lean",
            "checked_loop": "verification/fastcbor-proof/ProofCheck/Utf8Loop.lean",
            "excluded_proof_attempt": "verification/fastcbor-proof/repros/Utf8Word.lean",
            "direct_theorems": utf8_word_conditional_theorems,
            "claim": "the complete production validator loop and its reduction to this helper are checked; the helper itself is not proved and no trusted semantics has been changed",
        },
    },
    "production_entry_manifest": {
        "path": "verification/fastcbor-proof/results/entry-manifest.json",
        "checker": "go run ./verification/fastcbor-proof/cmd/entrymanifest -check (run by check.sh)",
        "destinations": len(entry_manifest["roots"]),
        "production_destinations": sum(bool(root["production_sites"]) for root in entry_manifest["roots"]),
        "typed_registered": [root["type"] for root in entry_manifest["roots"] if root["typed_registered"]],
        "structural_shapes": len(entry_manifest["shapes"]),
        "frozen_benchmark_cases": len(entry_manifest["benchmarks"]),
        "status": "checked preparatory inventory only; no additional typed reader, wire specification, compatibility result, performance result or formal proof is claimed",
    },
    "compatibility_smoke": {
        "path": "verification/fastcbor-proof/results/compatibility-smoke.json",
        "checker": "python3 verification/fastcbor-proof/smoke/run.py --check (separate source/corpus evidence check)",
        "baseline_counts": compatibility_smoke["baseline"]["counts"],
        "candidate_counts": compatibility_smoke["candidate"]["counts"],
        "race_checkptr": {
            "path": "verification/fastcbor-proof/results/compatibility-smoke-race-checkptr.json",
            "flags": race_checkptr_smoke["go_build_flags"],
            "baseline_counts": race_checkptr_smoke["baseline"]["counts"],
            "candidate_counts": race_checkptr_smoke["candidate"]["counts"],
            "claim": "saved corpus and instrumentation checks passed under race/checkptr=2; not universal race-freedom or memory-safety proof",
        },
        "status": "saved no-fallback corpus compatibility evidence for the recorded Go source hashes; not a formal equivalence proof or performance-gate result",
    },
    "performance_gate": {
        "path": "verification/fastcbor-proof/results/performance-gate.json",
        "archive": "verification/fastcbor-proof/results/" + performance_gate["archive"],
        "status": performance_gate["status"],
        "rounds": performance_gate["rounds"],
        "compatible_decoder_cases": performance_gate["compatible_decoder_cases"],
        "pairs": performance_gate["pairs"],
        "checker": "bench/check.py --archive results/performance-66d35a944.zip (run by check.sh)",
        "scope": "source-specific empirical performance; 177 comparable decoder cases pass all three metrics at the prespecified twenty-round look; six removed projections excluded and two zero-decoder controls reported separately; raw logs and decisions are rechecked; binary hashes were checked locally and are recorded, but executables are not archived",
    },
    "theorem_axiom_set": theorem_sets,
    "axiom_sets": axiom_sets,
    "sorryAx_in_pilot": False,
    "native_evaluation_axioms_in_pilot": False,
    "caveat": "Partial correctness and model safety under the pinned 64-bit Go model, with package initialization and owned inputs as preconditions; compiler/runtime correctness, OOM and termination are excluded. Generated opaque type declarations remain and are reported by the theorem axiom sets. Complete all-input/all-owned-destination Header API composition is checked against independent HeaderFirstSpec/HeaderFullSpec, but requires Utf8ValidatorContract. The complete actual UTF-8 validator loop reduces that premise to Utf8WordWindowContract; the actual generic eight-byte helper remains unproved because the documented trusted slice-index repair is unapplied. Integer-key dispatch is checked for every input conditional on the same Utf8ValidatorContract, because even unknown text keys are validated before their values are skipped. Complete public BigInt decoding is now checked against independent integer success/rejection relations, with exact values, consumption, owned outputs/errors, input preservation and rollback; the pinned errors.Is comparability model is restricted here to concrete pointer targets. Public Cursor.Int64 and the complete typed signed-array helper are also checked for every byte input without helper premises; arrays preserve nil/empty distinctions and specify unsigned-overflow consumption, signed-fallback rollback and destination preservation on errors. Relevant initialization establishes actual immutable tables and error-category objects after modeled runtime CPU startup. Resource-key map, transaction-interface and class-interface witnesses are checked, but they are not payload decoder proofs; felt-key maps expose a separate array-equality modeling gap. All 15 manifested destinations have registered typed readers: Header, its five projections, BlockTransactionsIndexes, the receipt family, Transaction, transactionHashProjection, StateUpdate and ClassDefinition. BlockTransactionsIndexes now has conditional all-input/all-owned-destination contracts against independent IndexesFirstSpec/IndexesFullSpec, including initialization, nil/empty distinctions, both arrays, first-key-wins, exact suffixes and parser/trailing/guard errors; independent outcome uniqueness/separation is checked. NextWanted has a conditional all-input scan contract with initialized error categories and exact state/diagnostic ownership; all five Header projection internal records now have conditional all-input/all-owned-initial-value contracts against independent HeaderProjectionRecordSpec, including optional tags, exact suffixes, retained partial results, depth and owned diagnostics. Their five registered First entry functions now also have conditional all-input/all-owned-destination contracts against independent HeaderProjectionFirstSpec, including actual guard rejection, failure cleanup, nil error suffix and exact original-input suffix aliasing. The receipt execution-status projection has complete conditional record and First contracts; the receipt-events projection now also has complete conditional record and First contracts, including nested Event failure depth/consumption, independent outcome uniqueness, initialized categories, guards, cleanup and exact suffix. Full receipts, transactions, state updates and classes are translated and behavior-tested, but their complete decoder contracts remain open. Transaction and class decoding take explicit tag profiles; its connection to registry initialization is outside the proof boundary. Projection table initialization is checked for all seven tables, and receipt payload-table initialization and opening wrappers for all eight tables; transaction, state and class payload-table initialization proofs remain open. The manifest and saved no-fallback corpus results are source-specific evidence, not formal fxamacker equivalence. Ignored CBOR text is intentionally not always validated, so successful decoding is not blanket RFC validity. The frozen performance gate passes for the recorded source hashes; no universal performance or whole-library correctness theorem is claimed.",
    "sha256": {str(p.relative_to(repo)): hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(set(inputs))},
}
(here / "results").mkdir(exist_ok=True)
(here / "results/proof-audit.json").write_text(json.dumps(report, indent=2) + "\n")
print(f"Audited {len(found)} pilot theorems: none depends on sorryAx or native evaluation axioms.")
