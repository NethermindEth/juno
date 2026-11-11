import ProofCheck.StateUpdatePairs
import ProofCheck.SkipFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Selected fields and ignored values have a unique boundary and result on
    the same complete input; contextual count guards retain the actual suffix. -/
theorem stateUpdateValue_prefix_unique (d : Nat) (choice : Option StateUpdateField) (old a b : StateUpdateValue)
    (left right restA restB : List w8)
    (first : StateUpdateValueValid d choice old left restA a)
    (second : StateUpdateValueValid d choice old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | selected valid => cases second with
    | selected valid2 => exact stateUpdateField_prefix_unique _ _ _ _ _ _ _ _ _ valid valid2 same
  | ignored valid => cases second with
    | ignored valid2 =>
      obtain ⟨rfl, rfl⟩ := skipValid_prefix_unique _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem stateUpdateValue_not_rejected (d consumed offset extraDepth : Nat) (choice : Option StateUpdateField)
    (old final retained : StateUpdateValue) (item rest : List w8) (reason : StateUpdateValueFailure)
    (valid : StateUpdateValueValid d choice old item rest final)
    (failure : StateUpdateValueRejected d choice old (item ++ rest) retained consumed offset extraDepth reason) (bound : (item ++ rest).length < 2^64) : False := by
  cases valid with
  | selected valid => cases failure with
    | selected failure => exact stateUpdateField_not_rejected _ _ _ _ _ _ _ _ _ _ valid failure bound
  | ignored valid => cases failure with
    | ignored failure => exact skipValid_not_failure _ _ _ _ _ valid failure

theorem stateUpdateValueRejected_unique (d consumedA offsetA depthA consumedB offsetB depthB : Nat)
    (choice : Option StateUpdateField) (old finalA finalB : StateUpdateValue) (wire : List w8)
    (a b : StateUpdateValueFailure)
    (first : StateUpdateValueRejected d choice old wire finalA consumedA offsetA depthA a)
    (second : StateUpdateValueRejected d choice old wire finalB consumedB offsetB depthB b) (bound : wire.length < 2^64) :
    finalA = finalB ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b := by
  cases first with
  | selected failure => cases second with
    | selected failure2 =>
      obtain ⟨rfl, rfl, rfl, rfl⟩ := stateUpdateField_rejected_unique _ _ _ _ _ _ _ _ _ _ _ failure failure2 bound
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩
  | ignored failure => cases second with
    | ignored failure2 =>
      obtain ⟨rfl, rfl⟩ := skipFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateValue_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateValue_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateValueRejected_unique
