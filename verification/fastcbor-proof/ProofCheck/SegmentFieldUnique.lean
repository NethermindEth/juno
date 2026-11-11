import ProofCheck.SegmentUniqueContracts

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

theorem segmentField_prefix_unique_of_array (d : Nat) (arrays : SegmentArrayDeterministic d)
    (field : SegmentField) (old a b : SegmentValue) (left right restA restB : List w8)
    (first : SegmentFieldValid d field old left restA a)
    (second : SegmentFieldValid d field old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | children valid => cases second with
    | children valid2 =>
      rw [← same] at valid2
      have eq := arrays _ _ _ valid valid2
      cases eq
      exact ⟨List.append_cancel_right same, rfl, rfl⟩
  | length valid => cases second with
    | length valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := scalarValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem segmentField_not_rejected_of_array (d : Nat) (arrays : SegmentArrayDeterministic d)
    (consumed offset extraDepth : Nat) (field : SegmentField) (old final : SegmentValue)
    (item rest : List w8) (reason : SegmentFieldFailure)
    (valid : SegmentFieldValid d field old item rest final)
    (failure : SegmentFieldRejected d field (item ++ rest) consumed offset extraDepth reason) : False := by
  cases valid with
  | children valid => cases failure with
    | children failure => have eq := arrays _ _ _ valid failure; cases eq
  | length valid => cases failure with
    | length failure => exact scalarValid_not_failure _ _ _ _ _ valid failure

theorem segmentField_rejected_unique_of_array (d : Nat) (arrays : SegmentArrayDeterministic d)
    (consumedA offsetA depthA consumedB offsetB depthB : Nat) (field : SegmentField)
    (wire : List w8) (a b : SegmentFieldFailure)
    (first : SegmentFieldRejected d field wire consumedA offsetA depthA a)
    (second : SegmentFieldRejected d field wire consumedB offsetB depthB b) :
    consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b := by
  cases first with
  | children failure => cases second with
    | children failure2 =>
      have eq := arrays _ _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl⟩
  | length failure => cases second with
    | length failure2 =>
      have eq := scalarFailure_reason_unique _ _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl⟩

theorem segmentValue_prefix_unique_of_array (d : Nat) (arrays : SegmentArrayDeterministic d) (choice : Option SegmentField) (old a b : SegmentValue)
    (left right restA restB : List w8)
    (first : SegmentValueValid d choice old left restA a)
    (second : SegmentValueValid d choice old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | selected valid => cases second with
    | selected valid2 => exact segmentField_prefix_unique_of_array _ arrays _ _ _ _ _ _ _ _ valid valid2 same
  | ignored valid => cases second with
    | ignored valid2 =>
      obtain ⟨rfl, rfl⟩ := skipValid_prefix_unique _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem segmentValue_not_rejected_of_array (d : Nat) (arrays : SegmentArrayDeterministic d) (consumed offset extraDepth : Nat) (choice : Option SegmentField)
    (old final retained : SegmentValue) (item rest : List w8) (reason : SegmentValueFailure)
    (valid : SegmentValueValid d choice old item rest final)
    (failure : SegmentValueRejected d choice old (item ++ rest) retained consumed offset extraDepth reason) : False := by
  cases valid with
  | selected valid => cases failure with
    | selected failure => exact segmentField_not_rejected_of_array _ arrays _ _ _ _ _ _ _ _ _ valid failure
  | ignored valid => cases failure with
    | ignored failure => exact skipValid_not_failure _ _ _ _ _ valid failure

theorem segmentValueRejected_unique_of_array (d : Nat) (arrays : SegmentArrayDeterministic d) (consumedA offsetA depthA consumedB offsetB depthB : Nat)
    (choice : Option SegmentField) (old finalA finalB : SegmentValue) (wire : List w8)
    (a b : SegmentValueFailure)
    (first : SegmentValueRejected d choice old wire finalA consumedA offsetA depthA a)
    (second : SegmentValueRejected d choice old wire finalB consumedB offsetB depthB b) :
    finalA = finalB ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b := by
  cases first with
  | selected failure => cases second with
    | selected failure2 =>
      obtain ⟨rfl, rfl, rfl, rfl⟩ := segmentField_rejected_unique_of_array _ arrays _ _ _ _ _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩
  | ignored failure => cases second with
    | ignored failure2 =>
      obtain ⟨rfl, rfl⟩ := skipFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentField_prefix_unique_of_array
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentField_not_rejected_of_array
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentField_rejected_unique_of_array
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentValue_prefix_unique_of_array
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentValue_not_rejected_of_array
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentValueRejected_unique_of_array
