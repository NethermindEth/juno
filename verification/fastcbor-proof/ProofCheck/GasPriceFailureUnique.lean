import ProofCheck.SliceFailureUnique
import ProofCheck.SupportedFailureSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

theorem gasPriceValueFailure_unique (d offsetA offsetB : Nat) (choice : Option Bool) (wire : List w8)
    (a b : FeltFailureReason) (first : GasPriceValueFailure d choice wire offsetA a)
    (second : GasPriceValueFailure d choice wire offsetB b) : offsetA = offsetB ∧ a = b := by
  cases first with
  | amount failure =>
    cases second with
    | amount failure2 => exact feltFailure_unique _ _ _ _ _ _ failure failure2
  | ignored failure =>
    cases second with
    | ignored failure2 =>
      have result := skipFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨result.1, congrArg FeltFailureReason.skipped result.2⟩

/-- Full pair failures uniquely retain the partial GasPrice, consumed bytes,
    diagnostic offset and reason, with key/duplicate/value precedence. -/
theorem gasPairsRejected_unique (d n consumedA offsetA consumedB offsetB : Nat) (fields : List Bool)
    (initial finalA finalB : GasPriceValue) (wire : List w8) (a b : GasPriceFailureReason)
    (first : GasPairsRejected d n fields initial wire finalA consumedA offsetA a)
    (second : GasPairsRejected d n fields initial wire finalB consumedB offsetB b) :
    finalA = finalB ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ a = b := by
  generalize eqWire : wire = wire2 at second
  induction first generalizing wire2 finalB consumedB offsetB b with
  | @key n fields initial wire reason failure =>
    cases second with
    | @key n2 fields2 initial2 wire2 reason2 failure2 =>
      rw [eqWire] at failure
      exact ⟨rfl, rfl, rfl, congrArg GasPriceFailureReason.key (keyRejected_reason_unique _ _ _ failure failure2)⟩
    | @duplicate n2 fields2 initial2 key2 tail2 wei2 keyValid2 duplicate2 =>
      exact False.elim (schemaKey_not_rejected GasKeyTextMeaning key2 tail2 (some wei2) _ keyValid2
        (by rw [eqWire] at failure; simpa only [List.append_assoc] using failure))
    | @value n2 offset2 fields2 choice2 initial2 key2 tail2 reason2 keyValid2 fresh2 failure2 =>
      exact False.elim (schemaKey_not_rejected GasKeyTextMeaning key2 tail2 choice2 _ keyValid2
        (by rw [eqWire] at failure; simpa only [List.append_assoc] using failure))
    | @next n2 consumed2 offset2 fields2 choice2 initial2 middle2 final2 key2 value2 tail2 reason2 keyValid2 fresh2 valueValid2 failure2 =>
      exact False.elim (schemaKey_not_rejected GasKeyTextMeaning key2 (value2 ++ tail2) choice2 _ keyValid2
        (by rw [eqWire] at failure; simpa only [List.append_assoc] using failure))
  | @duplicate n fields initial key tail wei keyValid duplicate =>
    cases second with
    | @key n2 fields2 initial2 wire2 reason2 failure2 =>
      exact False.elim (schemaKey_not_rejected GasKeyTextMeaning key tail (some wei) _ keyValid
        (by rw [eqWire.symm] at failure2; simpa only [List.append_assoc] using failure2))
    | @duplicate n2 fields2 initial2 key2 tail2 wei2 keyValid2 duplicate2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique GasKeyTextMeaning gasKeyMeaning_unique
        key key2 tail tail2 (some wei) (some wei2) keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      have weiEq := Option.some.inj choiceEq
      subst wei2
      exact ⟨rfl, rfl, rfl, rfl⟩
    | @value n2 offset2 fields2 choice2 initial2 key2 tail2 reason2 keyValid2 fresh2 failure2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique GasKeyTextMeaning gasKeyMeaning_unique
        key key2 tail tail2 (some wei) choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice2
      exact False.elim (fresh2 duplicate)
    | @next n2 consumed2 offset2 fields2 choice2 initial2 middle2 final2 key2 value2 tail2 reason2 keyValid2 fresh2 valueValid2 failure2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique GasKeyTextMeaning gasKeyMeaning_unique
        key key2 tail (value2 ++ tail2) (some wei) choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice2
      exact False.elim (fresh2 duplicate)
  | @value n offset fields choice initial key tail reason keyValid fresh failure =>
    cases second with
    | @key n2 fields2 initial2 wire2 reason2 failure2 =>
      exact False.elim (schemaKey_not_rejected GasKeyTextMeaning key tail choice _ keyValid
        (by rw [eqWire.symm] at failure2; simpa only [List.append_assoc] using failure2))
    | @duplicate n2 fields2 initial2 key2 tail2 wei2 keyValid2 duplicate2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique GasKeyTextMeaning gasKeyMeaning_unique
        key key2 tail tail2 choice (some wei2) keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice
      exact False.elim (fresh duplicate2)
    | @value n2 offset2 fields2 choice2 initial2 key2 tail2 reason2 keyValid2 fresh2 failure2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique GasKeyTextMeaning gasKeyMeaning_unique
        key key2 tail tail2 choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice2; subst tail2
      have diagnostic := gasPriceValueFailure_unique d offset offset2 choice tail reason reason2 failure failure2
      exact ⟨rfl, rfl, by have := diagnostic.1; omega, congrArg GasPriceFailureReason.inner diagnostic.2⟩
    | @next n2 consumed2 offset2 fields2 choice2 initial2 middle2 final2 key2 value2 tail2 reason2 keyValid2 fresh2 valueValid2 failure2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique GasKeyTextMeaning gasKeyMeaning_unique
        key key2 tail (value2 ++ tail2) choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice2; subst tail
      exact False.elim (gasValueValid_not_failure d offset choice initial middle2 value2 tail2 reason valueValid2 failure)
  | @next n consumed offset fields choice initial middle final key value tail reason keyValid fresh valueValid failure ih =>
    cases second with
    | @key n2 fields2 initial2 wire2 reason2 failure2 =>
      exact False.elim (schemaKey_not_rejected GasKeyTextMeaning key (value ++ tail) choice _ keyValid
        (by rw [eqWire.symm] at failure2; simpa only [List.append_assoc] using failure2))
    | @duplicate n2 fields2 initial2 key2 tail2 wei2 keyValid2 duplicate2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique GasKeyTextMeaning gasKeyMeaning_unique
        key key2 (value ++ tail) tail2 choice (some wei2) keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice
      exact False.elim (fresh duplicate2)
    | @value n2 offset2 fields2 choice2 initial2 key2 tail2 reason2 keyValid2 fresh2 failure2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique GasKeyTextMeaning gasKeyMeaning_unique
        key key2 (value ++ tail) tail2 choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice2; subst tail2
      exact False.elim (gasValueValid_not_failure d offset2 choice initial middle value tail reason2 valueValid failure2)
    | @next n2 consumed2 offset2 fields2 choice2 initial2 middle2 final2 key2 value2 tail2 reason2 keyValid2 fresh2 valueValid2 failure2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique GasKeyTextMeaning gasKeyMeaning_unique
        key key2 (value ++ tail) (value2 ++ tail2) choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice2
      obtain ⟨valueEq, middleEq, restEq⟩ := gasValueValid_prefix_unique d choice initial middle middle2
        value value2 tail tail2 valueValid valueValid2 tailsEq
      subst value2; subst middle2; subst tail2
      have diagnostic := ih _ _ _ _ _ rfl failure2
      exact ⟨diagnostic.1, by have := diagnostic.2.1; omega,
        by have := diagnostic.2.2.1; omega, diagnostic.2.2.2⟩

