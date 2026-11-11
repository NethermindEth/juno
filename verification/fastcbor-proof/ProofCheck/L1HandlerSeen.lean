import ProofCheck.L1HandlerTable
import ProofCheck.L1HandlerPairs

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
theorem l1HandlerKeyMasks_disjoint (a b : L1HandlerField) :
    ((W64 1 <<< l1HandlerKeyIndex a) &&& (W64 1 <<< l1HandlerKeyIndex b) = W64 0) ↔ a ≠ b := by
  cases a <;> cases b <;> decide

/-- Marking a key removes exactly that field from the fresh set. Unknown keys
    leave the set unchanged; no unrelated field becomes seen. -/
theorem l1HandlerKeyFresh_mark_iff (seen : w64) (choice : Option L1HandlerField) (field : L1HandlerField) :
    L1HandlerKeyFresh (l1HandlerKeySeen seen choice) (some field) ↔
      L1HandlerKeyFresh seen (some field) ∧ choice ≠ some field := by
  cases choice with
  | none => simp [l1HandlerKeySeen, L1HandlerKeyFresh]
  | some found =>
    have hor (a b : w64) : a ||| b = W64 0 ↔ a = W64 0 ∧ b = W64 0 := BitVec.or_eq_zero_iff
    simp only [l1HandlerKeySeen, L1HandlerKeyFresh, BitVec.and_or_distrib_right, hor,
      l1HandlerKeyMasks_disjoint, ne_eq, Option.some.injEq]

/-- Seen bits correspond to schema identities, independently of key spelling,
    cached lookup order, or which entries were unknown keys. -/
def L1HandlerSeen (seen : w64) (fields : List L1HandlerField) : Prop :=
  ∀ field, L1HandlerKeyFresh seen (some field) ↔ field ∉ fields

theorem l1HandlerSeen_zero : L1HandlerSeen (W64 0) [] := by
  intro field
  simp [L1HandlerKeyFresh, BitVec.zero_and]

theorem l1HandlerSeen_mark (seen : w64) (fields : List L1HandlerField) (choice : Option L1HandlerField)
    (hs : L1HandlerSeen seen fields) :
    L1HandlerSeen (l1HandlerKeySeen seen choice) (match choice with | none => fields | some field => field :: fields) := by
  intro field
  rw [l1HandlerKeyFresh_mark_iff, hs field]
  cases choice <;> simp [List.mem_cons, and_comm, eq_comm]

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerKeyMasks_disjoint
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerKeyFresh_mark_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerSeen_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerSeen_mark
