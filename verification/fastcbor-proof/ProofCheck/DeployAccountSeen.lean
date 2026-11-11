import ProofCheck.DeployAccountTable
import ProofCheck.DeployAccountPairs

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
theorem deployAccountKeyMasks_disjoint (a b : DeployAccountField) :
    ((W64 1 <<< deployAccountKeyIndex a) &&& (W64 1 <<< deployAccountKeyIndex b) = W64 0) ↔ a ≠ b := by
  cases a <;> cases b <;> decide

/-- Marking a key removes exactly that field from the fresh set. Unknown keys
    leave the set unchanged; no unrelated field becomes seen. -/
theorem deployAccountKeyFresh_mark_iff (seen : w64) (choice : Option DeployAccountField) (field : DeployAccountField) :
    DeployAccountKeyFresh (deployAccountKeySeen seen choice) (some field) ↔
      DeployAccountKeyFresh seen (some field) ∧ choice ≠ some field := by
  cases choice with
  | none => simp [deployAccountKeySeen, DeployAccountKeyFresh]
  | some found =>
    have hor (a b : w64) : a ||| b = W64 0 ↔ a = W64 0 ∧ b = W64 0 := BitVec.or_eq_zero_iff
    simp only [deployAccountKeySeen, DeployAccountKeyFresh, BitVec.and_or_distrib_right, hor,
      deployAccountKeyMasks_disjoint, ne_eq, Option.some.injEq]

/-- Seen bits correspond to schema identities, independently of key spelling,
    cached lookup order, or which entries were unknown keys. -/
def DeployAccountSeen (seen : w64) (fields : List DeployAccountField) : Prop :=
  ∀ field, DeployAccountKeyFresh seen (some field) ↔ field ∉ fields

theorem deployAccountSeen_zero : DeployAccountSeen (W64 0) [] := by
  intro field
  simp [DeployAccountKeyFresh, BitVec.zero_and]

theorem deployAccountSeen_mark (seen : w64) (fields : List DeployAccountField) (choice : Option DeployAccountField)
    (hs : DeployAccountSeen seen fields) :
    DeployAccountSeen (deployAccountKeySeen seen choice) (match choice with | none => fields | some field => field :: fields) := by
  intro field
  rw [deployAccountKeyFresh_mark_iff, hs field]
  cases choice <;> simp [List.mem_cons, and_comm, eq_comm]

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountKeyMasks_disjoint
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountKeyFresh_mark_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountSeen_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountSeen_mark
