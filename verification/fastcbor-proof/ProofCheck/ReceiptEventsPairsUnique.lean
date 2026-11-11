import ProofCheck.ReceiptEventsItemUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

theorem receiptEventsKey_prefix_unique
    (left right restA restB : List w8) (a b : Option Nat)
    (first : WantedKeyValid receiptEventsNames left a)
    (second : WantedKeyValid receiptEventsNames right b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  exact schemaKey_prefix_unique (WantedKeyMeaning receiptEventsNames)
    (fun key a b => wantedKeyMeaning_unique _ key a b (projectionNames_distinct .receiptEvents))
    left right restA restB a b first second same

/-- Successful wire-order pairs have exactly one decoded value and suffix. -/
theorem receiptEventsPairs_prefix_unique (d n : Nat)
    (seen : List Nat) (old a b : ReceiptEventsValue) (left right restA restB : List w8)
    (first : ReceiptEventsPairsValid d n seen old left restA a)
    (second : ReceiptEventsPairsValid d n seen old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  induction first generalizing right b restB with
  | nil => cases second; simpa using same
  | @step n seen choice old middle final key value tail suffix keyValid valueValid rest ih =>
    cases second with
    | @step n2 seen2 choice2 old2 middle2 final2 key2 value2 tail2 suffix2 keyValid2 valueValid2 rest2 =>
      obtain ⟨keyEq, choiceEq, valuesEq⟩ := receiptEventsKey_prefix_unique key key2
        (value ++ tail ++ suffix) (value2 ++ tail2 ++ restB) choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using same)
      subst choice2
      obtain ⟨valueEq, middleEq, tailsEq⟩ := receiptEventsItem_prefix_unique d seen choice old middle middle2
        value value2 (tail ++ suffix) (tail2 ++ restB) valueValid valueValid2
        (by simpa only [List.append_assoc] using valuesEq)
      subst middle2
      obtain ⟨tailEq, finalEq, suffixEq⟩ := ih _ _ _ rest2 tailsEq
      exact ⟨by rw [keyEq, valueEq, tailEq], finalEq, suffixEq⟩

theorem receiptEventsPairs_not_rejected (d n consumed offset extraDepth : Nat)
    (seen : List Nat) (old final retained : ReceiptEventsValue) (item rest : List w8) (reason : ReceiptEventsFailure)
    (valid : ReceiptEventsPairsValid d n seen old item rest final)
    (failure : ReceiptEventsPairsRejected d n seen old (item ++ rest) retained consumed offset extraDepth reason) : False := by
  generalize same : item ++ rest = wire at failure
  induction valid generalizing wire retained consumed offset extraDepth reason with
  | nil => cases failure
  | @step n seen choice old middle final key value tail rest keyValid valueValid restValid ih =>
    cases failure with
    | key rejected =>
      apply schemaKey_not_rejected (WantedKeyMeaning receiptEventsNames) key (value ++ tail ++ rest) choice _ keyValid
      rw [← same] at rejected
      simpa only [List.append_assoc] using rejected
    | @value n2 seen2 choice2 old2 key2 tail2 consumed2 offset2 depth2 reason2 valid2 rejected =>
      obtain ⟨_, choiceEq, valuesEq⟩ := receiptEventsKey_prefix_unique key key2
        (value ++ tail ++ rest) tail2 choice choice2 keyValid valid2
        (by simpa only [List.append_assoc] using same)
      subst choice2; subst tail2
      exact receiptEventsItem_not_rejected d consumed2 offset2 _ seen choice old middle value (tail ++ rest) _ valueValid
        (by simpa only [List.append_assoc] using rejected)
    | @next n2 seen2 choice2 old2 middle2 final2 key2 value2 tail2 consumed2 offset2 depth2 reason2 valid2 valueValid2 rejected =>
      obtain ⟨_, choiceEq, valuesEq⟩ := receiptEventsKey_prefix_unique key key2
        (value ++ tail ++ rest) (value2 ++ tail2) choice choice2 keyValid valid2
        (by simpa only [List.append_assoc] using same)
      subst choice2
      obtain ⟨_, middleEq, tailsEq⟩ := receiptEventsItem_prefix_unique d seen choice old middle middle2
        value value2 (tail ++ rest) tail2 valueValid valueValid2
        (by simpa only [List.append_assoc] using valuesEq)
      subst middle2
      exact ih _ _ _ _ _ _ tailsEq rejected

/-- Rejection fixes the retained fields as well as both error positions and the
    failure reason, even after arbitrarily many successful preceding pairs. -/
theorem receiptEventsPairsRejected_unique (d n consumedA offsetA depthA consumedB offsetB depthB : Nat)
    (seen : List Nat) (old finalA finalB : ReceiptEventsValue) (wire : List w8) (a b : ReceiptEventsFailure)
    (first : ReceiptEventsPairsRejected d n seen old wire finalA consumedA offsetA depthA a)
    (second : ReceiptEventsPairsRejected d n seen old wire finalB consumedB offsetB depthB b) :
    finalA = finalB ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b := by
  generalize same : wire = wire2 at second
  induction first generalizing wire2 finalB consumedB offsetB depthB b with
  | @key n seen old wire reason failure =>
    cases second with
    | key failure2 =>
      rw [← same] at failure2
      exact ⟨rfl, rfl, rfl, rfl, congrArg ReceiptEventsFailure.key (keyRejected_reason_unique _ _ _ failure failure2)⟩
    | @value n2 seen2 choice2 old2 key2 tail2 consumed2 offset2 depth2 reason2 valid2 failure2 =>
      exact False.elim (schemaKey_not_rejected (WantedKeyMeaning receiptEventsNames) key2 tail2 choice2 _ valid2
        (by rw [same] at failure; exact failure))
    | @next n2 seen2 choice2 old2 middle2 final2 key2 value2 tail2 consumed2 offset2 depth2 reason2 valid2 valueValid2 failure2 =>
      exact False.elim (schemaKey_not_rejected (WantedKeyMeaning receiptEventsNames) key2 (value2 ++ tail2) choice2 _ valid2
        (by rw [same] at failure; simpa only [List.append_assoc] using failure))
  | @value n seen choice old key tail consumed offset extraDepth reason valid failure =>
    cases second with
    | key failure2 =>
      exact False.elim (schemaKey_not_rejected (WantedKeyMeaning receiptEventsNames) key tail choice _ valid
        (by rw [← same] at failure2; exact failure2))
    | @value n2 seen2 choice2 old2 key2 tail2 consumed2 offset2 depth2 reason2 valid2 failure2 =>
      obtain ⟨rfl, rfl, rfl⟩ := receiptEventsKey_prefix_unique key key2 tail tail2 choice choice2 valid valid2 same
      obtain ⟨rfl, rfl, rfl, rfl⟩ := receiptEventsItemRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩
    | @next n2 seen2 choice2 old2 middle2 final2 key2 value2 tail2 consumed2 offset2 depth2 reason2 valid2 valueValid2 failure2 =>
      obtain ⟨_, choiceEq, tailEq⟩ := receiptEventsKey_prefix_unique key key2 tail (value2 ++ tail2) choice choice2 valid valid2
        (by simpa only [List.append_assoc] using same)
      subst choice2; subst tail
      exact False.elim (receiptEventsItem_not_rejected _ _ _ _ _ _ _ _ _ _ _ valueValid2 failure)
  | @next n seen choice old middle final key value tail consumed offset extraDepth reason valid valueValid failure ih =>
    cases second with
    | key failure2 =>
      exact False.elim (schemaKey_not_rejected (WantedKeyMeaning receiptEventsNames) key (value ++ tail) choice _ valid
        (by rw [← same] at failure2; simpa only [List.append_assoc] using failure2))
    | @value n2 seen2 choice2 old2 key2 tail2 consumed2 offset2 depth2 reason2 valid2 failure2 =>
      obtain ⟨_, choiceEq, tailEq⟩ := receiptEventsKey_prefix_unique key key2 (value ++ tail) tail2 choice choice2 valid valid2
        (by simpa only [List.append_assoc] using same)
      subst choice2; subst tail2
      exact False.elim (receiptEventsItem_not_rejected _ _ _ _ _ _ _ _ _ _ _ valueValid failure2)
    | @next n2 seen2 choice2 old2 middle2 final2 key2 value2 tail2 consumed2 offset2 depth2 reason2 valid2 valueValid2 failure2 =>
      obtain ⟨keyEq, choiceEq, valuesEq⟩ := receiptEventsKey_prefix_unique key key2 (value ++ tail) (value2 ++ tail2)
        choice choice2 valid valid2 (by simpa only [List.append_assoc] using same)
      subst choice2
      obtain ⟨valueEq, middleEq, tailEq⟩ := receiptEventsItem_prefix_unique _ _ _ _ _ _ _ _ _ _ valueValid valueValid2 valuesEq
      subst key2; subst value2; subst middle2; subst tail2
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := ih _ _ _ _ _ _ rfl failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsKey_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsPairs_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsPairs_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsPairsRejected_unique
