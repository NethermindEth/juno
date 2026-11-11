import ProofCheck.DeprecatedEntryPointTable
import ProofCheck.DeprecatedEntryPointPairs

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Distinct schema identities use disjoint nonzero bits in the actual mask. -/
theorem deprecatedEntryPointKeyMasks_disjoint (a b : DeprecatedEntryPointField) :
    ((W64 1 <<< deprecatedEntryPointKeyIndex a) &&& (W64 1 <<< deprecatedEntryPointKeyIndex b) = W64 0) ↔ a ≠ b := by
  cases a <;> cases b <;> decide

/-- Marking a key removes exactly that field from the fresh set. Unknown keys
    leave the set unchanged; no unrelated field becomes seen. -/
theorem deprecatedEntryPointKeyFresh_mark_iff (seen : w64) (choice : Option DeprecatedEntryPointField) (field : DeprecatedEntryPointField) :
    DeprecatedEntryPointKeyFresh (deprecatedEntryPointKeySeen seen choice) (some field) ↔
      DeprecatedEntryPointKeyFresh seen (some field) ∧ choice ≠ some field := by
  cases choice with
  | none => simp [deprecatedEntryPointKeySeen, DeprecatedEntryPointKeyFresh]
  | some found =>
    have hor (a b : w64) : a ||| b = W64 0 ↔ a = W64 0 ∧ b = W64 0 := BitVec.or_eq_zero_iff
    simp only [deprecatedEntryPointKeySeen, DeprecatedEntryPointKeyFresh, BitVec.and_or_distrib_right, hor,
      deprecatedEntryPointKeyMasks_disjoint, ne_eq, Option.some.injEq]

/-- Seen bits correspond to schema identities, independently of key spelling,
    cached lookup order, or which entries were unknown keys. -/
def DeprecatedEntryPointSeen (seen : w64) (fields : List DeprecatedEntryPointField) : Prop :=
  ∀ field, DeprecatedEntryPointKeyFresh seen (some field) ↔ field ∉ fields

theorem deprecatedEntryPointSeen_zero : DeprecatedEntryPointSeen (W64 0) [] := by
  intro field
  simp [DeprecatedEntryPointKeyFresh, BitVec.zero_and]

theorem deprecatedEntryPointSeen_mark (seen : w64) (fields : List DeprecatedEntryPointField) (choice : Option DeprecatedEntryPointField)
    (hs : DeprecatedEntryPointSeen seen fields) :
    DeprecatedEntryPointSeen (deprecatedEntryPointKeySeen seen choice) (match choice with | none => fields | some field => field :: fields) := by
  intro field
  rw [deprecatedEntryPointKeyFresh_mark_iff, hs field]
  cases choice <;> simp [List.mem_cons, and_comm, eq_comm]

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointKeyMasks_disjoint
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointKeyFresh_mark_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointSeen_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointSeen_mark
