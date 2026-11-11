import ProofCheck.L1MessageTable
import ProofCheck.L1MessagePairs

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
theorem l1MessageKeyMasks_disjoint (a b : L1MessageField) :
    ((W64 1 <<< l1MessageKeyIndex a) &&& (W64 1 <<< l1MessageKeyIndex b) = W64 0) ↔ a ≠ b := by
  cases a <;> cases b <;> decide

/-- Marking a key removes exactly that field from the fresh set. Unknown keys
    leave the set unchanged; no unrelated field becomes seen. -/
theorem l1MessageKeyFresh_mark_iff (seen : w64) (choice : Option L1MessageField) (field : L1MessageField) :
    L1MessageKeyFresh (l1MessageKeySeen seen choice) (some field) ↔
      L1MessageKeyFresh seen (some field) ∧ choice ≠ some field := by
  cases choice with
  | none => simp [l1MessageKeySeen, L1MessageKeyFresh]
  | some found =>
    have hor (a b : w64) : a ||| b = W64 0 ↔ a = W64 0 ∧ b = W64 0 := BitVec.or_eq_zero_iff
    simp only [l1MessageKeySeen, L1MessageKeyFresh, BitVec.and_or_distrib_right, hor,
      l1MessageKeyMasks_disjoint, ne_eq, Option.some.injEq]

/-- Seen bits correspond to schema identities, independently of key spelling,
    cached lookup order, or which entries were unknown keys. -/
def L1MessageSeen (seen : w64) (fields : List L1MessageField) : Prop :=
  ∀ field, L1MessageKeyFresh seen (some field) ↔ field ∉ fields

theorem l1MessageSeen_zero : L1MessageSeen (W64 0) [] := by
  intro field
  simp [L1MessageKeyFresh, BitVec.zero_and]

theorem l1MessageSeen_mark (seen : w64) (fields : List L1MessageField) (choice : Option L1MessageField)
    (hs : L1MessageSeen seen fields) :
    L1MessageSeen (l1MessageKeySeen seen choice) (match choice with | none => fields | some field => field :: fields) := by
  intro field
  rw [l1MessageKeyFresh_mark_iff, hs field]
  cases choice <;> simp [List.mem_cons, and_comm, eq_comm]

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageKeyMasks_disjoint
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageKeyFresh_mark_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageSeen_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageSeen_mark
