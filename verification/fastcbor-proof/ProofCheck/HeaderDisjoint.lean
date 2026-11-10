import ProofCheck.GasPriceDisjoint

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Every selected Header field has disjoint success and failure on physically
    representable input, including nested signature and GasPrice children. -/
theorem headerItemValid_not_rejected (d consumed offset extraDepth : Nat) (choice choice2 : Option HeaderField)
    (initial final : HeaderValue) (item rest : List w8) (reason : HeaderValueFailureReason)
    (bound : (item ++ rest).length < 2^64) (valid : HeaderItemValid d choice initial item final)
    (failure : HeaderItemRejected d choice2 (item ++ rest) consumed offset extraDepth reason)
    (choiceEq : choice = choice2) : False := by
  cases valid with
  | felt field valid =>
    cases failure with
    | felt field2 failure =>
      have fieldEq : field = field2 := by
        cases field <;> cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
      subst field2
      exact feltValid_not_failure d offset item rest _ _ valid failure
    | scalar field2 failure =>
      cases field <;> cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | signatures failure =>
      cases field <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | string failure =>
      cases field <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | gas field2 retained failure =>
      cases field <;> cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | ignored failure =>
      cases field <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
  | scalar field valid =>
    cases failure with
    | felt field2 failure =>
      cases field <;> cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | scalar field2 failure =>
      have fieldEq : field = field2 := by
        cases field <;> cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
      subst field2
      exact scalarValid_not_failure (headerScalarValue field initial) _ item rest _ valid failure
    | signatures failure =>
      cases field <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | string failure =>
      cases field <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | gas field2 retained failure =>
      cases field <;> cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | ignored failure =>
      cases field <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
  | signatures valid =>
    cases failure with
    | felt field2 failure =>
      cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | scalar field2 failure =>
      cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | signatures failure =>
      exact signaturesValid_not_failure d consumed offset extraDepth item rest _ _ bound valid failure
    | string failure =>
      simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | gas field2 retained failure =>
      cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | ignored failure =>
      simp_all [headerFeltKey, headerScalarKey, headerGasKey]
  | string valid =>
    cases failure with
    | felt field2 failure =>
      cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | scalar field2 failure =>
      cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | signatures failure =>
      simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | string failure =>
      exact stringValid_not_rejected initial.protocolVersion _ item rest _ valid failure
    | gas field2 retained failure =>
      cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | ignored failure =>
      simp_all [headerFeltKey, headerScalarKey, headerGasKey]
  | gas field valid =>
    cases failure with
    | felt field2 failure =>
      cases field <;> cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | scalar field2 failure =>
      cases field <;> cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | signatures failure =>
      cases field <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | string failure =>
      cases field <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | gas field2 retained failure =>
      have fieldEq : field = field2 := by
        cases field <;> cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
      subst field2
      exact gasPointerValid_not_rejected d consumed offset _ _ retained item rest _ valid failure
    | ignored failure =>
      cases field <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
  | ignored valid =>
    cases failure with
    | felt field2 failure =>
      cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | scalar field2 failure =>
      cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | signatures failure =>
      simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | string failure =>
      simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | gas field2 retained failure =>
      cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | ignored failure =>
      exact skipValid_not_failure d offset item rest _ valid failure

/-- A successful Header pair sequence cannot reject a key, duplicate, child
    value or later pair on the same input. -/
