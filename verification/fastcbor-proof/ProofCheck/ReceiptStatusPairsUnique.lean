import ProofCheck.ReceiptStatusItemUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

theorem receiptStatusKey_prefix_unique
    (left right restA restB : List w8) (a b : Option Nat)
    (first : WantedKeyValid receiptStatusNames left a)
    (second : WantedKeyValid receiptStatusNames right b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  exact schemaKey_prefix_unique (WantedKeyMeaning receiptStatusNames)
    (fun key a b => wantedKeyMeaning_unique _ key a b (projectionNames_distinct .receiptExecutionStatus))
    left right restA restB a b first second same

/-- Successful wire-order pairs have exactly one decoded value and suffix. -/
theorem receiptStatusPairs_prefix_unique (d n : Nat)
    (seen : List Nat) (old a b : ReceiptStatusValue) (left right restA restB : List w8)
    (first : ReceiptStatusPairsValid d n seen old left a)
    (second : ReceiptStatusPairsValid d n seen old right b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  induction first generalizing right b restA restB with
  | nil => cases second; simpa using same
  | @step n seen choice old middle final key value tail keyValid valueValid rest ih =>
    cases second with
    | @step n2 seen2 choice2 old2 middle2 final2 key2 value2 tail2 keyValid2 valueValid2 rest2 =>
      obtain ⟨keyEq, choiceEq, valuesEq⟩ := receiptStatusKey_prefix_unique key key2
        (value ++ tail ++ restA) (value2 ++ tail2 ++ restB) choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using same)
      subst choice2
      obtain ⟨valueEq, middleEq, tailsEq⟩ := receiptStatusItem_prefix_unique d seen choice old middle middle2
        value value2 (tail ++ restA) (tail2 ++ restB) valueValid valueValid2
        (by simpa only [List.append_assoc] using valuesEq)
      subst middle2
      obtain ⟨tailEq, finalEq, suffixEq⟩ := ih _ _ _ _ rest2 tailsEq
      exact ⟨by rw [keyEq, valueEq, tailEq], finalEq, suffixEq⟩

theorem receiptStatusPairs_not_rejected (d n consumed offset : Nat)
    (seen : List Nat) (old final retained : ReceiptStatusValue) (item rest : List w8) (reason : ReceiptStatusFailure)
    (valid : ReceiptStatusPairsValid d n seen old item final)
    (failure : ReceiptStatusPairsRejected d n seen old (item ++ rest) retained consumed offset reason) : False := by
  generalize same : item ++ rest = wire at failure
  induction valid generalizing wire rest retained consumed offset reason with
  | nil => cases failure
  | @step n seen choice old middle final key value tail keyValid valueValid restValid ih =>
    cases failure with
    | key rejected =>
      apply schemaKey_not_rejected (WantedKeyMeaning receiptStatusNames) key (value ++ tail ++ rest) choice _ keyValid
      rw [← same] at rejected
      simpa only [List.append_assoc] using rejected
    | @value n2 seen2 choice2 old2 key2 tail2 offset2 reason2 valid2 rejected =>
      obtain ⟨_, choiceEq, valuesEq⟩ := receiptStatusKey_prefix_unique key key2
        (value ++ tail ++ rest) tail2 choice choice2 keyValid valid2
        (by simpa only [List.append_assoc] using same)
      subst choice2; subst tail2
      exact receiptStatusItem_not_rejected d offset2 seen choice old middle value (tail ++ rest) _ valueValid
        (by simpa only [List.append_assoc] using rejected)
    | @next n2 seen2 choice2 old2 middle2 final2 key2 value2 tail2 consumed2 offset2 reason2 valid2 valueValid2 rejected =>
      obtain ⟨_, choiceEq, valuesEq⟩ := receiptStatusKey_prefix_unique key key2
        (value ++ tail ++ rest) (value2 ++ tail2) choice choice2 keyValid valid2
        (by simpa only [List.append_assoc] using same)
      subst choice2
      obtain ⟨_, middleEq, tailsEq⟩ := receiptStatusItem_prefix_unique d seen choice old middle middle2
        value value2 (tail ++ rest) tail2 valueValid valueValid2
        (by simpa only [List.append_assoc] using valuesEq)
      subst middle2
      exact ih _ _ _ _ _ _ tailsEq rejected

/-- Rejection fixes the retained fields as well as both error positions and the
    failure reason, even after arbitrarily many successful preceding pairs. -/
theorem receiptStatusPairsRejected_unique (d n consumedA offsetA consumedB offsetB : Nat)
    (seen : List Nat) (old finalA finalB : ReceiptStatusValue) (wire : List w8) (a b : ReceiptStatusFailure)
    (first : ReceiptStatusPairsRejected d n seen old wire finalA consumedA offsetA a)
    (second : ReceiptStatusPairsRejected d n seen old wire finalB consumedB offsetB b) :
    finalA = finalB ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ a = b := by
  generalize same : wire = wire2 at second
  induction first generalizing wire2 finalB consumedB offsetB b with
  | @key n seen old wire reason failure =>
    cases second with
    | key failure2 =>
      rw [← same] at failure2
      exact ⟨rfl, rfl, rfl, congrArg ReceiptStatusFailure.key (keyRejected_reason_unique _ _ _ failure failure2)⟩
    | @value n2 seen2 choice2 old2 key2 tail2 offset2 reason2 valid2 failure2 =>
      exact False.elim (schemaKey_not_rejected (WantedKeyMeaning receiptStatusNames) key2 tail2 choice2 _ valid2
        (by rw [same] at failure; exact failure))
    | @next n2 seen2 choice2 old2 middle2 final2 key2 value2 tail2 consumed2 offset2 reason2 valid2 valueValid2 failure2 =>
      exact False.elim (schemaKey_not_rejected (WantedKeyMeaning receiptStatusNames) key2 (value2 ++ tail2) choice2 _ valid2
        (by rw [same] at failure; simpa only [List.append_assoc] using failure))
  | @value n seen choice old key tail offset reason valid failure =>
    cases second with
    | key failure2 =>
      exact False.elim (schemaKey_not_rejected (WantedKeyMeaning receiptStatusNames) key tail choice _ valid
        (by rw [← same] at failure2; exact failure2))
    | @value n2 seen2 choice2 old2 key2 tail2 offset2 reason2 valid2 failure2 =>
      obtain ⟨rfl, rfl, rfl⟩ := receiptStatusKey_prefix_unique key key2 tail tail2 choice choice2 valid valid2 same
      obtain ⟨rfl, rfl⟩ := receiptStatusItemRejected_unique _ _ _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
    | @next n2 seen2 choice2 old2 middle2 final2 key2 value2 tail2 consumed2 offset2 reason2 valid2 valueValid2 failure2 =>
      obtain ⟨_, choiceEq, tailEq⟩ := receiptStatusKey_prefix_unique key key2 tail (value2 ++ tail2) choice choice2 valid valid2
        (by simpa only [List.append_assoc] using same)
      subst choice2; subst tail
      exact False.elim (receiptStatusItem_not_rejected _ _ _ _ _ _ _ _ _ valueValid2 failure)
  | @next n seen choice old middle final key value tail consumed offset reason valid valueValid failure ih =>
    cases second with
    | key failure2 =>
      exact False.elim (schemaKey_not_rejected (WantedKeyMeaning receiptStatusNames) key (value ++ tail) choice _ valid
        (by rw [← same] at failure2; simpa only [List.append_assoc] using failure2))
    | @value n2 seen2 choice2 old2 key2 tail2 offset2 reason2 valid2 failure2 =>
      obtain ⟨_, choiceEq, tailEq⟩ := receiptStatusKey_prefix_unique key key2 (value ++ tail) tail2 choice choice2 valid valid2
        (by simpa only [List.append_assoc] using same)
      subst choice2; subst tail2
      exact False.elim (receiptStatusItem_not_rejected _ _ _ _ _ _ _ _ _ valueValid failure2)
    | @next n2 seen2 choice2 old2 middle2 final2 key2 value2 tail2 consumed2 offset2 reason2 valid2 valueValid2 failure2 =>
      obtain ⟨keyEq, choiceEq, valuesEq⟩ := receiptStatusKey_prefix_unique key key2 (value ++ tail) (value2 ++ tail2)
        choice choice2 valid valid2 (by simpa only [List.append_assoc] using same)
      subst choice2
      obtain ⟨valueEq, middleEq, tailEq⟩ := receiptStatusItem_prefix_unique _ _ _ _ _ _ _ _ _ _ valueValid valueValid2 valuesEq
      subst key2; subst value2; subst middle2; subst tail2
      obtain ⟨rfl, rfl, rfl, rfl⟩ := ih _ _ _ _ _ rfl failure2
      exact ⟨rfl, rfl, rfl, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusKey_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusPairs_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusPairs_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusPairsRejected_unique
