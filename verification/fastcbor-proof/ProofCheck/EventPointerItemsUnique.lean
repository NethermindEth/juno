import ProofCheck.EventPointersSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

theorem eventPointer_prefix_unique (d : Nat) (left right restA restB : List w8) (a b : Option EventValue)
    (first : EventPointerValid d left restA a) (second : EventPointerValid d right restB b)
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
      obtain ⟨rfl, rfl, rfl⟩ := eventRecord_prefix_unique _ _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem eventPointer_not_rejected (d consumed offset : Nat) (item rest : List w8) (value : Option EventValue)
    (retained : EventValue) (entered : Bool) (reason : EventFailureReason)
    (valid : EventPointerValid d item rest value)
    (failure : EventPointerRejected d (item ++ rest) retained consumed offset entered reason) : False := by
  cases failure with
  | record hn hu failure =>
    cases valid with
    | nullish suffix nullish =>
      rcases nullish with rfl | rfl
      · exact hn rfl
      · exact hu rfl
    | record hn2 hu2 valid => exact eventRecord_not_rejected _ _ _ _ _ _ _ _ _ _ valid failure

theorem eventPointerRejected_unique (d consumedA offsetA consumedB offsetB : Nat) (wire : List w8)
    (a b : EventValue) (enteredA enteredB : Bool) (reasonA reasonB : EventFailureReason)
    (first : EventPointerRejected d wire a consumedA offsetA enteredA reasonA)
    (second : EventPointerRejected d wire b consumedB offsetB enteredB reasonB) :
    a = b ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ enteredA = enteredB ∧ reasonA = reasonB := by
  cases first with
  | record hn hu failure => cases second with
    | record hn2 hu2 failure2 => exact eventRecordRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ _ failure failure2

theorem eventPointerItems_prefix_unique (d n : Nat) (left right restA restB : List w8)
    (a b : List (Option EventValue))
    (first : EventPointerItemsValid d n left restA a) (second : EventPointerItemsValid d n right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  induction first generalizing right restB b with
  | nil => cases second; simpa using same
  | @step n item tail suffix value values valid rest ih =>
    cases second with
    | @step n2 item2 tail2 suffix2 value2 values2 valid2 rest2 =>
      obtain ⟨itemEq, valueEq, tailsEq⟩ := eventPointer_prefix_unique d item item2 (tail ++ suffix) (tail2 ++ restB)
        value value2 valid valid2 (by simpa only [List.append_assoc] using same)
      obtain ⟨tailEq, valuesEq, suffixEq⟩ := ih _ _ _ rest2 tailsEq
      exact ⟨by rw [itemEq, tailEq], by rw [valueEq, valuesEq], suffixEq⟩

theorem eventPointerItems_not_rejected (d n consumed offset : Nat) (item rest : List w8)
    (values done : List (Option EventValue)) (retained : EventValue) (entered : Bool) (reason : EventFailureReason)
    (valid : EventPointerItemsValid d n item rest values)
    (failure : EventPointerItemsRejected d n (item ++ rest) done retained consumed offset entered reason) : False := by
  generalize same : item ++ rest = wire at failure
  induction valid generalizing wire done retained consumed offset entered reason with
  | nil => cases failure
  | @step n item tail suffix value values valid restValid ih =>
    cases failure with
    | here failure =>
      apply eventPointer_not_rejected d consumed offset item (tail ++ suffix) value retained entered reason valid
      rw [← same] at failure
      simpa only [List.append_assoc] using failure
    | @next n2 item2 tail2 value2 done2 retained2 consumed2 offset2 entered2 reason2 valid2 failure2 =>
      obtain ⟨_, _, tailEq⟩ := eventPointer_prefix_unique d item item2 (tail ++ suffix) tail2 value value2 valid valid2
        (by simpa only [List.append_assoc] using same)
      exact ih _ _ _ _ _ _ _ tailEq failure2

/-- Private completed elements, the failed partial Event, and all position/depth
    diagnostics are uniquely determined, even though they are not published. -/
theorem eventPointerItemsRejected_unique (d n consumedA offsetA consumedB offsetB : Nat) (wire : List w8)
    (doneA doneB : List (Option EventValue)) (a b : EventValue) (enteredA enteredB : Bool)
    (reasonA reasonB : EventFailureReason)
    (first : EventPointerItemsRejected d n wire doneA a consumedA offsetA enteredA reasonA)
    (second : EventPointerItemsRejected d n wire doneB b consumedB offsetB enteredB reasonB) :
    doneA = doneB ∧ a = b ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ enteredA = enteredB ∧ reasonA = reasonB := by
  generalize same : wire = wire2 at second
  induction first generalizing wire2 doneB b consumedB offsetB enteredB reasonB with
  | @here n wire retained consumed offset entered reason failure =>
    cases second with
    | here failure2 =>
      rw [← same] at failure2
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := eventPointerRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl, rfl⟩
    | @next n2 item2 tail2 value2 done2 retained2 consumed2 offset2 entered2 reason2 valid2 failure2 =>
      rw [same] at failure
      exact False.elim (eventPointer_not_rejected _ _ _ _ _ _ _ _ _ valid2 failure)
  | @next n item tail value done retained consumed offset entered reason valid failure ih =>
    cases second with
    | here failure2 =>
      rw [← same] at failure2
      exact False.elim (eventPointer_not_rejected _ _ _ _ _ _ _ _ _ valid failure2)
    | @next n2 item2 tail2 value2 done2 retained2 consumed2 offset2 entered2 reason2 valid2 failure2 =>
      obtain ⟨rfl, rfl, rfl⟩ := eventPointer_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      obtain ⟨rfl, rfl, rfl, rfl, rfl, rfl⟩ := ih _ _ _ _ _ _ _ rfl failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.eventPointer_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.eventPointer_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.eventPointerRejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.eventPointerItems_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.eventPointerItems_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.eventPointerItemsRejected_unique
