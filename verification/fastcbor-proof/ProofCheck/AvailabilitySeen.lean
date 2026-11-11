import ProofCheck.AvailabilityTable
import ProofCheck.AvailabilityPairs

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
theorem availabilityKeyMasks_disjoint (a b : AvailabilityField) :
    ((W64 1 <<< availabilityKeyIndex a) &&& (W64 1 <<< availabilityKeyIndex b) = W64 0) ↔ a ≠ b := by
  cases a <;> cases b <;> decide

/-- Marking a key removes exactly that field from the fresh set. Unknown keys
    leave the set unchanged; no unrelated field becomes seen. -/
theorem availabilityKeyFresh_mark_iff (seen : w64) (choice : Option AvailabilityField) (field : AvailabilityField) :
    AvailabilityKeyFresh (availabilityKeySeen seen choice) (some field) ↔
      AvailabilityKeyFresh seen (some field) ∧ choice ≠ some field := by
  cases choice with
  | none => simp [availabilityKeySeen, AvailabilityKeyFresh]
  | some found =>
    have hor (a b : w64) : a ||| b = W64 0 ↔ a = W64 0 ∧ b = W64 0 := BitVec.or_eq_zero_iff
    simp only [availabilityKeySeen, AvailabilityKeyFresh, BitVec.and_or_distrib_right, hor,
      availabilityKeyMasks_disjoint, ne_eq, Option.some.injEq]

/-- Seen bits correspond to schema identities, independently of key spelling,
    cached lookup order, or which entries were unknown keys. -/
def AvailabilitySeen (seen : w64) (fields : List AvailabilityField) : Prop :=
  ∀ field, AvailabilityKeyFresh seen (some field) ↔ field ∉ fields

theorem availabilitySeen_zero : AvailabilitySeen (W64 0) [] := by
  intro field
  simp [AvailabilityKeyFresh, BitVec.zero_and]

theorem availabilitySeen_mark (seen : w64) (fields : List AvailabilityField) (choice : Option AvailabilityField)
    (hs : AvailabilitySeen seen fields) :
    AvailabilitySeen (availabilityKeySeen seen choice) (match choice with | none => fields | some field => field :: fields) := by
  intro field
  rw [availabilityKeyFresh_mark_iff, hs field]
  cases choice <;> simp [List.mem_cons, and_comm, eq_comm]

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilityKeyMasks_disjoint
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilityKeyFresh_mark_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilitySeen_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilitySeen_mark
