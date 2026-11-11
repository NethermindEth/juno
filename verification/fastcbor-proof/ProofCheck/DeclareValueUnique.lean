import ProofCheck.DeclarePairs
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
theorem declareValue_prefix_unique (d : Nat) (choice : Option DeclareField) (old a b : DeclareValue)
    (left right restA restB : List w8)
    (first : DeclareValueValid d choice old left restA a)
    (second : DeclareValueValid d choice old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | selected valid => cases second with
    | selected valid2 => exact declareField_prefix_unique _ _ _ _ _ _ _ _ _ valid valid2 same
  | ignored valid => cases second with
    | ignored valid2 =>
      obtain ⟨rfl, rfl⟩ := skipValid_prefix_unique _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem declareValue_not_rejected (d consumed offset extraDepth : Nat) (choice : Option DeclareField)
    (old final retained : DeclareValue) (item rest : List w8) (reason : DeclareValueFailure)
    (valid : DeclareValueValid d choice old item rest final)
    (failure : DeclareValueRejected d choice old (item ++ rest) retained consumed offset extraDepth reason) : False := by
  cases valid with
  | selected valid => cases failure with
    | selected failure => exact declareField_not_rejected _ _ _ _ _ _ _ _ _ _ valid failure
  | ignored valid => cases failure with
    | ignored failure => exact skipValid_not_failure _ _ _ _ _ valid failure

theorem declareValueRejected_unique (d consumedA offsetA depthA consumedB offsetB depthB : Nat)
    (choice : Option DeclareField) (old finalA finalB : DeclareValue) (wire : List w8)
    (a b : DeclareValueFailure)
    (first : DeclareValueRejected d choice old wire finalA consumedA offsetA depthA a)
    (second : DeclareValueRejected d choice old wire finalB consumedB offsetB depthB b) :
    finalA = finalB ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b := by
  cases first with
  | selected failure => cases second with
    | selected failure2 =>
      obtain ⟨rfl, rfl, rfl, rfl⟩ := declareField_rejected_unique _ _ _ _ _ _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩
  | ignored failure => cases second with
    | ignored failure2 =>
      obtain ⟨rfl, rfl⟩ := skipFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.declareValue_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.declareValue_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.declareValueRejected_unique
