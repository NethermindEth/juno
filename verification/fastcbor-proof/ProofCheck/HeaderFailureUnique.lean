import ProofCheck.GasPriceFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Every selected field has unique failure consumption, diagnostic, retained
    depth and reason, including private failed GasPrice construction. -/
theorem headerItemRejected_unique (d consumedA offsetA depthA consumedB offsetB depthB : Nat)
    (choice choice2 : Option HeaderField) (wire : List w8) (a b : HeaderValueFailureReason)
    (bound : wire.length < 2^64)
    (first : HeaderItemRejected d choice wire consumedA offsetA depthA a)
    (second : HeaderItemRejected d choice2 wire consumedB offsetB depthB b) (choiceEq : choice = choice2) :
    consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b := by
  cases first with
  | felt field failure =>
    cases second with
    | felt field2 failure2 =>
      have fieldEq : field = field2 := by
        cases field <;> cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
      subst field2
      have result := feltFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, result.1, rfl, congrArg HeaderValueFailureReason.felt result.2⟩
    | scalar field2 failure2 =>
      cases field <;> cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | signatures failure2 =>
      cases field <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | string failure2 =>
      cases field <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | gas field2 retained2 failure2 =>
      cases field <;> cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | ignored failure2 =>
      cases field <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
  | scalar field failure =>
    cases second with
    | felt field2 failure2 =>
      cases field <;> cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | scalar field2 failure2 =>
      have fieldEq : field = field2 := by
        cases field <;> cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
      subst field2
      have reason := scalarFailure_reason_unique _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, congrArg (fun r => HeaderValueFailureReason.felt (.expected (W8 0) r)) reason⟩
    | signatures failure2 =>
      cases field <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | string failure2 =>
      cases field <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | gas field2 retained2 failure2 =>
      cases field <;> cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | ignored failure2 =>
      cases field <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
  | signatures failure =>
    cases second with
    | felt field2 failure2 =>
      cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | scalar field2 failure2 =>
      cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | signatures failure2 =>
      have result := signaturesFailure_unique _ _ _ _ _ _ _ wire _ _ bound failure failure2
      exact ⟨result.1, result.2.1, result.2.2.1, congrArg HeaderValueFailureReason.felt result.2.2.2⟩
    | string failure2 =>
      simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | gas field2 retained2 failure2 =>
      cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | ignored failure2 =>
      simp_all [headerFeltKey, headerScalarKey, headerGasKey]
  | string failure =>
    cases second with
    | felt field2 failure2 =>
      cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | scalar field2 failure2 =>
      cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | signatures failure2 =>
      simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | string failure2 =>
      exact ⟨rfl, rfl, rfl, congrArg HeaderValueFailureReason.string (stringRejected_reason_unique _ _ _ failure failure2)⟩
    | gas field2 retained2 failure2 =>
      cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | ignored failure2 =>
      simp_all [headerFeltKey, headerScalarKey, headerGasKey]
  | gas field retained failure =>
    cases second with
    | felt field2 failure2 =>
      cases field <;> cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | scalar field2 failure2 =>
      cases field <;> cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | signatures failure2 =>
      cases field <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | string failure2 =>
      cases field <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | gas field2 retained2 failure2 =>
      have fieldEq : field = field2 := by
        cases field <;> cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
      subst field2
      have result := gasRecordRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ _ failure failure2
      exact ⟨result.2.1, result.2.2.1, by rw [result.2.2.2.1], congrArg HeaderValueFailureReason.gas result.2.2.2.2⟩
    | ignored failure2 =>
      cases field <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
  | ignored failure =>
    cases second with
    | felt field2 failure2 =>
      cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | scalar field2 failure2 =>
      cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | signatures failure2 =>
      simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | string failure2 =>
      simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | gas field2 retained2 failure2 =>
      cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | ignored failure2 =>
      have result := skipFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, result.1, rfl, congrArg (fun r => HeaderValueFailureReason.felt (.skipped r)) result.2⟩

/-- Full pair failures uniquely retain the partial Header, consumed bytes,
    diagnostic offset and reason, with key/duplicate/value precedence. -/
