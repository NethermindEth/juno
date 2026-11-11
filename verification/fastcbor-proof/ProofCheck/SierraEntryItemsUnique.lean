import ProofCheck.SierraEntryValuesSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

theorem sierraEntryItem_prefix_unique (d : Nat) (left right restA restB : List w8) (a b : SierraEntryPointValue)
    (first : SierraEntryItemValid d left restA a) (second : SierraEntryItemValid d right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB :=
  sierraEntryPointRecord_prefix_unique d sierraEntryPointZero a b left right restA restB first second same

theorem sierraEntryItem_not_rejected (d consumed offset : Nat) (item rest : List w8) (value retained : SierraEntryPointValue)
    (entered : Bool) (reason : SierraEntryPointFailureReason)
    (valid : SierraEntryItemValid d item rest value)
    (failure : SierraEntryItemRejected d (item ++ rest) retained consumed offset entered reason) : False :=
  sierraEntryPointRecord_not_rejected d consumed offset sierraEntryPointZero value retained item rest entered reason valid failure

theorem sierraEntryItemRejected_unique (d consumedA offsetA consumedB offsetB : Nat) (wire : List w8)
    (a b : SierraEntryPointValue) (enteredA enteredB : Bool) (reasonA reasonB : SierraEntryPointFailureReason)
    (first : SierraEntryItemRejected d wire a consumedA offsetA enteredA reasonA)
    (second : SierraEntryItemRejected d wire b consumedB offsetB enteredB reasonB) :
    a = b ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ enteredA = enteredB ∧ reasonA = reasonB :=
  sierraEntryPointRecordRejected_unique d consumedA offsetA consumedB offsetB sierraEntryPointZero a b wire
    enteredA enteredB reasonA reasonB first second

theorem sierraEntryItems_prefix_unique (d n : Nat) (left right restA restB : List w8)
    (a b : List SierraEntryPointValue)
    (first : SierraEntryItemsValid d n left restA a) (second : SierraEntryItemsValid d n right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  induction first generalizing right restB b with
  | nil => cases second; simpa using same
  | @step n item tail suffix value values valid rest ih =>
    cases second with
    | @step n2 item2 tail2 suffix2 value2 values2 valid2 rest2 =>
      obtain ⟨itemEq, valueEq, tailsEq⟩ := sierraEntryItem_prefix_unique d item item2 (tail ++ suffix) (tail2 ++ restB)
        value value2 valid valid2 (by simpa only [List.append_assoc] using same)
      obtain ⟨tailEq, valuesEq, suffixEq⟩ := ih _ _ _ rest2 tailsEq
      exact ⟨by rw [itemEq, tailEq], by rw [valueEq, valuesEq], suffixEq⟩

theorem sierraEntryItems_not_rejected (d n consumed offset : Nat) (item rest : List w8)
    (values done : List SierraEntryPointValue) (retained : SierraEntryPointValue) (entered : Bool) (reason : SierraEntryPointFailureReason)
    (valid : SierraEntryItemsValid d n item rest values)
    (failure : SierraEntryItemsRejected d n (item ++ rest) done retained consumed offset entered reason) : False := by
  generalize same : item ++ rest = wire at failure
  induction valid generalizing wire done retained consumed offset entered reason with
  | nil => cases failure
  | @step n item tail suffix value values valid restValid ih =>
    cases failure with
    | here failure =>
      apply sierraEntryItem_not_rejected d consumed offset item (tail ++ suffix) value retained entered reason valid
      rw [← same] at failure
      simpa only [List.append_assoc] using failure
    | @next n2 item2 tail2 value2 done2 retained2 consumed2 offset2 entered2 reason2 valid2 failure2 =>
      obtain ⟨_, _, tailEq⟩ := sierraEntryItem_prefix_unique d item item2 (tail ++ suffix) tail2 value value2 valid valid2
        (by simpa only [List.append_assoc] using same)
      exact ih _ _ _ _ _ _ _ tailEq failure2

/-- Private completed elements, the failed partial SierraEntryPoint, and all position/depth
    diagnostics are uniquely determined, even though they are not published. -/
theorem sierraEntryItemsRejected_unique (d n consumedA offsetA consumedB offsetB : Nat) (wire : List w8)
    (doneA doneB : List SierraEntryPointValue) (a b : SierraEntryPointValue) (enteredA enteredB : Bool)
    (reasonA reasonB : SierraEntryPointFailureReason)
    (first : SierraEntryItemsRejected d n wire doneA a consumedA offsetA enteredA reasonA)
    (second : SierraEntryItemsRejected d n wire doneB b consumedB offsetB enteredB reasonB) :
    doneA = doneB ∧ a = b ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ enteredA = enteredB ∧ reasonA = reasonB := by
  generalize same : wire = wire2 at second
  induction first generalizing wire2 doneB b consumedB offsetB enteredB reasonB with
  | @here n wire retained consumed offset entered reason failure =>
    cases second with
    | here failure2 =>
      rw [← same] at failure2
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := sierraEntryItemRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl, rfl⟩
    | @next n2 item2 tail2 value2 done2 retained2 consumed2 offset2 entered2 reason2 valid2 failure2 =>
      rw [same] at failure
      exact False.elim (sierraEntryItem_not_rejected _ _ _ _ _ _ _ _ _ valid2 failure)
  | @next n item tail value done retained consumed offset entered reason valid failure ih =>
    cases second with
    | here failure2 =>
      rw [← same] at failure2
      exact False.elim (sierraEntryItem_not_rejected _ _ _ _ _ _ _ _ _ valid failure2)
    | @next n2 item2 tail2 value2 done2 retained2 consumed2 offset2 entered2 reason2 valid2 failure2 =>
      obtain ⟨rfl, rfl, rfl⟩ := sierraEntryItem_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      obtain ⟨rfl, rfl, rfl, rfl, rfl, rfl⟩ := ih _ _ _ _ _ _ _ rfl failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryItem_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryItem_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryItemRejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryItems_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryItems_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryItemsRejected_unique
