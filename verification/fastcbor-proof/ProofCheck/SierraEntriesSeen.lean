import ProofCheck.SierraEntriesTable
import ProofCheck.SierraEntriesPairs

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
theorem sierraEntriesKeyMasks_disjoint (a b : SierraEntriesField) :
    ((W64 1 <<< sierraEntriesKeyIndex a) &&& (W64 1 <<< sierraEntriesKeyIndex b) = W64 0) ↔ a ≠ b := by
  cases a <;> cases b <;> decide

/-- Marking a key removes exactly that field from the fresh set. Unknown keys
    leave the set unchanged; no unrelated field becomes seen. -/
theorem sierraEntriesKeyFresh_mark_iff (seen : w64) (choice : Option SierraEntriesField) (field : SierraEntriesField) :
    SierraEntriesKeyFresh (sierraEntriesKeySeen seen choice) (some field) ↔
      SierraEntriesKeyFresh seen (some field) ∧ choice ≠ some field := by
  cases choice with
  | none => simp [sierraEntriesKeySeen, SierraEntriesKeyFresh]
  | some found =>
    have hor (a b : w64) : a ||| b = W64 0 ↔ a = W64 0 ∧ b = W64 0 := BitVec.or_eq_zero_iff
    simp only [sierraEntriesKeySeen, SierraEntriesKeyFresh, BitVec.and_or_distrib_right, hor,
      sierraEntriesKeyMasks_disjoint, ne_eq, Option.some.injEq]

/-- Seen bits correspond to schema identities, independently of key spelling,
    cached lookup order, or which entries were unknown keys. -/
def SierraEntriesSeen (seen : w64) (fields : List SierraEntriesField) : Prop :=
  ∀ field, SierraEntriesKeyFresh seen (some field) ↔ field ∉ fields

theorem sierraEntriesSeen_zero : SierraEntriesSeen (W64 0) [] := by
  intro field
  simp [SierraEntriesKeyFresh, BitVec.zero_and]

theorem sierraEntriesSeen_mark (seen : w64) (fields : List SierraEntriesField) (choice : Option SierraEntriesField)
    (hs : SierraEntriesSeen seen fields) :
    SierraEntriesSeen (sierraEntriesKeySeen seen choice) (match choice with | none => fields | some field => field :: fields) := by
  intro field
  rw [sierraEntriesKeyFresh_mark_iff, hs field]
  cases choice <;> simp [List.mem_cons, and_comm, eq_comm]

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntriesKeyMasks_disjoint
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntriesKeyFresh_mark_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntriesSeen_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntriesSeen_mark
