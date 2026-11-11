import ProofCheck.DeployTable
import ProofCheck.DeployPairs

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
theorem deployKeyMasks_disjoint (a b : DeployField) :
    ((W64 1 <<< deployKeyIndex a) &&& (W64 1 <<< deployKeyIndex b) = W64 0) ↔ a ≠ b := by
  cases a <;> cases b <;> decide

/-- Marking a key removes exactly that field from the fresh set. Unknown keys
    leave the set unchanged; no unrelated field becomes seen. -/
theorem deployKeyFresh_mark_iff (seen : w64) (choice : Option DeployField) (field : DeployField) :
    DeployKeyFresh (deployKeySeen seen choice) (some field) ↔
      DeployKeyFresh seen (some field) ∧ choice ≠ some field := by
  cases choice with
  | none => simp [deployKeySeen, DeployKeyFresh]
  | some found =>
    have hor (a b : w64) : a ||| b = W64 0 ↔ a = W64 0 ∧ b = W64 0 := BitVec.or_eq_zero_iff
    simp only [deployKeySeen, DeployKeyFresh, BitVec.and_or_distrib_right, hor,
      deployKeyMasks_disjoint, ne_eq, Option.some.injEq]

/-- Seen bits correspond to schema identities, independently of key spelling,
    cached lookup order, or which entries were unknown keys. -/
def DeploySeen (seen : w64) (fields : List DeployField) : Prop :=
  ∀ field, DeployKeyFresh seen (some field) ↔ field ∉ fields

theorem deploySeen_zero : DeploySeen (W64 0) [] := by
  intro field
  simp [DeployKeyFresh, BitVec.zero_and]

theorem deploySeen_mark (seen : w64) (fields : List DeployField) (choice : Option DeployField)
    (hs : DeploySeen seen fields) :
    DeploySeen (deployKeySeen seen choice) (match choice with | none => fields | some field => field :: fields) := by
  intro field
  rw [deployKeyFresh_mark_iff, hs field]
  cases choice <;> simp [List.mem_cons, and_comm, eq_comm]

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.deployKeyMasks_disjoint
#print axioms Perennial.github_com.NethermindEth.juno.core.deployKeyFresh_mark_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.deploySeen_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.deploySeen_mark
