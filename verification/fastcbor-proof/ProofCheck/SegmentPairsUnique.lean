import ProofCheck.SegmentFieldUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

theorem segmentKey_prefix_unique (left right restA restB : List w8) (a b : Option SegmentField)
    (first : SegmentKeyValid left a) (second : SegmentKeyValid right b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB :=
  schemaKey_prefix_unique SegmentKeyMeaning segmentKeyMeaning_unique left right restA restB a b first second same

/-- Every successful pair sequence uniquely determines the partial/final Segment
    and exact consumed prefix/suffix, including contextual nested-array guards. -/
theorem segmentPairs_prefix_unique_of_array (d : Nat) (arrays : SegmentArrayDeterministic d) (n : Nat) (seen : List SegmentField) (old a b : SegmentValue)
    (left right restA restB : List w8)
    (first : SegmentPairsValid d n seen old left restA a)
    (second : SegmentPairsValid d n seen old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | nil => cases second; simpa using same
  | @step _ n seen choice old middle final key value tail suffix keyValid fresh valueValid rest =>
    cases second with
    | @step _ n2 seen2 choice2 old2 middle2 final2 key2 value2 tail2 suffix2 keyValid2 fresh2 valueValid2 rest2 =>
      obtain ⟨keyEq, choiceEq, valuesEq⟩ := segmentKey_prefix_unique key key2
        (value ++ tail ++ restA) (value2 ++ tail2 ++ restB) choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using same)
      subst choice2
      obtain ⟨valueEq, middleEq, tailsEq⟩ := segmentValue_prefix_unique_of_array d arrays choice old middle middle2
        value value2 (tail ++ restA) (tail2 ++ restB) valueValid valueValid2
        (by simpa only [List.append_assoc] using valuesEq)
      subst middle2
      obtain ⟨tailEq, finalEq, suffixEq⟩ := segmentPairs_prefix_unique_of_array d arrays n _ _ _ _ _ _ _ _ rest rest2 tailsEq
      exact ⟨by rw [keyEq, valueEq, tailEq], finalEq, suffixEq⟩
termination_by n

theorem segmentPairs_not_rejected_of_array (d : Nat) (arrays : SegmentArrayDeterministic d) (n consumed offset extraDepth : Nat) (fields : List SegmentField)
    (initial final retained : SegmentValue) (item rest : List w8) (reason : SegmentFailureReason)
    (valid : SegmentPairsValid d n fields initial item rest final)
    (failure : SegmentPairsRejected d n fields initial (item ++ rest) retained consumed offset extraDepth reason) : False := by
  generalize eqWire : item ++ rest = input at failure
  cases valid with
  | nil => cases failure
  | @step _ n fields choice initial middle final key value tail rest keyValid fresh valueValid validTail =>
    cases failure with
    | key failure =>
      apply schemaKey_not_rejected SegmentKeyMeaning key (value ++ tail ++ rest) choice _ keyValid
      rw [← eqWire] at failure
      simpa only [List.append_assoc] using failure
    | @duplicate _ n2 fields2 initial2 key2 tail2 wei keyValid2 duplicate =>
      obtain ⟨keyEq, choiceEq, tailEq⟩ := schemaKey_prefix_unique SegmentKeyMeaning segmentKeyMeaning_unique
        key key2 (value ++ tail ++ rest) tail2 choice (some wei) keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst choice
      exact fresh duplicate
    | @value _ n2 fields2 choice2 initial2 retained2 key2 tail2 consumed2 offset2 extraDepth2 reason2 keyValid2 fresh2 failure =>
      obtain ⟨keyEq, choiceEq, tailEq⟩ := schemaKey_prefix_unique SegmentKeyMeaning segmentKeyMeaning_unique
        key key2 (value ++ tail ++ rest) tail2 choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst choice2; subst tail2
      exact segmentValue_not_rejected_of_array d arrays consumed2 offset2 _ choice initial middle _ value (tail ++ rest) reason2 valueValid
        (by simpa only [List.append_assoc] using failure)
    | @next _ n2 fields2 choice2 initial2 middle2 final2 key2 value2 tail2 consumed2 offset2 extraDepth2 reason2 keyValid2 fresh2 valueValid2 failure =>
      obtain ⟨keyEq, choiceEq, valuesEq⟩ := schemaKey_prefix_unique SegmentKeyMeaning segmentKeyMeaning_unique
        key key2 (value ++ tail ++ rest) (value2 ++ tail2) choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst choice2
      obtain ⟨valueEq, middleEq, tailsEq⟩ := segmentValue_prefix_unique_of_array d arrays choice initial middle middle2
        value value2 (tail ++ rest) tail2 valueValid valueValid2
        (by simpa only [List.append_assoc] using valuesEq)
      subst middle2
      exact segmentPairs_not_rejected_of_array d arrays n _ _ _ _ _ _ _ _ _ _ validTail (by rw [tailsEq]; exact failure)
termination_by n

/-- Full pair failures uniquely retain the partial Segment, consumed bytes,
    diagnostic offset and reason, with key/duplicate/value precedence. -/
theorem segmentPairsRejected_unique_of_array (d : Nat) (arrays : SegmentArrayDeterministic d) (n consumedA offsetA depthA consumedB offsetB depthB : Nat) (fields : List SegmentField)
    (initial finalA finalB : SegmentValue) (wire : List w8) (a b : SegmentFailureReason)
    (first : SegmentPairsRejected d n fields initial wire finalA consumedA offsetA depthA a)
    (second : SegmentPairsRejected d n fields initial wire finalB consumedB offsetB depthB b) :
    finalA = finalB ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b := by
  generalize eqWire : wire = wire2 at second
  cases first with
  | @key _ n fields initial wire reason failure =>
    cases second with
    | @key _ n2 fields2 initial2 wire2 reason2 failure2 =>
      rw [eqWire] at failure
      exact ⟨rfl, rfl, rfl, rfl, congrArg SegmentFailureReason.key (keyRejected_reason_unique _ _ _ failure failure2)⟩
    | @duplicate _ n2 fields2 initial2 key2 tail2 wei2 keyValid2 duplicate2 =>
      exact False.elim (schemaKey_not_rejected SegmentKeyMeaning key2 tail2 (some wei2) _ keyValid2
        (by rw [eqWire] at failure; simpa only [List.append_assoc] using failure))
    | @value _ n2 fields2 choice2 initial2 retained2 key2 tail2 consumed2 offset2 extraDepth2 reason2 keyValid2 fresh2 failure2 =>
      exact False.elim (schemaKey_not_rejected SegmentKeyMeaning key2 tail2 choice2 _ keyValid2
        (by rw [eqWire] at failure; simpa only [List.append_assoc] using failure))
    | @next _ n2 fields2 choice2 initial2 middle2 final2 key2 value2 tail2 consumed2 offset2 extraDepth2 reason2 keyValid2 fresh2 valueValid2 failure2 =>
      exact False.elim (schemaKey_not_rejected SegmentKeyMeaning key2 (value2 ++ tail2) choice2 _ keyValid2
        (by rw [eqWire] at failure; simpa only [List.append_assoc] using failure))
  | @duplicate _ n fields initial key tail wei keyValid duplicate =>
    cases second with
    | @key _ n2 fields2 initial2 wire2 reason2 failure2 =>
      exact False.elim (schemaKey_not_rejected SegmentKeyMeaning key tail (some wei) _ keyValid
        (by rw [eqWire.symm] at failure2; simpa only [List.append_assoc] using failure2))
    | @duplicate _ n2 fields2 initial2 key2 tail2 wei2 keyValid2 duplicate2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique SegmentKeyMeaning segmentKeyMeaning_unique
        key key2 tail tail2 (some wei) (some wei2) keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      have weiEq := Option.some.inj choiceEq
      subst wei2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩
    | @value _ n2 fields2 choice2 initial2 retained2 key2 tail2 consumed2 offset2 extraDepth2 reason2 keyValid2 fresh2 failure2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique SegmentKeyMeaning segmentKeyMeaning_unique
        key key2 tail tail2 (some wei) choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice2
      exact False.elim (fresh2 duplicate)
    | @next _ n2 fields2 choice2 initial2 middle2 final2 key2 value2 tail2 consumed2 offset2 extraDepth2 reason2 keyValid2 fresh2 valueValid2 failure2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique SegmentKeyMeaning segmentKeyMeaning_unique
        key key2 tail (value2 ++ tail2) (some wei) choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice2
      exact False.elim (fresh2 duplicate)
  | @value _ n fields choice initial retained key tail consumed offset extraDepth reason keyValid fresh failure =>
    cases second with
    | @key _ n2 fields2 initial2 wire2 reason2 failure2 =>
      exact False.elim (schemaKey_not_rejected SegmentKeyMeaning key tail choice _ keyValid
        (by rw [eqWire.symm] at failure2; simpa only [List.append_assoc] using failure2))
    | @duplicate _ n2 fields2 initial2 key2 tail2 wei2 keyValid2 duplicate2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique SegmentKeyMeaning segmentKeyMeaning_unique
        key key2 tail tail2 choice (some wei2) keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice
      exact False.elim (fresh duplicate2)
    | @value _ n2 fields2 choice2 initial2 retained2 key2 tail2 consumed2 offset2 extraDepth2 reason2 keyValid2 fresh2 failure2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique SegmentKeyMeaning segmentKeyMeaning_unique
        key key2 tail tail2 choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice2; subst tail2
      have diagnostic := segmentValueRejected_unique_of_array d arrays consumed offset depthA consumed2 offset2 _ choice initial finalA _ tail reason reason2 failure failure2
      exact ⟨diagnostic.1, by have := diagnostic.2.1; omega, by have := diagnostic.2.2.1; omega, diagnostic.2.2.2.1, congrArg SegmentFailureReason.value diagnostic.2.2.2.2⟩
    | @next _ n2 fields2 choice2 initial2 middle2 final2 key2 value2 tail2 consumed2 offset2 extraDepth2 reason2 keyValid2 fresh2 valueValid2 failure2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique SegmentKeyMeaning segmentKeyMeaning_unique
        key key2 tail (value2 ++ tail2) choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice2; subst tail
      exact False.elim (segmentValue_not_rejected_of_array d arrays consumed offset depthA choice initial middle2 finalA value2 tail2 reason valueValid2 failure)
  | @next _ n fields choice initial middle final key value tail consumed offset extraDepth reason keyValid fresh valueValid failure =>
    cases second with
    | @key _ n2 fields2 initial2 wire2 reason2 failure2 =>
      exact False.elim (schemaKey_not_rejected SegmentKeyMeaning key (value ++ tail) choice _ keyValid
        (by rw [eqWire.symm] at failure2; simpa only [List.append_assoc] using failure2))
    | @duplicate _ n2 fields2 initial2 key2 tail2 wei2 keyValid2 duplicate2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique SegmentKeyMeaning segmentKeyMeaning_unique
        key key2 (value ++ tail) tail2 choice (some wei2) keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice
      exact False.elim (fresh duplicate2)
    | @value _ n2 fields2 choice2 initial2 retained2 key2 tail2 consumed2 offset2 extraDepth2 reason2 keyValid2 fresh2 failure2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique SegmentKeyMeaning segmentKeyMeaning_unique
        key key2 (value ++ tail) tail2 choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice2; subst tail2
      exact False.elim (segmentValue_not_rejected_of_array d arrays consumed2 offset2 _ choice initial middle _ value tail reason2 valueValid failure2)
    | @next _ n2 fields2 choice2 initial2 middle2 final2 key2 value2 tail2 consumed2 offset2 extraDepth2 reason2 keyValid2 fresh2 valueValid2 failure2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique SegmentKeyMeaning segmentKeyMeaning_unique
        key key2 (value ++ tail) (value2 ++ tail2) choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice2
      obtain ⟨valueEq, middleEq, restEq⟩ := segmentValue_prefix_unique_of_array d arrays choice initial middle middle2
        value value2 tail tail2 valueValid valueValid2 tailsEq
      subst value2; subst middle2; subst tail2
      have diagnostic := segmentPairsRejected_unique_of_array d arrays n _ _ _ _ _ _ _ _ _ _ _ _ _ failure failure2
      exact ⟨diagnostic.1, by have := diagnostic.2.1; omega,
        by have := diagnostic.2.2.1; omega, diagnostic.2.2.2⟩
termination_by n


end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentKey_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentPairs_prefix_unique_of_array
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentPairs_not_rejected_of_array
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentPairsRejected_unique_of_array
