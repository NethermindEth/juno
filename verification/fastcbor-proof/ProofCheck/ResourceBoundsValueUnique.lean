import ProofCheck.ResourceBoundsPairs
import ProofCheck.SkipFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Selected fields and ignored values have a unique boundary and result on
    the same complete input, retaining the actual unread suffix. -/
theorem resourceBoundsValue_prefix_unique (d : Nat) (choice : Option ResourceBoundsField) (old a b : ResourceBoundsValue)
    (left right restA restB : List w8)
    (first : ResourceBoundsValueValid d choice old left restA a)
    (second : ResourceBoundsValueValid d choice old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | selected valid => cases second with
    | selected valid2 => exact resourceBoundsField_prefix_unique _ _ _ _ _ _ _ _ _ valid valid2 same
  | ignored valid => cases second with
    | ignored valid2 =>
      obtain ⟨rfl, rfl⟩ := skipValid_prefix_unique _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem resourceBoundsValue_not_rejected (d offset : Nat) (choice : Option ResourceBoundsField) (old final : ResourceBoundsValue)
    (item rest : List w8) (reason : ResourceBoundsValueFailure)
    (valid : ResourceBoundsValueValid d choice old item rest final)
    (failure : ResourceBoundsValueRejected d choice (item ++ rest) offset reason) : False := by
  cases valid with
  | selected valid => cases failure with
    | selected failure => exact resourceBoundsField_not_rejected _ _ _ _ _ _ _ _ valid failure
  | ignored valid => cases failure with
    | ignored failure => exact skipValid_not_failure _ _ _ _ _ valid failure

theorem resourceBoundsValueRejected_unique (d offsetA offsetB : Nat) (choice : Option ResourceBoundsField) (wire : List w8)
    (a b : ResourceBoundsValueFailure) (first : ResourceBoundsValueRejected d choice wire offsetA a)
    (second : ResourceBoundsValueRejected d choice wire offsetB b) : offsetA = offsetB ∧ a = b := by
  cases first with
  | selected failure => cases second with
    | selected failure2 =>
      obtain ⟨rfl, rfl⟩ := resourceBoundsField_rejected_unique _ _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl⟩
  | ignored failure => cases second with
    | ignored failure2 =>
      obtain ⟨rfl, rfl⟩ := skipFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsValue_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsValue_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsValueRejected_unique
