import ProofCheck.BuiltinsTable
import ProofCheck.BuiltinsPairs

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
theorem builtinsKeyMasks_disjoint (a b : BuiltinsField) :
    ((W64 1 <<< builtinsKeyIndex a) &&& (W64 1 <<< builtinsKeyIndex b) = W64 0) ↔ a ≠ b := by
  cases a <;> cases b <;> decide

/-- Marking a key removes exactly that field from the fresh set. Unknown keys
    leave the set unchanged; no unrelated field becomes seen. -/
theorem builtinsKeyFresh_mark_iff (seen : w64) (choice : Option BuiltinsField) (field : BuiltinsField) :
    BuiltinsKeyFresh (builtinsKeySeen seen choice) (some field) ↔
      BuiltinsKeyFresh seen (some field) ∧ choice ≠ some field := by
  cases choice with
  | none => simp [builtinsKeySeen, BuiltinsKeyFresh]
  | some found =>
    have hor (a b : w64) : a ||| b = W64 0 ↔ a = W64 0 ∧ b = W64 0 := BitVec.or_eq_zero_iff
    simp only [builtinsKeySeen, BuiltinsKeyFresh, BitVec.and_or_distrib_right, hor,
      builtinsKeyMasks_disjoint, ne_eq, Option.some.injEq]

/-- Seen bits correspond to schema identities, independently of key spelling,
    cached lookup order, or which entries were unknown keys. -/
def BuiltinsSeen (seen : w64) (fields : List BuiltinsField) : Prop :=
  ∀ field, BuiltinsKeyFresh seen (some field) ↔ field ∉ fields

theorem builtinsSeen_zero : BuiltinsSeen (W64 0) [] := by
  intro field
  simp [BuiltinsKeyFresh, BitVec.zero_and]

theorem builtinsSeen_mark (seen : w64) (fields : List BuiltinsField) (choice : Option BuiltinsField)
    (hs : BuiltinsSeen seen fields) :
    BuiltinsSeen (builtinsKeySeen seen choice) (match choice with | none => fields | some field => field :: fields) := by
  intro field
  rw [builtinsKeyFresh_mark_iff, hs field]
  cases choice <;> simp [List.mem_cons, and_comm, eq_comm]

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsKeyMasks_disjoint
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsKeyFresh_mark_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsSeen_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsSeen_mark