theorem headerPairsRejected_unique (d n consumedA offsetA depthA consumedB offsetB depthB : Nat) (fields : List HeaderField)
    (initial finalA finalB : HeaderValue) (wire : List w8) (a b : HeaderRecordFailureReason)
    (bound : wire.length < 2^64)
    (first : HeaderPairsRejected d n fields initial wire finalA consumedA offsetA depthA a)
    (second : HeaderPairsRejected d n fields initial wire finalB consumedB offsetB depthB b) :
    finalA = finalB ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b := by
  generalize eqWire : wire = wire2 at second
  induction first generalizing wire2 finalB consumedB offsetB depthB b with
  | @key n fields initial wire reason failure =>
    cases second with
    | @key n2 fields2 initial2 wire2 reason2 failure2 =>
      rw [eqWire] at failure
      exact ⟨rfl, rfl, rfl, rfl, congrArg HeaderRecordFailureReason.key (keyRejected_reason_unique _ _ _ failure failure2)⟩
    | @duplicate n2 fields2 initial2 key2 tail2 field2 keyValid2 duplicate2 =>
      exact False.elim (schemaKey_not_rejected HeaderKeyTextMeaning key2 tail2 (some field2) _ keyValid2
        (by rw [eqWire] at failure; simpa only [List.append_assoc] using failure))
    | @value n2 consumed2 offset2 extraDepth2 fields2 choice2 initial2 key2 tail2 reason2 keyValid2 fresh2 failure2 =>
      exact False.elim (schemaKey_not_rejected HeaderKeyTextMeaning key2 tail2 choice2 _ keyValid2
        (by rw [eqWire] at failure; simpa only [List.append_assoc] using failure))
    | @next n2 consumed2 offset2 extraDepth2 fields2 choice2 initial2 middle2 final2 key2 value2 tail2 reason2 keyValid2 fresh2 valueValid2 failure2 =>
      exact False.elim (schemaKey_not_rejected HeaderKeyTextMeaning key2 (value2 ++ tail2) choice2 _ keyValid2
        (by rw [eqWire] at failure; simpa only [List.append_assoc] using failure))
  | @duplicate n fields initial key tail field keyValid duplicate =>
    cases second with
    | @key n2 fields2 initial2 wire2 reason2 failure2 =>
      exact False.elim (schemaKey_not_rejected HeaderKeyTextMeaning key tail (some field) _ keyValid
        (by rw [eqWire.symm] at failure2; simpa only [List.append_assoc] using failure2))
    | @duplicate n2 fields2 initial2 key2 tail2 field2 keyValid2 duplicate2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique HeaderKeyTextMeaning headerKeyMeaning_unique
        key key2 tail tail2 (some field) (some field2) keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      have fieldEq := Option.some.inj choiceEq
      subst field2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩
    | @value n2 consumed2 offset2 extraDepth2 fields2 choice2 initial2 key2 tail2 reason2 keyValid2 fresh2 failure2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique HeaderKeyTextMeaning headerKeyMeaning_unique
        key key2 tail tail2 (some field) choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice2
      exact False.elim (fresh2 duplicate)
    | @next n2 consumed2 offset2 extraDepth2 fields2 choice2 initial2 middle2 final2 key2 value2 tail2 reason2 keyValid2 fresh2 valueValid2 failure2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique HeaderKeyTextMeaning headerKeyMeaning_unique
        key key2 tail (value2 ++ tail2) (some field) choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice2
      exact False.elim (fresh2 duplicate)
  | @value n consumed offset extraDepth fields choice initial key tail reason keyValid fresh failure =>
    cases second with
    | @key n2 fields2 initial2 wire2 reason2 failure2 =>
      exact False.elim (schemaKey_not_rejected HeaderKeyTextMeaning key tail choice _ keyValid
        (by rw [eqWire.symm] at failure2; simpa only [List.append_assoc] using failure2))
    | @duplicate n2 fields2 initial2 key2 tail2 field2 keyValid2 duplicate2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique HeaderKeyTextMeaning headerKeyMeaning_unique
        key key2 tail tail2 choice (some field2) keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice
      exact False.elim (fresh duplicate2)
    | @value n2 consumed2 offset2 extraDepth2 fields2 choice2 initial2 key2 tail2 reason2 keyValid2 fresh2 failure2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique HeaderKeyTextMeaning headerKeyMeaning_unique
        key key2 tail tail2 choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice2; subst tail2
      have tailBound : tail.length < 2^64 := by simp only [List.length_append] at bound; omega
      have diagnostic := headerItemRejected_unique d consumed offset extraDepth consumed2 offset2 depthB
        choice choice tail reason reason2 tailBound failure failure2 rfl
      exact ⟨rfl, by have := diagnostic.1; omega, by have := diagnostic.2.1; omega,
        diagnostic.2.2.1, congrArg HeaderRecordFailureReason.value diagnostic.2.2.2⟩
    | @next n2 consumed2 offset2 extraDepth2 fields2 choice2 initial2 middle2 final2 key2 value2 tail2 reason2 keyValid2 fresh2 valueValid2 failure2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique HeaderKeyTextMeaning headerKeyMeaning_unique
        key key2 tail (value2 ++ tail2) choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice2; subst tail
      have valueBound : (value2 ++ tail2).length < 2^64 := by simp only [List.length_append] at bound ⊢; omega
      exact False.elim (headerItemValid_not_rejected d consumed offset extraDepth choice choice initial middle2 value2 tail2
        reason valueBound valueValid2 failure rfl)
  | @next n consumed offset extraDepth fields choice initial middle final key value tail reason keyValid fresh valueValid failure ih =>
    cases second with
    | @key n2 fields2 initial2 wire2 reason2 failure2 =>
      exact False.elim (schemaKey_not_rejected HeaderKeyTextMeaning key (value ++ tail) choice _ keyValid
        (by rw [eqWire.symm] at failure2; simpa only [List.append_assoc] using failure2))
    | @duplicate n2 fields2 initial2 key2 tail2 field2 keyValid2 duplicate2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique HeaderKeyTextMeaning headerKeyMeaning_unique
        key key2 (value ++ tail) tail2 choice (some field2) keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice
      exact False.elim (fresh duplicate2)
    | @value n2 consumed2 offset2 extraDepth2 fields2 choice2 initial2 key2 tail2 reason2 keyValid2 fresh2 failure2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique HeaderKeyTextMeaning headerKeyMeaning_unique
        key key2 (value ++ tail) tail2 choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice2; subst tail2
      have valueBound : (value ++ tail).length < 2^64 := by simp only [List.length_append] at bound ⊢; omega
      exact False.elim (headerItemValid_not_rejected d consumed2 offset2 depthB choice choice initial middle value tail
        reason2 valueBound valueValid failure2 rfl)
    | @next n2 consumed2 offset2 extraDepth2 fields2 choice2 initial2 middle2 final2 key2 value2 tail2 reason2 keyValid2 fresh2 valueValid2 failure2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := schemaKey_prefix_unique HeaderKeyTextMeaning headerKeyMeaning_unique
        key key2 (value ++ tail) (value2 ++ tail2) choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice2
      obtain ⟨valueEq, middleEq, restEq⟩ := headerItemValid_prefix_unique d choice choice initial middle middle2
        value value2 tail tail2 valueValid valueValid2 rfl tailsEq
      subst value2; subst middle2; subst tail2
      have tailBound : tail.length < 2^64 := by simp only [List.length_append] at bound; omega
      have diagnostic := ih _ _ _ _ _ tailBound _ rfl failure2
      exact ⟨diagnostic.1, by have := diagnostic.2.1; omega,
        by have := diagnostic.2.2.1; omega, diagnostic.2.2.2.1, diagnostic.2.2.2.2⟩

