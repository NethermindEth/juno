import ProofCheck.DeployPairs
import ProofCheck.SkipFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Selected fields and ignored values have a unique boundary and result on
    the same complete input; contextual slice guards retain the actual suffix. -/
theorem deployValue_prefix_unique (d : Nat) (choice : Option DeployField) (old a b : DeployValue)
    (left right restA restB : List w8)
    (first : DeployValueValid d choice old left restA a)
    (second : DeployValueValid d choice old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | selected valid => cases second with
    | selected valid2 => exact deployField_prefix_unique _ _ _ _ _ _ _ _ _ valid valid2 same
  | ignored valid => cases second with
    | ignored valid2 =>
      obtain ⟨rfl, rfl⟩ := skipValid_prefix_unique _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem deployValue_not_rejected (d offset : Nat) (choice : Option DeployField) (old final : DeployValue)
    (item rest : List w8) (reason : DeployValueFailure)
    (valid : DeployValueValid d choice old item rest final)
    (failure : DeployValueRejected d choice (item ++ rest) offset reason) : False := by
  cases valid with
  | selected valid => cases failure with
    | selected failure => exact deployField_not_rejected _ _ _ _ _ _ _ _ valid failure
  | ignored valid => cases failure with
    | ignored failure => exact skipValid_not_failure _ _ _ _ _ valid failure

theorem deployValueRejected_unique (d offsetA offsetB : Nat) (choice : Option DeployField) (wire : List w8)
    (a b : DeployValueFailure) (first : DeployValueRejected d choice wire offsetA a)
    (second : DeployValueRejected d choice wire offsetB b) : offsetA = offsetB ∧ a = b := by
  cases first with
  | selected failure => cases second with
    | selected failure2 =>
      obtain ⟨rfl, rfl⟩ := deployField_rejected_unique _ _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl⟩
  | ignored failure => cases second with
    | ignored failure2 =>
      obtain ⟨rfl, rfl⟩ := skipFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.deployValue_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.deployValue_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.deployValueRejected_unique
