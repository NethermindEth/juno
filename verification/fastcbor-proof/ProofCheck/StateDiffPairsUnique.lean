import ProofCheck.StateDiffValueUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

theorem stateDiffKey_prefix_unique (left right restA restB : List w8) (a b : Option StateDiffField)
    (first : StateDiffKeyValid left a) (second : StateDiffKeyValid right b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB :=
  schemaKey_prefix_unique StateDiffKeyMeaning stateDiffKeyMeaning_unique left right restA restB a b first second same

/-- Every successful pair sequence uniquely determines the partial/final StateDiff
    and exact consumed prefix/suffix, including contextual nested-array guards. -/
theorem stateDiffPairs_prefix_unique (d n : Nat) (seen : List StateDiffField) (old a b : StateDiffValue)
    (left right restA restB : List w8)
    (first : StateDiffPairsValid d n seen old left restA a)
    (second : StateDiffPairsValid d n seen old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  induction first generalizing right b restB with
  | nil => cases second; simpa using same
  | @step n seen choice old middle final key value tail suffix keyValid fresh valueValid rest ih =>
    cases second with
    | @step n2 seen2 choice2 old2 middle2 final2 key2 value2 tail2 suffix2 keyValid2 fresh2 valueValid2 rest2 =>
      obtain ⟨keyEq, choiceEq, valuesEq⟩ := stateDiffKey_prefix_unique key key2
        (value ++ tail ++ suffix) (value2 ++ tail2 ++ restB) choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using same)
      subst choice2
      obtain ⟨valueEq, middleEq, tailsEq⟩ := stateDiffValue_prefix_unique d choice old middle middle2
        value value2 (tail ++ suffix) (tail2 ++ restB) valueValid valueValid2
        (by simpa only [List.append_assoc] using valuesEq)
      subst middle2
      obtain ⟨tailEq, finalEq, suffixEq⟩ := ih _ _ _ rest2 tailsEq
      exact ⟨by rw [keyEq, valueEq, tailEq], finalEq, suffixEq⟩
theorem stateDiffPairs_not_rejected (d n consumed offset extraDepth : Nat) (fields : List StateDiffField)
    (initial final retained : StateDiffValue) (item rest : List w8) (reason : StateDiffFailureReason)
    (valid : StateDiffPairsValid d n fields initial item rest final)
    (failure : StateDiffPairsRejected d n fields initial (item ++ rest) retained consumed offset extraDepth reason) (bound : (item ++ rest).length < 2^64) : False := by
  revert bound
  generalize eqWire : item ++ rest = input at failure ⊢
  induction valid generalizing input retained consumed offset extraDepth reason with
  | nil =>
    intro bound
    cases failure
  | @step n fields choice initial middle final key value tail rest keyValid fresh valueValid validTail ih =>
    intro bound
    cases failure with
    | key failure =>
      apply schemaKey_not_rejected StateDiffKeyMeaning key (value ++ tail ++ rest) choice _ keyValid
      rw [← eqWire] at failure
      simpa only [List.append_assoc] using failure
    | @duplicate n2 fields2 initial2 key2 tail2 wei keyValid2 duplicate =>
      obtain ⟨keyEq, choiceEq, tailEq⟩ := schemaKey_prefix_unique StateDiffKeyMeaning stateDiffKeyMeaning_unique
        key key2 (value ++ tail ++ rest) tail2 choice (some wei) keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst choice
      exact fresh duplicate
    | @value n2 fields2 choice2 initial2 retained2 key2 tail2 consumed2 offset2 extraDepth2 reason2 keyValid2 fresh2 failure =>
      obtain ⟨keyEq, choiceEq, tailEq⟩ := schemaKey_prefix_unique StateDiffKeyMeaning stateDiffKeyMeaning_unique
        key key2 (value ++ tail ++ rest) tail2 choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst choice2; subst tail2
      exact stateDiffValue_not_rejected d consumed2 offset2 _ choice initial middle _ value (tail ++ rest) reason2 valueValid
        (by simpa only [List.append_assoc] using failure) (by
          have lengths := congrArg List.length eqWire
          simp only [List.length_append] at bound lengths ⊢
          omega)
    | @next n2 fields2 choice2 initial2 middle2 final2 key2 value2 tail2 consumed2 offset2 extraDepth2 reason2 keyValid2 fresh2 valueValid2 failure =>
      obtain ⟨keyEq, choiceEq, valuesEq⟩ := schemaKey_prefix_unique StateDiffKeyMeaning stateDiffKeyMeaning_unique
        key key2 (value ++ tail ++ rest) (value2 ++ tail2) choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst choice2
      obtain ⟨valueEq, middleEq, tailsEq⟩ := stateDiffValue_prefix_unique d choice initial middle middle2
        value value2 (tail ++ rest) tail2 valueValid valueValid2
        (by simpa only [List.append_assoc] using valuesEq)
      subst middle2
      exact ih _ _ _ _ _ _ tailsEq failure (by
        have lengths := congrArg List.length eqWire
        simp only [List.length_append] at bound lengths ⊢
        omega)

/-- Full pair failures uniquely retain the partial StateDiff, consumed bytes,
    diagnostic offset and reason, with key/duplicate/value precedence. -/
theorem stateDiffPairsRejected_unique (d n consumedA offsetA depthA consumedB offsetB depthB : Nat) (fields : List StateDiffField)
    (initial finalA finalB : StateDiffValue) (wire : List w8) (a b : StateDiffFailureReason)
    (first : StateDiffPairsRejected d n fields initial wire finalA consumedA offsetA depthA a)
    (second : StateDiffPairsRejected d n fields initial wire finalB consumedB offsetB depthB b) (bound : wire.length < 2^64) :
    finalA = finalB ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b := by
  revert bound
  generalize eqWire : wire = wire2 at second
  induction first generalizing wire2 finalB consumedB offsetB depthB b with
  | @key n fields initial wire reason failure =>
    intro bound
    cases second with
    | @key n2 fields2 initial2 wire2 reason2 failure2 =>
      rw [eqWire] at failure
      exact ⟨rfl, rfl, rfl, rfl, congrArg StateDiffFailureReason.key (keyRejected_reason_unique _ _ _ failure failure2)⟩
    | @duplicate n2 fields2 initial2 key2 tail2 wei2 keyValid2 duplicate2 =>
      exact False.elim (schemaKey_not_rejected StateDiffKeyMeaning key2 tail2 (some wei2) _ keyValid2
        (by rw [eqWire] at failure; simpa only [List.append_assoc] using failure))
    | @value n2 fields2 choice2 initial2 retained2 key2 tail2 consumed2 offset2 extraDepth2 reason2 keyValid2 fresh2 failure2 =>
      exact False.elim (schemaKey_not_rejected StateDiffKeyMeaning key2 tail2 choice2 _ keyValid2
        (by rw [eqWire] at failure; simpa only [List.append_assoc] using failure))
    | @next n2 fields2 choice2 initial2 middle2 final2 key2 value2 tail2 consumed2 offset2 extraDepth2 reason2 keyValid2 fresh2 valueValid2 failure2 =>
      exact False.elim (schemaKey_not_rejected StateDiffKeyMeaning key2 (value2 ++ tail2) choice2 _ keyValid2
        (by rw [eqWire] at failure; simpa only [List.append_assoc] using failure))
  | @duplicate n fields initial key tail wei keyValid duplicate =>
    intro bound
    cases second with
    | @key n2 fields2 initial2 wire2 reason2 failure2 =>
      exact False.elim (schemaKey_not_rejected StateDiffKeyMeaning key tail (some wei) _ keyValid
        (by rw [eqWire.symm] at failure2; simpa only [List.append_assoc] using failure2))
    | @duplicate n2 fields2 initial2 key2 tail2 wei2 keyValid2 duplicate2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique StateDiffKeyMeaning stateDiffKeyMeaning_unique
        key key2 tail tail2 (some wei) (some wei2) keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      have weiEq := Option.some.inj choiceEq
      subst wei2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩
    | @value n2 fields2 choice2 initial2 retained2 key2 tail2 consumed2 offset2 extraDepth2 reason2 keyValid2 fresh2 failure2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique StateDiffKeyMeaning stateDiffKeyMeaning_unique
        key key2 tail tail2 (some wei) choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice2
      exact False.elim (fresh2 duplicate)
    | @next n2 fields2 choice2 initial2 middle2 final2 key2 value2 tail2 consumed2 offset2 extraDepth2 reason2 keyValid2 fresh2 valueValid2 failure2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique StateDiffKeyMeaning stateDiffKeyMeaning_unique
        key key2 tail (value2 ++ tail2) (some wei) choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice2
      exact False.elim (fresh2 duplicate)
  | @value n fields choice initial retained key tail consumed offset extraDepth reason keyValid fresh failure =>
    intro bound
    cases second with
    | @key n2 fields2 initial2 wire2 reason2 failure2 =>
      exact False.elim (schemaKey_not_rejected StateDiffKeyMeaning key tail choice _ keyValid
        (by rw [eqWire.symm] at failure2; simpa only [List.append_assoc] using failure2))
    | @duplicate n2 fields2 initial2 key2 tail2 wei2 keyValid2 duplicate2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique StateDiffKeyMeaning stateDiffKeyMeaning_unique
        key key2 tail tail2 choice (some wei2) keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice
      exact False.elim (fresh duplicate2)
    | @value n2 fields2 choice2 initial2 retained2 key2 tail2 consumed2 offset2 extraDepth2 reason2 keyValid2 fresh2 failure2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique StateDiffKeyMeaning stateDiffKeyMeaning_unique
        key key2 tail tail2 choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice2; subst tail2
      have diagnostic := stateDiffValueRejected_unique d consumed offset extraDepth consumed2 offset2 _ choice initial retained _ tail reason reason2 failure failure2 (by simp only [List.length_append] at bound ⊢; omega)
      exact ⟨diagnostic.1, by have := diagnostic.2.1; omega, by have := diagnostic.2.2.1; omega, diagnostic.2.2.2.1, congrArg StateDiffFailureReason.value diagnostic.2.2.2.2⟩
    | @next n2 fields2 choice2 initial2 middle2 final2 key2 value2 tail2 consumed2 offset2 extraDepth2 reason2 keyValid2 fresh2 valueValid2 failure2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique StateDiffKeyMeaning stateDiffKeyMeaning_unique
        key key2 tail (value2 ++ tail2) choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice2; subst tail
      exact False.elim (stateDiffValue_not_rejected d consumed offset extraDepth choice initial middle2 retained value2 tail2 reason valueValid2 failure (by simp only [List.length_append] at bound ⊢; omega))
  | @next n fields choice initial middle final key value tail consumed offset extraDepth reason keyValid fresh valueValid failure ih =>
    intro bound
    cases second with
    | @key n2 fields2 initial2 wire2 reason2 failure2 =>
      exact False.elim (schemaKey_not_rejected StateDiffKeyMeaning key (value ++ tail) choice _ keyValid
        (by rw [eqWire.symm] at failure2; simpa only [List.append_assoc] using failure2))
    | @duplicate n2 fields2 initial2 key2 tail2 wei2 keyValid2 duplicate2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique StateDiffKeyMeaning stateDiffKeyMeaning_unique
        key key2 (value ++ tail) tail2 choice (some wei2) keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice
      exact False.elim (fresh duplicate2)
    | @value n2 fields2 choice2 initial2 retained2 key2 tail2 consumed2 offset2 extraDepth2 reason2 keyValid2 fresh2 failure2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique StateDiffKeyMeaning stateDiffKeyMeaning_unique
        key key2 (value ++ tail) tail2 choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice2; subst tail2
      exact False.elim (stateDiffValue_not_rejected d consumed2 offset2 _ choice initial middle _ value tail reason2 valueValid failure2 (by simp only [List.length_append] at bound ⊢; omega))
    | @next n2 fields2 choice2 initial2 middle2 final2 key2 value2 tail2 consumed2 offset2 extraDepth2 reason2 keyValid2 fresh2 valueValid2 failure2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique StateDiffKeyMeaning stateDiffKeyMeaning_unique
        key key2 (value ++ tail) (value2 ++ tail2) choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice2
      obtain ⟨valueEq, middleEq, restEq⟩ := stateDiffValue_prefix_unique d choice initial middle middle2
        value value2 tail tail2 valueValid valueValid2 tailsEq
      subst value2; subst middle2; subst tail2
      have diagnostic := ih _ _ _ _ _ _ rfl failure2 (by simp only [List.length_append] at bound ⊢; omega)
      exact ⟨diagnostic.1, by have := diagnostic.2.1; omega,
        by have := diagnostic.2.2.1; omega, diagnostic.2.2.2⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffKey_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffPairs_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffPairs_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffPairsRejected_unique
