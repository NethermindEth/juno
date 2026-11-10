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
from pathlib import Path

here = Path(__file__).resolve().parent
repo = here.parent.parent
log = Path(sys.argv[1]).read_text()
proofs = sorted((here / "ProofCheck").glob("*.lean"))
expected = {name for proof in proofs for name in re.findall(r"^theorem (\w+)", proof.read_text(), re.M)}
found = {}
for name, body in re.findall(r"'Perennial\.github_com\.NethermindEth\.juno\.core\.(\w+)' depends on axioms: \[(.*?)\]", log, re.S):
    found[name] = [x.strip() for x in body.split(",") if x.strip()]
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
for rel in ["core", "core/felt", "utils/cbor/cursor", "utils/cbor/fastcbor", "utils/cbor/cmd/cborgen"]:
    inputs += [p for p in (repo / rel).glob("*.go") if not p.name.endswith("_test.go")]
inputs += list((here / "config").rglob("*.toml"))
axiom_sets = []
theorem_sets = {}
for name, axioms in found.items():
    if axioms not in axiom_sets:
        axiom_sets.append(axioms)
    theorem_sets[name] = axiom_sets.index(axioms)
report = {
    "status": "pilot only; full decoder correctness and performance gates pending",
    "perennial": rev,
    "go": subprocess.check_output(["go", "version"], text=True).strip(),
    "theorem_axiom_set": theorem_sets,
    "axiom_sets": axiom_sets,
    "sorryAx_in_pilot": False,
    "native_evaluation_axioms_in_pilot": False,
    "caveat": "Package initialization and relevant global ownership are preconditions. Generated opaque type axioms remain. End-to-end populated Header proofs still cover only a single inline Number field, including cleanup after a subsequent truncation at byte 9. General head/unsigned/container success contracts, container count/depth failures, slow-head rejection classification, scalar type/tag and extended-head truncation failures, and null-field contracts are proved separately; their full Header composition, general EOF and remaining error propagation are pending. The typed felt reader is proved against the independent FeltValid success relation: null/undefined produce nil, short arrays zero-fill, and extended arrays retain four unsigned limbs while consuming arbitrarily nested extras satisfying SkipValid. This covers independently chosen head/limb widths and arbitrary limb values, dispatch, pointer conversion, fresh owned allocation for arrays, exact suffix and cursor advancement without signed wraparound. Depth rejection covers all supported array heads. Atomic Skip success, simple/string/tag rejection and EOF are proved through the public wrapper, including exact consumption on success and cursor rollback with retained diagnostic offset on error. General recursive Skip success is proved for the independent SkipValid relation, including arrays/maps/accepted tags, chained-tag depth and both wide-felt shortcuts; all child hypotheses are discharged by well-founded induction on encoded length. This is not yet a converse or complete rejection theorem. Recursive Skip rejection and other head errors remain pending, as do remaining felt errors and rejection/converse completeness. Definite payload extraction and oversized-length rollback are proved, including exact borrowed ranges and suffixes. BytesBorrowed composes both contracts; String and its typed field reader propagate truncation while preserving prior field state. General UTF8 validation is pending: a reversed operand tuple in the pinned generic slice-index rule blocks the actual word-helper proof; a separate repair proposal awaits approval, and its expected-failure reproducer is outside the audited target. General ASCII folding and both name-matching passes are proved: success identifies a folded-name match and failure excludes all names. Distinct folded names give a unique result; Header and GasPrice satisfy that property. New-field bit updates and duplicate rejection with owned diagnostic names are proved. Matching and duplicate marking preserve fractional metadata ownership. Integer-key classification and int64-range rejection, wrong-key type/tag diagnostics, and text-body truncation are proved through public Key and alternate field lookup, preserving exact success suffixes or original cursor/input on error. The full batched ASCII helper is proved equivalent to every byte being below 128 for arbitrary input lengths, preserving whole-input ownership. Public Key returns exact borrowed slices and suffixes for inline/one-byte ASCII keys. These compose through otherField, name matching and marking into an independent FieldReadOutcome covering unknown/new/duplicate fields, fractional metadata and owned duplicate diagnostics after key consumption. Successful wider text heads, non-ASCII keys, UTF8, remaining head-error propagation, cached lookup and initialization establishing shared ownership remain pending; older table examples still require exclusive ownership. Complete felt rejection behavior, UTF8, byte-to-BigInt packing/carry, complete BigInt decoding, and whole-library correctness remain unproved.",
    "sha256": {str(p.relative_to(repo)): hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(set(inputs))},
}
(here / "results").mkdir(exist_ok=True)
(here / "results/proof-audit.json").write_text(json.dumps(report, indent=2) + "\n")
print(f"Audited {len(found)} pilot theorems: none depends on sorryAx or native evaluation axioms.")
