import ProofCheck.SegmentClassify

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

theorem segmentRecord_nonempty (d : Nat) (old final : SegmentValue) (item rest : List w8)
    (valid : SegmentRecordValid d old item rest final) : 0 < item.length := by
  cases valid <;> simp

theorem segmentField_nonempty (d : Nat) (field : SegmentField) (old final : SegmentValue)
    (item rest : List w8) (valid : SegmentFieldValid d field old item rest final) : 0 < item.length := by
  cases valid with
  | children valid =>
    obtain ⟨other, same, positive⟩ := segmentArray_success_suffix d (item ++ rest) rest _ valid
    have eq := List.append_cancel_right same
    subst other
    exact positive
  | length valid => cases valid <;> simp

theorem segmentValue_nonempty (d : Nat) (choice : Option SegmentField) (old final : SegmentValue)
    (item rest : List w8) (valid : SegmentValueValid d choice old item rest final) : 0 < item.length := by
  cases valid with
  | selected valid => exact segmentField_nonempty _ _ _ _ _ _ valid
  | ignored valid => exact skipValid_nonempty _ _ valid

/-- Each map pair consumes a nonempty key and a nonempty value, including
    recursively decoded Children and ignored unknown fields. -/
theorem segmentPairs_count_bound (d n : Nat) (seen : List SegmentField) (old final : SegmentValue)
    (item rest : List w8) (valid : SegmentPairsValid d n seen old item rest final) : 2 * n ≤ item.length := by
  induction n generalizing seen old final item rest with
  | zero => omega
  | succ n ih =>
    cases valid with
    | step keyValid fresh valueValid rest =>
      have hk := schemaKey_nonempty SegmentKeyMeaning _ _ keyValid
      have hv := segmentValue_nonempty d _ _ _ _ _ valueValid
      have bound := ih _ _ _ _ _ rest
      simp only [List.length_append]
      omega

/-- Every successful array has exactly its declared number of embedded records;
    each record consumes at least one byte even when it is nullish. -/
theorem segmentItems_count (d n : Nat) (item rest : List w8) (values : List SegmentValue)
    (valid : SegmentItemsValid d n item rest values) : values.length = n ∧ n ≤ item.length := by
  induction n generalizing item rest values with
  | zero => cases valid; simp
  | succ n ih =>
    cases valid with
    | step valid rest =>
      have nonempty := segmentRecord_nonempty _ _ _ _ _ valid
      have bound := ih _ _ _ rest
      simp only [List.length_cons, List.length_append]
      omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentRecord_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentField_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentValue_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentPairs_count_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentItems_count
