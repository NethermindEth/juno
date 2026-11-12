# Proposed array-value equality repair

Status: **awaiting explicit approval; the patch is not applied**. This proposal
is separate from the adopted Header generic slice-index repair. Approval of that
upstream branch did not approve this rule. Current base: Perennial
`9489271c166c7567fb4561f3aed83da866cc921f`. The patch was originally prepared against
`84d5380639a38f56cd517f1e81246fad7be5b21b`; the relevant array model is unchanged.

## Defect and scope

The pinned model is incomplete: Go can compare two `[4]uint64` values and use them
as map keys, but `ArraySemantics` has no array-value `GoEquals` rule.
`equals_array` checks comparability only. The existing Go map witnesses pass,
while their translated felt-key insertion proof cannot establish `SafeMapKey`.
This is not a failure of the CBOR decoder or evidence that all array operations
are unsupported.

Go compares array elements in ascending index order, stopping at the first
mismatch. Float comparison follows IEEE equality; interface comparison can panic
for uncomparable dynamic values. The array operation must retain these behaviors.
The array length is part of its type and must fit a nonnegative `int`.
Sources: [comparison operators](https://go.dev/ref/spec#Comparison_operators)
and [array types](https://go.dev/ref/spec#Array_types), checked 2026-11-11.

## Exact proposed trusted change

[`array-value-equality.patch`](array-value-equality.patch) adds one rule,
`ArraySemantics.go_eq_array`, and registers/exports it. It expands equality into
the same left-folded, short-circuiting comparison scheme as
`CoreComparisonSemantics.go_eq_struct`. Each element is obtained using the
existing array `Index` operation and compared using `GoEquals` at the Go element
type. It does not compare the Lean representation directly.

The rule first checks `0 ≤ n < 2^63` and that both representation lists have
length `n`. Malformed representations use the existing `AngelicExit` convention
of array allocation/store; they are not Go arrays of the stated type. No
application proof may use that case to establish successful decoding.

| Trusted-change cost | Proposed amount |
|---|---:|
| New semantics rules | 1 |
| Changed existing semantics rules | 0 |
| Goose branches, including contextual branches | 0 |
| New opaque helper models or correctness axioms | 0 |
| New negative translation tests | 0 |
| Trusted files changed | 1: `Perennial/Golang/Defn/Array.lean` |
| Text delta, including registration/export | 16 inserted, 2 deleted lines |

The new semantics rule is itself an addition to the trusted model. The zero in
the opaque-helper row does not mean that the patch adds no trust. The proposal
does not change `GoSemanticsFunctions`, map operations, float operations or the
meaning of `SafeMapKey`.

Closest decided precedents in the framework's `fix-semantics/examples.md`:

- Case 1: complete an existing uniform operation scheme without a translator
  special case. Here the sibling scheme is struct equality.
- Cases 13 and 15: use element Go equality, preserving float and interface
  behavior; do not replace map key comparison with representation equality.
- Case 17 and the existing array load/store rules: exclude invalid array
  representations, and obtain length facts for actual owned arrays.

Sibling inspection found no second missing rule to add in this change. Named
arrays and instantiated generic array types reach the underlying array through
`go_op_underlying`. Nested arrays recursively need the same new rule. Structs
containing arrays already delegate to field equality; arrays of structs delegate
back to the existing struct rule. Interfaces already check their dynamic type's
comparability and delegate to its equality. Map insertion/lookup/delete already
use Go equality. Arrays containing unsupported element operations may remain
stuck; this patch does not invent semantics for those operations.

## Checked preparation, with the trusted checkout unchanged

[`ArrayEqualityExpansion.lean`](ArrayEqualityExpansion.lean) defines only the
proposed expression, outside the audited `ProofCheck` root. It adds no step
instance and assumes no array equality or map-key correctness. Lean checks:

- Empty arrays expand to true; a malformed left length selects `AngelicExit`.
- Four arbitrary `uint64` elements compare equal exactly when all four pairs
  are equal. These checks execute the existing element/index operations.
- Two interface arrays with different first integer elements return false
  without comparing their arbitrary second elements.
- A one-element float array delegates to the existing `float64Eq` operation.
  This is not a new proof of IEEE arithmetic.

The three named WP results each report only `propext`, `Classical.choice` and
`Quot.sound` in their axiom lists. Their ambient semantics remain explicit model
parameters. These results validate the proposed RHS; **they do not establish
that the pinned array equality operation takes that step**. They are not added
to the production theorem count.

The zero-context patch passes `git apply --unidiff-zero --check` against the clean pin. The unchanged Go map
witness test passes with `-count=1`. The two existing Lean reproducers still fail
at their documented equality / `SafeMapKey` obligations. The standalone Go
witness below passes all 12 checks, including NaN, signed zero and interface
short-circuit/panic behavior. No tracked Go file was changed.

One correction to the future regression criteria: `MapKeyEquality.lean` currently
asks for a *single step directly to mathematical equality* for unconstrained Lean
arrays. A faithful element-wise rule does not supply that step, and arbitrary
Lean arrays may have the wrong length. Those examples must become multi-step WP
proofs for well-formed arrays after the repair. Passing them unchanged is not an
acceptance criterion. `FeltMap.lean` likewise needs explicit four-limb input facts.

The subsequent `FeltKeyWellFormed`, `FeltMapWellFormed`,
`StorageDiffsWellFormed` and `MigratedClassesWellFormed` modules now derive these
length facts from the wire relations and actual array ownership. They also prove
preservation through duplicate replacements and all completed entries before a
later failure, including nested inner maps and any retained key. They install no
`MapKeys` instance and do not discharge `SafeMapKey`; those integration steps
still belong to the repair work below. Their 28 results are included in the
production dependency audit, unlike the standalone RHS preparation checks.

## Revalidation on the current pin (2026-11-12)

The exact patch still passes `git apply --unidiff-zero --check` against
`9489271c166c7567fb4561f3aed83da866cc921f`. Both `Array.lean` and the sibling
struct-comparison scheme in `PostLang.lean` are unchanged since the original base.
`ArrayEqualityExpansion.lean` checks again, with its three WP results depending
only on `propext`, `Classical.choice` and `Quot.sound`.
`FeltMap.lean` still fails at the expected `SafeMapKey` obligation, and
`MapKeyEquality.lean` still fails to synthesize array/Felt/SierraClassHash equality
steps. The 12-case standalone Go witness and the existing Go map feasibility tests
pass. The Perennial checkout is clean; no part of the patch is installed.

The 14 non-StateUpdate roots now have general API contracts without helper
correctness premises. This rule is needed for the remaining StateUpdate root's
felt-keyed maps, not for those completed contracts. Approval would authorize the
isolated rule implementation, framework regressions and review, safe-key/map
witness proofs, and repinning after validation. Populated map-loop and
StateDiff/StateUpdate API proofs would then continue against that validated pin.

## Work authorized by approval of this proposal

1. Create a separate Perennial worktree from the pinned revision. Apply exactly
   the attached rule/registration patch. Preserve the already-authorized upstream
   slice-index fix in the base; this adds only the separate array-equality rule.
2. Add framework regressions for actual array `GoEquals`: zero-length arrays;
   equal and unequal words at each index; named and generic arrays; nested
   arrays; arrays of structs and structs with arrays; interface short-circuiting
   and a reached uncomparable dynamic value; float delegation. Check malformed
   lengths on both operands and invalid type lengths as model-boundary cases.
   For unsupported operations, verify that they remain stuck rather than adding
   a successful-return proof. Go runtime tests and Lean checks have distinct
   claims; neither substitutes for the other.
3. Prove equality of well-formed four-word arrays by executing the new rule.
   Derive `SafeMapKey` for Felt and SierraClassHash from that proof. Use a
   proof-side `MapKeys` instance whose admissible keys have exactly four limbs,
   following the existing restricted interface-key instance. Integrate that
   instance consistently into felt, storage and migrated map ownership. Obtain
   each produced key's length from the actual limbs decoder/array ownership;
   derive preservation by insertion. Do not add a `SafeMapKey` premise or infer
   key safety by exiting on malformed representations.
4. Prove the real translated `FeltEntries`, `MigratedEntries` and `StorageEntries`
   witnesses for arbitrary well-formed keys, including collisions, missing
   lookups, nil values and nested-map alias updates. Keep the passing resource
   witness as a regression. These are a prerequisite for populated CBOR maps,
   not a substitute for their loop and ownership proofs.
5. Run `etc/lean-ci.sh -q`, inspect `etc/trusted-delta.py` against the base commit,
   and follow `review-trusted` before landing the local repair. Record the
   approved decision in the framework's fix-semantics examples. If validation
   discovers a need for another trusted change, report it separately.
6. Commit the local repair, explicitly repin this proof harness, regenerate and
   run its complete checks and dependency/source audit. Revalidate the frozen
   performance archive and unchanged Go sources. No production behavior change,
   upstream publication or benchmark redesign is included.

Full StateDiff/StateUpdate entry loops, record composition and public APIs remain
work after this prerequisite. Fourteen other roots now have their API helper
dependencies discharged. This proposal does not itself establish
whole-library verification, compiler/runtime verification or fxamacker equivalence.

The explicit approval requirement comes from the active goal and
[`fix-semantics/SKILL.md`](../.tools/perennial/.agents/skills/fix-semantics/SKILL.md),
step 3: “Never change trusted files before the user signs off.”

## Reproduction

From `verification/fastcbor-proof`, with the existing pinned Lean toolchain:

```sh
"$ELAN_HOME/bin/lake" env lean repros/ArrayEqualityExpansion.lean
git -C .tools/perennial apply --unidiff-zero --check ../../repros/array-value-equality.patch
```

From the repository root:

```sh
GOTOOLCHAIN=go1.27.0 GOWORK=off go test -count=1 -short ./verification/fastcbor-proof/feasibility/maps
```

Copy this standalone witness to a temporary `.go` file and run it with
`GOTOOLCHAIN=go1.27.0 GOWORK=off go run /path/to/witness.go`. It does not import or
modify Juno, and its runtime results do not assert that the model is repaired.

```go
package main
import ("fmt"; "math")
type Words [4]uint64
func equal[T comparable](a,b T) bool { return a == b }
func panics(f func()) (p bool) { defer func(){p = recover()!=nil}(); f(); return }
func main() {
 n:=math.NaN(); neg:=math.Copysign(0,-1)
 bad:=any([]int{1})
 checks:=[]struct{name string; ok bool}{
  {"zero_length", [0]uint64{} == [0]uint64{}},
  {"equal_words", Words{1,2,3,4} == Words{1,2,3,4}},
  {"last_limb_differs", Words{1,2,3,4} != Words{1,2,3,5}},
  {"named_generic", equal(Words{1,2,3,4},Words{1,2,3,4})},
  {"nested_arrays", [1]Words{{1,2,3,4}} != [1]Words{{1,2,3,5}}},
  {"array_of_struct", [1]struct{W Words}{{Words{1,2,3,4}}} == [1]struct{W Words}{{Words{1,2,3,4}}}},
  {"struct_with_array", struct{W Words}{Words{1,2,3,4}} != struct{W Words}{Words{1,2,3,5}}},
  {"nan_not_reflexive", [1]float64{n} != [1]float64{n}},
  {"signed_zeros_equal", [1]float64{neg} == [1]float64{0}},
  {"interface_short_circuit", [2]any{uint64(0),bad} != [2]any{uint64(1),bad}},
  {"interface_reached_panics", panics(func(){ _ = [2]any{uint64(0),bad} == [2]any{uint64(0),bad} })},
  {"nan_array_map_misses", func()bool{m:=map[[1]float64]int{}; m[[1]float64{n}]=1; _,ok:=m[[1]float64{n}]; return !ok}()},
 }
 for _,c:=range checks {if !c.ok {panic(c.name)};fmt.Println("PASS",c.name)}
}
```
