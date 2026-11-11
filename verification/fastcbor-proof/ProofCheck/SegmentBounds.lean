import ProofCheck.SegmentUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- The diagnostic is at or after the committed cursor and within the input;
    retained recursive entries stay within the shared nesting limit. -/
def SegmentRecordFailureBounded (d : Nat) : Prop :=
  ∀ (consumed offset extraDepth : Nat) (old final : SegmentValue) (wire : List w8)
    (reason : SegmentFailureReason),
    SegmentRecordRejected d old wire final consumed offset extraDepth reason →
    consumed ≤ offset ∧ offset ≤ wire.length ∧ d + extraDepth ≤ 32

def SegmentArrayFailureBounded (d : Nat) : Prop :=
  ∀ (consumed offset extraDepth : Nat) (wire : List w8) (reason : SegmentArrayFailureReason),
    SegmentArraySpec d wire (.failure consumed offset extraDepth reason) →
    consumed ≤ offset ∧ offset ≤ wire.length ∧ d + extraDepth ≤ 32

theorem segmentField_rejected_bounds_of_array (d : Nat) (depth : d ≤ 32)
    (arrays : SegmentArrayFailureBounded d) (field : SegmentField) (wire : List w8)
    (consumed offset extraDepth : Nat) (reason : SegmentFieldFailure)
    (failure : SegmentFieldRejected d field wire consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length ∧ d + extraDepth ≤ 32 := by
  cases failure with
  | children failure => exact arrays _ _ _ _ _ failure
  | length failure => omega

theorem segmentValueRejected_bounds_of_array (d : Nat) (depth : d ≤ 32)
    (arrays : SegmentArrayFailureBounded d) (choice : Option SegmentField)
    (old final : SegmentValue) (wire : List w8) (consumed offset extraDepth : Nat)
    (reason : SegmentValueFailure)
    (failure : SegmentValueRejected d choice old wire final consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length ∧ d + extraDepth ≤ 32 := by
  cases failure with
  | selected failure => exact segmentField_rejected_bounds_of_array d depth arrays _ _ _ _ _ _ failure
  | ignored failure =>
    have bound := skipFailure_offset _ _ _ _ failure
    omega

theorem segmentPairsRejected_bounds_of_array (d : Nat) (depth : d ≤ 32)
    (arrays : SegmentArrayFailureBounded d) (n : Nat) (seen : List SegmentField)
    (old final : SegmentValue) (wire : List w8) (consumed offset extraDepth : Nat)
    (reason : SegmentFailureReason)
    (failure : SegmentPairsRejected d n seen old wire final consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length ∧ d + extraDepth ≤ 32 := by
  induction n generalizing seen old final wire consumed offset extraDepth reason with
  | zero => cases failure
  | succ n ih =>
    cases failure with
    | key => omega
    | duplicate => simp only [List.length_append]; omega
    | value keyValid fresh failure =>
      have bounds := segmentValueRejected_bounds_of_array d depth arrays _ _ _ _ _ _ _ _ failure
      simp only [List.length_append]; omega
    | next keyValid fresh valueValid failure =>
      have bounds := ih _ _ _ _ _ _ _ _ failure
      simp only [List.length_append]; omega


theorem segmentRecord_bounded_of_array (d : Nat) (depth : d ≤ 32)
    (next : d < 32 → SegmentArrayFailureBounded (d + 1)) : SegmentRecordFailureBounded d := by
  intro consumed offset extraDepth old final wire reason failure
  cases failure with
  | opening => omega
  | map enc major size inside fit failure =>
    have bounds := segmentPairsRejected_bounds_of_array (d + 1) (by omega) (next inside) _ _ _ _ _ _ _ _ _ failure
    simp only [List.length_append, List.length_cons]; omega

theorem segmentItemsRejected_bounds_of_record (d : Nat) (records : SegmentRecordFailureBounded d)
    (n : Nat) (wire : List w8) (done : List SegmentValue) (retained : SegmentValue)
    (consumed offset extraDepth : Nat) (reason : SegmentFailureReason)
    (failure : SegmentItemsRejected d n wire done retained consumed offset extraDepth reason) :
    done.length < n ∧ consumed ≤ offset ∧ offset ≤ wire.length ∧ d + extraDepth ≤ 32 := by
  induction n generalizing wire done retained consumed offset extraDepth reason with
  | zero => cases failure
  | succ n ih =>
    cases failure with
    | here failure =>
      have bounds := records _ _ _ _ _ _ _ failure
      simp only [List.length_nil]; omega
    | next valid failure =>
      have bounds := ih _ _ _ _ _ _ _ failure
      simp only [List.length_cons, List.length_append]; omega


theorem segmentArray_bounded_of_record (d : Nat) (depth : d ≤ 32)
    (next : d < 32 → SegmentRecordFailureBounded (d + 1)) : SegmentArrayFailureBounded d := by
  intro consumed offset extraDepth wire reason spec
  cases spec with
  | outer => omega
  | count => omega
  | depth => omega
  | element enc major size fit inside failure =>
    have bounds := segmentItemsRejected_bounds_of_record (d + 1) (next inside) _ _ _ _ _ _ _ _ failure
    simp only [List.length_append, List.length_cons]; omega

/-- Close both recursive failure-bound obligations without a child-contract
    assumption. Count failures precede depth failures, and both roll back. -/
theorem segment_failure_bounds_depth (fuel : Nat) : ∀ d : Nat, d ≤ 32 → 32 - d = fuel →
    SegmentRecordFailureBounded d ∧ SegmentArrayFailureBounded d := by
  induction fuel with
  | zero =>
    intro d depth remaining
    have impossible : ¬ d < 32 := by omega
    exact ⟨segmentRecord_bounded_of_array d depth (fun inside => False.elim (impossible inside)),
      segmentArray_bounded_of_record d depth (fun inside => False.elim (impossible inside))⟩
  | succ fuel ih =>
    intro d depth remaining
    have next : d < 32 → SegmentRecordFailureBounded (d + 1) ∧ SegmentArrayFailureBounded (d + 1) := by
      intro inside
      exact ih (d + 1) (by omega) (by omega)
    exact ⟨segmentRecord_bounded_of_array d depth (fun inside => (next inside).2),
      segmentArray_bounded_of_record d depth (fun inside => (next inside).1)⟩

theorem segmentRecordRejected_bounds (d : Nat) (depth : d ≤ 32)
    (consumed offset extraDepth : Nat) (old final : SegmentValue) (wire : List w8)
    (reason : SegmentFailureReason)
    (failure : SegmentRecordRejected d old wire final consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length ∧ d + extraDepth ≤ 32 :=
  (segment_failure_bounds_depth (32 - d) d depth rfl).1 _ _ _ _ _ _ _ failure

theorem segmentArray_failure_bounds (d : Nat) (depth : d ≤ 32)
    (consumed offset extraDepth : Nat) (wire : List w8) (reason : SegmentArrayFailureReason)
    (spec : SegmentArraySpec d wire (.failure consumed offset extraDepth reason)) :
    consumed ≤ offset ∧ offset ≤ wire.length ∧ d + extraDepth ≤ 32 :=
  (segment_failure_bounds_depth (32 - d) d depth rfl).2 _ _ _ _ _ spec

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentField_rejected_bounds_of_array
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentValueRejected_bounds_of_array
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentPairsRejected_bounds_of_array
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentRecord_bounded_of_array
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentItemsRejected_bounds_of_record
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentArray_bounded_of_record
#print axioms Perennial.github_com.NethermindEth.juno.core.segment_failure_bounds_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentRecordRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentArray_failure_bounds
