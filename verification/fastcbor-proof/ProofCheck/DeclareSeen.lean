import ProofCheck.DeclareTable
import ProofCheck.DeclarePairs

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
theorem declareKeyMasks_disjoint (a b : DeclareField) :
    ((W64 1 <<< declareKeyIndex a) &&& (W64 1 <<< declareKeyIndex b) = W64 0) ↔ a ≠ b := by
  cases a <;> cases b <;> decide

/-- Marking a key removes exactly that field from the fresh set. Unknown keys
    leave the set unchanged; no unrelated field becomes seen. -/
theorem declareKeyFresh_mark_iff (seen : w64) (choice : Option DeclareField) (field : DeclareField) :
    DeclareKeyFresh (declareKeySeen seen choice) (some field) ↔
      DeclareKeyFresh seen (some field) ∧ choice ≠ some field := by
  cases choice with
  | none => simp [declareKeySeen, DeclareKeyFresh]
  | some found =>
    have hor (a b : w64) : a ||| b = W64 0 ↔ a = W64 0 ∧ b = W64 0 := BitVec.or_eq_zero_iff
    simp only [declareKeySeen, DeclareKeyFresh, BitVec.and_or_distrib_right, hor,
      declareKeyMasks_disjoint, ne_eq, Option.some.injEq]

/-- Seen bits correspond to schema identities, independently of key spelling,
    cached lookup order, or which entries were unknown keys. -/
def DeclareSeen (seen : w64) (fields : List DeclareField) : Prop :=
  ∀ field, DeclareKeyFresh seen (some field) ↔ field ∉ fields

theorem declareSeen_zero : DeclareSeen (W64 0) [] := by
  intro field
  simp [DeclareKeyFresh, BitVec.zero_and]

theorem declareSeen_mark (seen : w64) (fields : List DeclareField) (choice : Option DeclareField)
    (hs : DeclareSeen seen fields) :
    DeclareSeen (declareKeySeen seen choice) (match choice with | none => fields | some field => field :: fields) := by
  intro field
  rw [declareKeyFresh_mark_iff, hs field]
  cases choice <;> simp [List.mem_cons, and_comm, eq_comm]

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.declareKeyMasks_disjoint
#print axioms Perennial.github_com.NethermindEth.juno.core.declareKeyFresh_mark_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.declareSeen_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.declareSeen_mark