/-- Complete Header record failures also uniquely determine whether the map
    depth was entered and the exact partially populated record. -/
theorem headerRecordRejected_unique (d consumedA offsetA consumedB offsetB : Nat) (depthA depthB : Nat)
    (initial finalA finalB : HeaderValue) (wire : List w8) (a b : HeaderRecordFailureReason)
    (bound : wire.length < 2^64)
    (first : HeaderRecordRejected d initial wire finalA consumedA offsetA depthA a)
    (second : HeaderRecordRejected d initial wire finalB consumedB offsetB depthB b) :
    finalA = finalB ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b := by
  generalize eqWire : wire = wire2 at second
  cases first with
  | opening failure =>
    cases second with
    | opening failure2 =>
      rw [eqWire] at failure
      exact ⟨rfl, rfl, rfl, rfl, congrArg HeaderRecordFailureReason.opening (structOpenFailure_reason_unique _ _ _ _ failure failure2)⟩
    | map enc2 major2 length2 depth2 fit2 failure2 =>
      rw [eqWire] at failure
      exact False.elim (structOpenFailure_excludes_map _ _ _ _ _ _ enc2 major2 depth2 (by omega) failure)
  | @map head payload body argument count consumed offset extraDepth final reason enc major length depth fit failure =>
    cases second with
    | opening failure2 =>
      rw [← eqWire] at failure2
      exact False.elim (structOpenFailure_excludes_map _ _ _ _ _ _ enc major depth (by omega) failure2)
    | @map head2 payload2 body2 argument2 count2 consumed2 offset2 extraDepth2 final2 reason2 enc2 major2 length2 depth2 fit2 failure2 =>
      obtain ⟨headEq, payloadEq, argumentEq, bodiesEq⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2 eqWire
      subst head2; subst payload2; subst argument2; subst body2
      have counts : count = count2 := by omega
      subst count2
      have bodyBound : body.length < 2^64 := by simp only [List.length_append, List.length_cons] at bound; omega
      have diagnostic := headerPairsRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ bodyBound failure failure2
      exact ⟨diagnostic.1, by have := diagnostic.2.1; omega,
        by have := diagnostic.2.2.1; omega, by have := diagnostic.2.2.2.1; omega, diagnostic.2.2.2.2⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerItemRejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.headerPairsRejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.headerRecordRejected_unique