theorem headerPairsValid_not_rejected (d n consumed offset extraDepth : Nat) (fields : List HeaderField)
    (initial final retained : HeaderValue) (item rest : List w8) (reason : HeaderRecordFailureReason)
    (bound : (item ++ rest).length < 2^64) (valid : HeaderPairsValid d n fields initial item final)
    (failure : HeaderPairsRejected d n fields initial (item ++ rest) retained consumed offset extraDepth reason) : False := by
  generalize eqWire : item ++ rest = input at failure
  induction valid generalizing input rest retained consumed offset extraDepth reason with
  | nil fields initial => cases failure
  | @step n fields choice initial middle final key value tail keyValid fresh valueValid validTail ih =>
    cases failure with
    | key failure =>
      apply schemaKey_not_rejected HeaderKeyTextMeaning key (value ++ tail ++ rest) choice _ keyValid
      rw [← eqWire] at failure
      simpa only [List.append_assoc] using failure
    | @duplicate n2 fields2 initial2 key2 tail2 field keyValid2 duplicate =>
      obtain ⟨keyEq, choiceEq, tailEq⟩ := schemaKey_prefix_unique HeaderKeyTextMeaning headerKeyMeaning_unique
        key key2 (value ++ tail ++ rest) tail2 choice (some field) keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst choice
      exact fresh duplicate
    | @value n2 consumed2 offset2 extraDepth2 fields2 choice2 initial2 key2 tail2 reason2 keyValid2 fresh2 failure =>
      obtain ⟨keyEq, choiceEq, tailEq⟩ := schemaKey_prefix_unique HeaderKeyTextMeaning headerKeyMeaning_unique
        key key2 (value ++ tail ++ rest) tail2 choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst choice2; subst tail2
      have valueBound : (value ++ (tail ++ rest)).length < 2^64 := by
        simp only [List.length_append] at bound ⊢; omega
      exact headerItemValid_not_rejected d consumed2 offset2 extraDepth choice choice initial middle value
        (tail ++ rest) reason2 valueBound valueValid (by simpa only [List.append_assoc] using failure) rfl
    | @next n2 consumed2 offset2 extraDepth2 fields2 choice2 initial2 middle2 final2 key2 value2 tail2 reason2 keyValid2 fresh2 valueValid2 failure =>
      obtain ⟨keyEq, choiceEq, valuesEq⟩ := schemaKey_prefix_unique HeaderKeyTextMeaning headerKeyMeaning_unique
        key key2 (value ++ tail ++ rest) (value2 ++ tail2) choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst choice2
      obtain ⟨valueEq, middleEq, tailsEq⟩ := headerItemValid_prefix_unique d choice choice initial middle middle2
        value value2 (tail ++ rest) tail2 valueValid valueValid2 rfl
        (by simpa only [List.append_assoc] using valuesEq)
      subst middle2
      have tailBound : (tail ++ rest).length < 2^64 := by
        simp only [List.length_append] at bound ⊢; omega
      exact ih _ _ _ _ _ _ tailBound _ tailsEq failure

/-- Complete Header records have disjoint successful and failed outcomes. -/
theorem headerRecordValid_not_rejected (d consumed offset extraDepth : Nat)
    (initial final retained : HeaderValue) (item rest : List w8) (reason : HeaderRecordFailureReason)
    (bound : (item ++ rest).length < 2^64) (valid : HeaderRecordValid d initial item final)
    (failure : HeaderRecordRejected d initial (item ++ rest) retained consumed offset extraDepth reason) : False := by
  generalize eqWire : item ++ rest = input at failure
  cases failure with
  | opening failure =>
    rw [← eqWire] at failure
    exact headerRecordValid_not_open_failure d initial final item rest _ valid failure
  | @map head payload body argument count consumed offset extraDepth final2 reason enc major length depth fit failure =>
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
      have bodyBound : (body2 ++ rest).length < 2^64 := by
        simp only [List.length_append, List.length_cons] at bound ⊢; omega
      exact headerPairsValid_not_rejected (d + 1) count consumed offset extraDepth [] initial _ _ body2 rest reason
        bodyBound valid failure

/-- First decoding cannot specify both success and failure on the same
    representable input and initial Header, including destination-guard errors. -/
theorem headerFirstSpec_success_not_failure (old value failed : HeaderValue) (wire rest : List w8)
    (error : HeaderAPIError) (bound : wire.length < 2^64)
    (success : HeaderFirstSpec old wire (.success value rest))
    (failure : HeaderFirstSpec old wire (.failure failed error)) : False := by
  obtain ⟨zero, item, split, valid⟩ := headerFirstSpec_success old value wire rest success
  subst old
  cases failure with
  | nonzero old wire nonzero => exact nonzero rfl
  | rejected failure =>
    subst wire
    exact headerRecordValid_not_rejected _ _ _ _ _ _ _ _ _ _ bound valid failure

/-- Full decoding success excludes parse/guard errors and trailing-data errors.
    The latter uses the unique First suffix, which must be empty on success. -/
theorem headerFullSpec_success_not_failure (old value failed : HeaderValue) (wire : List w8)
    (error : HeaderAPIError) (bound : wire.length < 2^64)
    (success : HeaderFullSpec old wire (.success value))
    (failure : HeaderFullSpec old wire (.failure failed error)) : False := by
  cases success with
  | success first =>
    cases failure with
    | firstError failure => exact headerFirstSpec_success_not_failure old value failed wire [] error bound first failure
    | trailing second nonempty =>
      have suffix := (headerFirstSpec_success_unique old _ _ wire _ _ first second).2
      exact nonempty suffix.symm

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerItemValid_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.headerPairsValid_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.headerRecordValid_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.headerFirstSpec_success_not_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.headerFullSpec_success_not_failure
