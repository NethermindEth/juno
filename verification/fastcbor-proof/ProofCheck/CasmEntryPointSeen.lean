import ProofCheck.CasmEntryPointTable
import ProofCheck.CasmEntryPointPairs

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
theorem casmEntryPointKeyMasks_disjoint (a b : CasmEntryPointField) :
    ((W64 1 <<< casmEntryPointKeyIndex a) &&& (W64 1 <<< casmEntryPointKeyIndex b) = W64 0) ↔ a ≠ b := by
  cases a <;> cases b <;> decide

/-- Marking a key removes exactly that field from the fresh set. Unknown keys
    leave the set unchanged; no unrelated field becomes seen. -/
theorem casmEntryPointKeyFresh_mark_iff (seen : w64) (choice : Option CasmEntryPointField) (field : CasmEntryPointField) :
    CasmEntryPointKeyFresh (casmEntryPointKeySeen seen choice) (some field) ↔
      CasmEntryPointKeyFresh seen (some field) ∧ choice ≠ some field := by
  cases choice with
  | none => simp [casmEntryPointKeySeen, CasmEntryPointKeyFresh]
  | some found =>
    have hor (a b : w64) : a ||| b = W64 0 ↔ a = W64 0 ∧ b = W64 0 := BitVec.or_eq_zero_iff
    simp only [casmEntryPointKeySeen, CasmEntryPointKeyFresh, BitVec.and_or_distrib_right, hor,
      casmEntryPointKeyMasks_disjoint, ne_eq, Option.some.injEq]

/-- Seen bits correspond to schema identities, independently of key spelling,
    cached lookup order, or which entries were unknown keys. -/
def CasmEntryPointSeen (seen : w64) (fields : List CasmEntryPointField) : Prop :=
  ∀ field, CasmEntryPointKeyFresh seen (some field) ↔ field ∉ fields

theorem casmEntryPointSeen_zero : CasmEntryPointSeen (W64 0) [] := by
  intro field
  simp [CasmEntryPointKeyFresh, BitVec.zero_and]

theorem casmEntryPointSeen_mark (seen : w64) (fields : List CasmEntryPointField) (choice : Option CasmEntryPointField)
    (hs : CasmEntryPointSeen seen fields) :
    CasmEntryPointSeen (casmEntryPointKeySeen seen choice) (match choice with | none => fields | some field => field :: fields) := by
  intro field
  rw [casmEntryPointKeyFresh_mark_iff, hs field]
  cases choice <;> simp [List.mem_cons, and_comm, eq_comm]

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointKeyMasks_disjoint
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointKeyFresh_mark_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointSeen_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointSeen_mark