/-- Complete GasPrice record failures also uniquely determine whether the map
    depth was entered and the exact partially populated record. -/
theorem gasRecordRejected_unique (d consumedA offsetA consumedB offsetB : Nat) (enteredA enteredB : Bool)
    (initial finalA finalB : GasPriceValue) (wire : List w8) (a b : GasPriceFailureReason)
    (first : GasRecordRejected d initial wire finalA consumedA offsetA enteredA a)
    (second : GasRecordRejected d initial wire finalB consumedB offsetB enteredB b) :
    finalA = finalB ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ enteredA = enteredB ∧ a = b := by
  generalize eqWire : wire = wire2 at second
  cases first with
  | opening failure =>
    cases second with
    | opening failure2 =>
      rw [eqWire] at failure
      exact ⟨rfl, rfl, rfl, rfl, congrArg GasPriceFailureReason.inner (structOpenFailure_reason_unique _ _ _ _ failure failure2)⟩
    | map enc2 major2 length2 depth2 fit2 failure2 =>
      rw [eqWire] at failure
      exact False.elim (structOpenFailure_excludes_map _ _ _ _ _ _ enc2 major2 depth2 (by omega) failure)
  | @map head payload body argument count consumed offset final reason enc major length depth fit failure =>
    cases second with
    | opening failure2 =>
      rw [← eqWire] at failure2
      exact False.elim (structOpenFailure_excludes_map _ _ _ _ _ _ enc major depth (by omega) failure2)
    | @map head2 payload2 body2 argument2 count2 consumed2 offset2 final2 reason2 enc2 major2 length2 depth2 fit2 failure2 =>
      obtain ⟨headEq, payloadEq, argumentEq, bodiesEq⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2 eqWire
      subst head2; subst payload2; subst argument2; subst body2
      have counts : count = count2 := by omega
      subst count2
      have diagnostic := gasPairsRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ _ failure failure2
      exact ⟨diagnostic.1, by have := diagnostic.2.1; omega,
        by have := diagnostic.2.2.1; omega, rfl, diagnostic.2.2.2⟩

/-- The existing production-proved GasPrice failures inherit all of those
    equalities through their representable-input embedding. -/
theorem gasPriceFailure_unique (d consumedA offsetA consumedB offsetB : Nat) (enteredA enteredB : Bool)
    (initial finalA finalB : GasPriceValue) (wire : List w8) (a b : GasPriceFailureReason)
    (bound : wire.length < 2^64)
    (first : GasPriceFailure d wire initial finalA consumedA offsetA enteredA a)
    (second : GasPriceFailure d wire initial finalB consumedB offsetB enteredB b) :
    finalA = finalB ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ enteredA = enteredB ∧ a = b := by
  exact gasRecordRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ _
    (gasPriceFailure_rejected _ _ _ _ _ _ _ _ bound first)
    (gasPriceFailure_rejected _ _ _ _ _ _ _ _ bound second)

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.gasPriceValueFailure_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.gasPairsRejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.gasRecordRejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.gasPriceFailure_unique
