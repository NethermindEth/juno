import ProofCheck.ResourceBoundsTable
import ProofCheck.ResourceBoundsPairs

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
theorem resourceBoundsKeyMasks_disjoint (a b : ResourceBoundsField) :
    ((W64 1 <<< resourceBoundsKeyIndex a) &&& (W64 1 <<< resourceBoundsKeyIndex b) = W64 0) ↔ a ≠ b := by
  cases a <;> cases b <;> decide

/-- Marking a key removes exactly that field from the fresh set. Unknown keys
    leave the set unchanged; no unrelated field becomes seen. -/
theorem resourceBoundsKeyFresh_mark_iff (seen : w64) (choice : Option ResourceBoundsField) (field : ResourceBoundsField) :
    ResourceBoundsKeyFresh (resourceBoundsKeySeen seen choice) (some field) ↔
      ResourceBoundsKeyFresh seen (some field) ∧ choice ≠ some field := by
  cases choice with
  | none => simp [resourceBoundsKeySeen, ResourceBoundsKeyFresh]
  | some found =>
    have hor (a b : w64) : a ||| b = W64 0 ↔ a = W64 0 ∧ b = W64 0 := BitVec.or_eq_zero_iff
    simp only [resourceBoundsKeySeen, ResourceBoundsKeyFresh, BitVec.and_or_distrib_right, hor,
      resourceBoundsKeyMasks_disjoint, ne_eq, Option.some.injEq]

/-- Seen bits correspond to schema identities, independently of key spelling,
    cached lookup order, or which entries were unknown keys. -/
def ResourceBoundsSeen (seen : w64) (fields : List ResourceBoundsField) : Prop :=
  ∀ field, ResourceBoundsKeyFresh seen (some field) ↔ field ∉ fields

theorem resourceBoundsSeen_zero : ResourceBoundsSeen (W64 0) [] := by
  intro field
  simp [ResourceBoundsKeyFresh, BitVec.zero_and]

theorem resourceBoundsSeen_mark (seen : w64) (fields : List ResourceBoundsField) (choice : Option ResourceBoundsField)
    (hs : ResourceBoundsSeen seen fields) :
    ResourceBoundsSeen (resourceBoundsKeySeen seen choice) (match choice with | none => fields | some field => field :: fields) := by
  intro field
  rw [resourceBoundsKeyFresh_mark_iff, hs field]
  cases choice <;> simp [List.mem_cons, and_comm, eq_comm]

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsKeyMasks_disjoint
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsKeyFresh_mark_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsSeen_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsSeen_mark
