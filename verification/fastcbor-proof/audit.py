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
        "pending": "BlockTransactionsIndexes record-loop refinement, outcome uniqueness/separation and complete public API composition; integer-key contract remains conditional on UTF-8 validation",
        "scope": "partial correctness and model safety under pinned 64-bit Go semantics; no fxamacker equivalence, compiler/runtime correctness, OOM or termination claim; production Go and trusted semantics unchanged",
    },
    "signed_array_decoder": {
        "specification": ["verification/fastcbor-proof/ProofCheck/IntsSpec.lean", "verification/fastcbor-proof/ProofCheck/IntElementFailure.lean", "verification/fastcbor-proof/ProofCheck/IntsFailureSpec.lean"],
        "implementation": "verification/fastcbor-proof/ProofCheck/IntsComplete.lean",
        "theorem": "wp_readInts_complete",
        "proved": "actual cborReadInts for every byte input at any valid cursor offset and supported depth; arbitrary mixed signed/unsigned/null elements and head widths; exact mathematical values, nil versus allocated-empty distinction, owned output, unchanged fractional input and exact suffix; all array opening/count/depth and element failures after arbitrary valid prefixes; failed destination preserved; exact nonwrapping progress and retained depth; unsigned overflow consumes its head and returns bare ErrType, signed fallback errors roll back to the element start and return owned diagnostics",
        "initialization": "actual schema/cursor initialization supplies all immutable error-category objects, including the depth category; no caller-supplied helper or initialization WP premise",
        "unproved_helper_premises": [],
        "pending": "containing index-record loop refinement and complete public API composition; independent exhaustive record specification and field ownership are checked; integer-key contract is composed conditional on UTF-8 validation",
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
        "pending": "discharge the shared UTF-8 word-helper dependency after separately approved trusted repair; prove the containing index-record loop and complete public APIs; zero guards, standalone cleanup and nonzero-destination branches are checked",
        "scope": "conditional partial correctness and model safety under pinned 64-bit semantics; not an unconditional helper or complete record milestone; production Go and trusted semantics unchanged",
    },
    "typed_header_projections": {
        "generator": "utils/cbor/cmd/cborgen/projections.go",
        "implementation": "core/cbor_projections_generated.go",
        "metadata": "core/internal/cborprojections/fields_generated.go",
        "entries": ["headerHashProjection", "headerGlobalStateRootProjection", "headerTransactionCountProjection", "headerTimestampProjection", "headerHashAndStateRootProjection"],
        "behavior": "typed field stores and zero guards; selected-field static lookup tables; NextWanted first-duplicate-wins; original one-tag projection policy; nullable timestamp pointer; owned output, exact suffix and failure cleanup",
        "proof_status": "translated implementations; complete independent projection specifications, NextWanted contract, table initialization and public entry proofs remain open",
        "performance": "frozen whole-library gate passes for the source hashes in performance-gate.json; no universal performance claim",
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
    "typed_receipts": {
        "generator": "utils/cbor/cmd/cborgen/receipts.go",
        "implementation": "core/cbor_receipts_generated.go",
        "projections": "core/cbor_receipt_projections_generated.go",
        "metadata": ["core/internal/cborreceipts/fields_generated.go", "core/internal/cborprojections/receipts_generated.go"],
        "entries": ["TransactionReceipt", "*TransactionReceipt", "receiptExecutionStatusProjection", "receiptEventsProjection"],
        "behavior": "typed nested records, felt-value slices, bool/string/byte-backed FeeUnit and fixed byte-array addresses; bounded shared event/message slice storage; execution resource children allocated with their parent; original full-record duplicate rejection and projection first-wins policy; zero guards, exact suffix, owned output and failure cleanup",
        "proof_status": "translated implementations and concrete layouts; complete independent receipt specifications, nested slice/record contracts, table initialization and public API proofs remain open",
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
        "proof_status": "independent all-input record classification, pair-count/progress/depth bounds, first-key-wins preservation and production-mask correspondence checked; actual signed-array writes and failures compose at both owned fields; nil/empty-aware zero guard, standalone cleanup and both nonzero-destination API branches checked",
        "pending": "record success/failure loop refinement, outcome uniqueness/separation and zero-destination public API composition; no complete index-record theorem yet; arbitrary text keys retain the existing UTF-8 dependency",
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
    "caveat": "Partial correctness and model safety under the pinned 64-bit Go model, with package initialization and owned inputs as preconditions; compiler/runtime correctness, OOM and termination are excluded. Generated opaque type declarations remain and are reported by the theorem axiom sets. Complete all-input/all-owned-destination Header API composition is checked against independent HeaderFirstSpec/HeaderFullSpec, but requires Utf8ValidatorContract. The complete actual UTF-8 validator loop reduces that premise to Utf8WordWindowContract; the actual generic eight-byte helper remains unproved because the documented trusted slice-index repair is unapplied. Integer-key dispatch is checked for every input conditional on the same Utf8ValidatorContract, because even unknown text keys are validated before their values are skipped. Complete public BigInt decoding is now checked against independent integer success/rejection relations, with exact values, consumption, owned outputs/errors, input preservation and rollback; the pinned errors.Is comparability model is restricted here to concrete pointer targets. Public Cursor.Int64 and the complete typed signed-array helper are also checked for every byte input without helper premises; arrays preserve nil/empty distinctions and specify unsigned-overflow consumption, signed-fallback rollback and destination preservation on errors. Relevant initialization establishes actual immutable tables and error-category objects after modeled runtime CPU startup. Resource-key map, transaction-interface and class-interface witnesses are checked, but they are not payload decoder proofs; felt-key maps expose a separate array-equality modeling gap. All 15 manifested destinations have registered typed readers: Header, its five projections, BlockTransactionsIndexes, the receipt family, Transaction, transactionHashProjection, StateUpdate and ClassDefinition. Projection, integer-key index, receipt, transaction, state-update and class implementations are translated and behavior-tested, but their complete decoder contracts remain open. Transaction and class decoding take explicit tag profiles; its connection to registry initialization is outside the proof boundary. Projection, receipt, transaction, state and class table initialization proofs also remain open. The manifest and saved no-fallback corpus results are source-specific evidence, not formal fxamacker equivalence. Ignored CBOR text is intentionally not always validated, so successful decoding is not blanket RFC validity. The frozen performance gate passes for the recorded source hashes; no universal performance or whole-library correctness theorem is claimed.",
    "sha256": {str(p.relative_to(repo)): hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(set(inputs))},
}
(here / "results").mkdir(exist_ok=True)
(here / "results/proof-audit.json").write_text(json.dumps(report, indent=2) + "\n")
print(f"Audited {len(found)} pilot theorems: none depends on sorryAx or native evaluation axioms.")
