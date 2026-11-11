import ProofCheck.StringsItemsSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

theorem stringsItems_prefix_unique (n : Nat) (left right restA restB : List w8)
    (a b : List GoString) (first : StringsItemsValid n left a) (second : StringsItemsValid n right b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  induction first generalizing right restA restB b with
  | nil => cases second; simpa using same
  | @step n item tail value values valid rest ih =>
    cases second with
    | @step n2 item2 tail2 value2 values2 valid2 rest2 =>
      obtain ⟨itemEq, valueEq, tailsEq⟩ := stringValid_prefix_unique [] value value2 item item2
        (tail ++ restA) (tail2 ++ restB) valid valid2 (by simpa only [List.append_assoc] using same)
      obtain ⟨tailEq, valuesEq, suffixEq⟩ := ih _ _ _ _ rest2 tailsEq
      exact ⟨by rw [itemEq, tailEq], by rw [valueEq, valuesEq], suffixEq⟩

theorem stringsItems_not_rejected (n offset : Nat) (item rest : List w8)
    (values done : List GoString) (reason : StringFailureReason)
    (valid : StringsItemsValid n item values)
    (failure : StringsItemsRejected n (item ++ rest) done offset reason) : False := by
  generalize same : item ++ rest = wire at failure
  induction valid generalizing rest wire done offset reason with
  | nil => cases failure
  | @step n item tail value values valid restValid ih =>
    cases failure with
    | here failure =>
      apply stringValid_not_rejected [] value item (tail ++ rest) reason valid
      rw [← same] at failure
      simpa only [List.append_assoc] using failure
    | @next n2 item2 tail2 value2 done2 offset2 reason2 valid2 failure2 =>
      obtain ⟨_, _, tailEq⟩ := stringValid_prefix_unique [] value value2 item item2 (tail ++ rest) tail2 valid valid2
        (by simpa only [List.append_assoc] using same)
      exact ih _ _ _ _ _ tailEq failure2

/-- The unpublished completed prefix and the error position/reason are unique. -/
theorem stringsItemsRejected_unique (n offsetA offsetB : Nat) (wire : List w8)
    (doneA doneB : List GoString) (reasonA reasonB : StringFailureReason)
    (first : StringsItemsRejected n wire doneA offsetA reasonA)
    (second : StringsItemsRejected n wire doneB offsetB reasonB) :
    doneA = doneB ∧ offsetA = offsetB ∧ reasonA = reasonB := by
  generalize same : wire = wire2 at second
  induction first generalizing wire2 doneB offsetB reasonB with
  | @here n wire reason failure =>
    cases second with
    | here failure2 =>
      rw [← same] at failure2
      have eq := stringRejected_reason_unique _ _ _ failure failure2
      exact ⟨rfl, rfl, eq⟩
    | @next n2 item2 tail2 value2 done2 offset2 reason2 valid2 failure2 =>
      rw [same] at failure
      exact False.elim (stringValid_not_rejected _ _ _ _ _ valid2 failure)
  | @next n item tail value done offset reason valid failure ih =>
    cases second with
    | here failure2 =>
      rw [← same] at failure2
      exact False.elim (stringValid_not_rejected _ _ _ _ _ valid failure2)
    | @next n2 item2 tail2 value2 done2 offset2 reason2 valid2 failure2 =>
      obtain ⟨rfl, rfl, rfl⟩ := stringValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      obtain ⟨rfl, rfl, rfl⟩ := ih _ _ _ _ rfl failure2
      exact ⟨rfl, rfl, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.stringsItems_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.stringsItems_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.stringsItemsRejected_unique
