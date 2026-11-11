import ProofCheck.SegmentUniqueContracts

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

abbrev SegmentItemValid (d : Nat) (item rest : List w8) (value : SegmentValue) :=
  SegmentRecordValid d segmentZero item rest value
abbrev SegmentItemRejected (d : Nat) (wire : List w8) (retained : SegmentValue)
    (consumed offset childDepth : Nat) (reason : SegmentFailureReason) :=
  SegmentRecordRejected d segmentZero wire retained consumed offset childDepth reason

theorem segmentItem_prefix_unique_of_record (d : Nat) (records : SegmentRecordDeterministic d) (left right restA restB : List w8) (a b : SegmentValue)
    (first : SegmentItemValid d left restA a) (second : SegmentItemValid d right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB :=
  records.successful segmentZero a b left right restA restB first second same

theorem segmentItem_not_rejected_of_record (d : Nat) (records : SegmentRecordDeterministic d) (consumed offset : Nat) (item rest : List w8) (value retained : SegmentValue)
    (childDepth : Nat) (reason : SegmentFailureReason)
    (valid : SegmentRecordValid d segmentZero item rest value)
    (failure : SegmentItemRejected d (item ++ rest) retained consumed offset childDepth reason) : False :=
  records.disjoint consumed offset segmentZero value retained item rest childDepth reason valid failure

theorem segmentItemRejected_unique_of_record (d : Nat) (records : SegmentRecordDeterministic d) (consumedA offsetA consumedB offsetB : Nat) (wire : List w8)
    (a b : SegmentValue) (depthA depthB : Nat) (reasonA reasonB : SegmentFailureReason)
    (first : SegmentItemRejected d wire a consumedA offsetA depthA reasonA)
    (second : SegmentItemRejected d wire b consumedB offsetB depthB reasonB) :
    a = b ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ reasonA = reasonB :=
  records.rejected consumedA offsetA consumedB offsetB segmentZero a b wire
    depthA depthB reasonA reasonB first second

theorem segmentItems_prefix_unique_of_record (d : Nat) (records : SegmentRecordDeterministic d) (n : Nat) (left right restA restB : List w8)
    (a b : List SegmentValue)
    (first : SegmentItemsValid d n left restA a) (second : SegmentItemsValid d n right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | nil => cases second; simpa using same
  | @step _ n item tail suffix value values valid rest =>
    cases second with
    | @step _ n2 item2 tail2 suffix2 value2 values2 valid2 rest2 =>
      obtain ⟨itemEq, valueEq, tailsEq⟩ := segmentItem_prefix_unique_of_record d records item item2 (tail ++ restA) (tail2 ++ restB)
        value value2 valid valid2 (by simpa only [List.append_assoc] using same)
      obtain ⟨tailEq, valuesEq, suffixEq⟩ := segmentItems_prefix_unique_of_record d records n _ _ _ _ _ _ rest rest2 tailsEq
      exact ⟨by rw [itemEq, tailEq], by rw [valueEq, valuesEq], suffixEq⟩
termination_by n

theorem segmentItems_not_rejected_of_record (d : Nat) (records : SegmentRecordDeterministic d) (n consumed offset : Nat) (item rest : List w8)
    (values done : List SegmentValue) (retained : SegmentValue) (childDepth : Nat) (reason : SegmentFailureReason)
    (valid : SegmentItemsValid d n item rest values)
    (failure : SegmentItemsRejected d n (item ++ rest) done retained consumed offset childDepth reason) : False := by
  generalize same : item ++ rest = wire at failure
  cases valid with
  | nil => cases failure
  | @step _ n item tail suffix value values valid restValid =>
    cases failure with
    | here failure =>
      apply segmentItem_not_rejected_of_record d records consumed offset item (tail ++ rest) value retained childDepth reason valid
      rw [← same] at failure
      simpa only [List.append_assoc] using failure
    | @next _ n2 item2 tail2 value2 done2 retained2 consumed2 offset2 depth2 reason2 valid2 failure2 =>
      obtain ⟨_, _, tailEq⟩ := segmentItem_prefix_unique_of_record d records item item2 (tail ++ rest) tail2 value value2 valid valid2
        (by simpa only [List.append_assoc] using same)
      exact segmentItems_not_rejected_of_record d records n _ _ _ _ _ _ _ _ _ restValid (by rw [tailEq]; exact failure2)
termination_by n

/-- Private completed elements, the failed partial Segment, and all position/depth
    diagnostics are uniquely determined, even though they are not published. -/
theorem segmentItemsRejected_unique_of_record (d : Nat) (records : SegmentRecordDeterministic d) (n consumedA offsetA consumedB offsetB : Nat) (wire : List w8)
    (doneA doneB : List SegmentValue) (a b : SegmentValue) (depthA depthB : Nat)
    (reasonA reasonB : SegmentFailureReason)
    (first : SegmentItemsRejected d n wire doneA a consumedA offsetA depthA reasonA)
    (second : SegmentItemsRejected d n wire doneB b consumedB offsetB depthB reasonB) :
    doneA = doneB ∧ a = b ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ reasonA = reasonB := by
  generalize same : wire = wire2 at second
  cases first with
  | @here _ n wire retained consumed offset childDepth reason failure =>
    cases second with
    | here failure2 =>
      rw [← same] at failure2
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := segmentItemRejected_unique_of_record _ records _ _ _ _ _ _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl, rfl⟩
    | @next _ n2 item2 tail2 value2 done2 retained2 consumed2 offset2 depth2 reason2 valid2 failure2 =>
      rw [same] at failure
      exact False.elim (segmentItem_not_rejected_of_record _ records _ _ _ _ _ _ _ _ valid2 failure)
  | @next _ n item tail value done retained consumed offset childDepth reason valid failure =>
    cases second with
    | here failure2 =>
      rw [← same] at failure2
      exact False.elim (segmentItem_not_rejected_of_record _ records _ _ _ _ _ _ _ _ valid failure2)
    | @next _ n2 item2 tail2 value2 done2 retained2 consumed2 offset2 depth2 reason2 valid2 failure2 =>
      obtain ⟨rfl, rfl, rfl⟩ := segmentItem_prefix_unique_of_record _ records _ _ _ _ _ _ valid valid2 same
      obtain ⟨rfl, rfl, rfl, rfl, rfl, rfl⟩ := segmentItemsRejected_unique_of_record d records n _ _ _ _ _ _ _ _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl, rfl⟩
termination_by n


end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentItem_prefix_unique_of_record
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentItem_not_rejected_of_record
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentItemRejected_unique_of_record
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentItems_prefix_unique_of_record
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentItems_not_rejected_of_record
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentItemsRejected_unique_of_record
