import ProofCheck.L2MessagePointersSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

theorem l2MessagePointer_prefix_unique (d : Nat) (left right restA restB : List w8) (a b : Option L2MessageValue)
    (first : L2MessagePointerValid d left restA a) (second : L2MessagePointerValid d right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | nullish suffix hn =>
    cases second with
    | nullish suffix2 hn2 =>
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, rfl⟩ := same
      exact ⟨rfl, rfl, rfl⟩
    | record hn2 hu2 valid2 =>
      rw [← same] at hn2 hu2
      rcases hn with rfl | rfl
      · exact False.elim (hn2 rfl)
      · exact False.elim (hu2 rfl)
  | record hn hu valid =>
    cases second with
    | nullish suffix2 hn2 =>
      rw [same] at hn hu
      rcases hn2 with rfl | rfl
      · exact False.elim (hn rfl)
      · exact False.elim (hu rfl)
    | record hn2 hu2 valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := l2MessageRecord_prefix_unique _ _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem l2MessagePointer_not_rejected (d consumed offset : Nat) (item rest : List w8) (value : Option L2MessageValue)
    (retained : L2MessageValue) (entered : Bool) (reason : L2MessageFailureReason)
    (valid : L2MessagePointerValid d item rest value)
    (failure : L2MessagePointerRejected d (item ++ rest) retained consumed offset entered reason) : False := by
  cases failure with
  | record hn hu failure =>
    cases valid with
    | nullish suffix nullish =>
      rcases nullish with rfl | rfl
      · exact hn rfl
      · exact hu rfl
    | record hn2 hu2 valid => exact l2MessageRecord_not_rejected _ _ _ _ _ _ _ _ _ _ valid failure

theorem l2MessagePointerRejected_unique (d consumedA offsetA consumedB offsetB : Nat) (wire : List w8)
    (a b : L2MessageValue) (enteredA enteredB : Bool) (reasonA reasonB : L2MessageFailureReason)
    (first : L2MessagePointerRejected d wire a consumedA offsetA enteredA reasonA)
    (second : L2MessagePointerRejected d wire b consumedB offsetB enteredB reasonB) :
    a = b ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ enteredA = enteredB ∧ reasonA = reasonB := by
  cases first with
  | record hn hu failure => cases second with
    | record hn2 hu2 failure2 => exact l2MessageRecordRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ _ failure failure2

theorem l2MessagePointerItems_prefix_unique (d n : Nat) (left right restA restB : List w8)
    (a b : List (Option L2MessageValue))
    (first : L2MessagePointerItemsValid d n left restA a) (second : L2MessagePointerItemsValid d n right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  induction first generalizing right restB b with
  | nil => cases second; simpa using same
  | @step n item tail suffix value values valid rest ih =>
    cases second with
    | @step n2 item2 tail2 suffix2 value2 values2 valid2 rest2 =>
      obtain ⟨itemEq, valueEq, tailsEq⟩ := l2MessagePointer_prefix_unique d item item2 (tail ++ suffix) (tail2 ++ restB)
        value value2 valid valid2 (by simpa only [List.append_assoc] using same)
      obtain ⟨tailEq, valuesEq, suffixEq⟩ := ih _ _ _ rest2 tailsEq
      exact ⟨by rw [itemEq, tailEq], by rw [valueEq, valuesEq], suffixEq⟩

theorem l2MessagePointerItems_not_rejected (d n consumed offset : Nat) (item rest : List w8)
    (values done : List (Option L2MessageValue)) (retained : L2MessageValue) (entered : Bool) (reason : L2MessageFailureReason)
    (valid : L2MessagePointerItemsValid d n item rest values)
    (failure : L2MessagePointerItemsRejected d n (item ++ rest) done retained consumed offset entered reason) : False := by
  generalize same : item ++ rest = wire at failure
  induction valid generalizing wire done retained consumed offset entered reason with
  | nil => cases failure
  | @step n item tail suffix value values valid restValid ih =>
    cases failure with
    | here failure =>
      apply l2MessagePointer_not_rejected d consumed offset item (tail ++ suffix) value retained entered reason valid
      rw [← same] at failure
      simpa only [List.append_assoc] using failure
    | @next n2 item2 tail2 value2 done2 retained2 consumed2 offset2 entered2 reason2 valid2 failure2 =>
      obtain ⟨_, _, tailEq⟩ := l2MessagePointer_prefix_unique d item item2 (tail ++ suffix) tail2 value value2 valid valid2
        (by simpa only [List.append_assoc] using same)
      exact ih _ _ _ _ _ _ _ tailEq failure2

/-- Private completed elements, the failed partial L2ToL1Message, and all position/depth
    diagnostics are uniquely determined, even though they are not published. -/
theorem l2MessagePointerItemsRejected_unique (d n consumedA offsetA consumedB offsetB : Nat) (wire : List w8)
    (doneA doneB : List (Option L2MessageValue)) (a b : L2MessageValue) (enteredA enteredB : Bool)
    (reasonA reasonB : L2MessageFailureReason)
    (first : L2MessagePointerItemsRejected d n wire doneA a consumedA offsetA enteredA reasonA)
    (second : L2MessagePointerItemsRejected d n wire doneB b consumedB offsetB enteredB reasonB) :
    doneA = doneB ∧ a = b ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ enteredA = enteredB ∧ reasonA = reasonB := by
  generalize same : wire = wire2 at second
  induction first generalizing wire2 doneB b consumedB offsetB enteredB reasonB with
  | @here n wire retained consumed offset entered reason failure =>
    cases second with
    | here failure2 =>
      rw [← same] at failure2
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := l2MessagePointerRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl, rfl⟩
    | @next n2 item2 tail2 value2 done2 retained2 consumed2 offset2 entered2 reason2 valid2 failure2 =>
      rw [same] at failure
      exact False.elim (l2MessagePointer_not_rejected _ _ _ _ _ _ _ _ _ valid2 failure)
  | @next n item tail value done retained consumed offset entered reason valid failure ih =>
    cases second with
    | here failure2 =>
      rw [← same] at failure2
      exact False.elim (l2MessagePointer_not_rejected _ _ _ _ _ _ _ _ _ valid failure2)
    | @next n2 item2 tail2 value2 done2 retained2 consumed2 offset2 entered2 reason2 valid2 failure2 =>
      obtain ⟨rfl, rfl, rfl⟩ := l2MessagePointer_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      obtain ⟨rfl, rfl, rfl, rfl, rfl, rfl⟩ := ih _ _ _ _ _ _ _ rfl failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessagePointer_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessagePointer_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessagePointerRejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessagePointerItems_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessagePointerItems_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessagePointerItemsRejected_unique
