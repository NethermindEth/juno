import ProofCheck.StructOpenDisjoint
import ProofCheck.SignaturesDisjoint

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

theorem gasValueValid_not_failure (d offset : Nat) (choice : Option Bool) (initial final : GasPriceValue)
    (item rest : List w8) (reason : FeltFailureReason)
    (valid : GasValueValid d choice initial item final)
    (failure : GasPriceValueFailure d choice (item ++ rest) offset reason) : False := by
  cases valid with
  | amount valid =>
    cases failure with
    | amount failure => exact feltValid_not_failure d offset item rest _ reason valid failure
  | ignored valid =>
    cases failure with
    | ignored failure => exact skipValid_not_failure d offset item rest _ valid failure

/-- A successful GasPrice pair sequence cannot reject a key, duplicate, child
    value or later pair on the same input. -/
theorem gasPairsValid_not_rejected (d n consumed offset : Nat) (fields : List Bool)
    (initial final retained : GasPriceValue) (item rest : List w8) (reason : GasPriceFailureReason)
    (valid : GasPairsValid d n fields initial item final)
    (failure : GasPairsRejected d n fields initial (item ++ rest) retained consumed offset reason) : False := by
  generalize eqWire : item ++ rest = input at failure
  induction valid generalizing input rest retained consumed offset reason with
  | nil fields initial => cases failure
  | @step n fields choice initial middle final key value tail keyValid fresh valueValid validTail ih =>
    cases failure with
    | key failure =>
      apply schemaKey_not_rejected GasKeyTextMeaning key (value ++ tail ++ rest) choice _ keyValid
      rw [← eqWire] at failure
      simpa only [List.append_assoc] using failure
    | @duplicate n2 fields2 initial2 key2 tail2 wei keyValid2 duplicate =>
      obtain ⟨keyEq, choiceEq, tailEq⟩ := schemaKey_prefix_unique GasKeyTextMeaning gasKeyMeaning_unique
        key key2 (value ++ tail ++ rest) tail2 choice (some wei) keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst choice
      exact fresh duplicate
    | @value n2 offset2 fields2 choice2 initial2 key2 tail2 reason2 keyValid2 fresh2 failure =>
      obtain ⟨keyEq, choiceEq, tailEq⟩ := schemaKey_prefix_unique GasKeyTextMeaning gasKeyMeaning_unique
        key key2 (value ++ tail ++ rest) tail2 choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst choice2; subst tail2
      exact gasValueValid_not_failure d offset2 choice initial middle value (tail ++ rest) reason2 valueValid
        (by simpa only [List.append_assoc] using failure)
    | @next n2 consumed2 offset2 fields2 choice2 initial2 middle2 final2 key2 value2 tail2 reason2 keyValid2 fresh2 valueValid2 failure =>
      obtain ⟨keyEq, choiceEq, valuesEq⟩ := schemaKey_prefix_unique GasKeyTextMeaning gasKeyMeaning_unique
        key key2 (value ++ tail ++ rest) (value2 ++ tail2) choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst choice2
      obtain ⟨valueEq, middleEq, tailsEq⟩ := gasValueValid_prefix_unique d choice initial middle middle2
        value value2 (tail ++ rest) tail2 valueValid valueValid2
        (by simpa only [List.append_assoc] using valuesEq)
      subst middle2
      exact ih _ _ _ _ _ _ tailsEq failure

/-- Complete GasPrice records have disjoint successful and failed outcomes. -/
theorem gasRecordValid_not_rejected (d consumed offset : Nat) (entered : Bool)
    (initial final retained : GasPriceValue) (item rest : List w8) (reason : GasPriceFailureReason)
    (valid : GasRecordValid d initial item final)
    (failure : GasRecordRejected d initial (item ++ rest) retained consumed offset entered reason) : False := by
  generalize eqWire : item ++ rest = input at failure
  cases failure with
  | opening failure =>
    rw [← eqWire] at failure
    exact gasRecordValid_not_open_failure d initial final item rest _ valid failure
  | @map head payload body argument count consumed offset final2 reason enc major length depth fit failure =>
    cases valid with
    | nullish hn =>
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      rcases hn with rfl | rfl <;> contradiction
    | @map head2 payload2 body2 argument2 count2 final3 enc2 major2 length2 depth2 valid =>
      obtain ⟨headEq, payloadEq, argumentEq, tailEq⟩ := encodedHead_prefix_unique head2 head payload2 payload
        (body2 ++ rest) body argument2 argument enc2 enc (by simpa only [List.append_assoc] using eqWire)
      have counts : count2 = count := by rw [argumentEq] at length2; omega
      subst count2; subst body
      exact gasPairsValid_not_rejected (d + 1) count consumed offset [] initial _ _ body2 rest reason valid failure

/-- Nullish pointer success and complete child-map success both exclude every
    direct GasPrice record failure from the zero initial value. -/
theorem gasPointerValid_not_rejected (d consumed offset : Nat) (entered : Bool)
    (value : Option GasPriceValue) (retained : GasPriceValue) (item rest : List w8) (reason : GasPriceFailureReason)
    (valid : GasPointerValid d item value)
    (failure : GasRecordRejected d ⟨none, none⟩ (item ++ rest) retained consumed offset entered reason) : False := by
  cases valid with
  | nullish hn =>
    exact gasRecordValid_not_rejected d consumed offset entered ⟨none, none⟩ ⟨none, none⟩ retained _ rest reason
      (.nullish hn) failure
  | map enc major length depth valid =>
    exact gasRecordValid_not_rejected d consumed offset entered ⟨none, none⟩ _ retained _ rest reason
      (.map enc major length depth valid) failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.gasValueValid_not_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.gasPairsValid_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.gasRecordValid_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.gasPointerValid_not_rejected
