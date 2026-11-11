import ProofCheck.IndexesDeterminism
import ProofCheck.IntsDisjoint
import ProofCheck.StructOpenDisjoint

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Selected signed arrays and ignored values have disjoint success/failure. -/
theorem indexesItemValid_not_rejected (d consumed offset extraDepth : Nat) (choice : Option Bool)
    (initial final : IndexesValue) (item rest : List w8) (reason : IndexesItemFailureReason)
    (bound : (item ++ rest).length < 2^64) (valid : IndexesItemValid d choice initial item final)
    (failure : IndexesItemRejected d choice (item ++ rest) consumed offset extraDepth reason) : False := by
  cases valid with
  | ignored valid =>
    cases failure with
    | ignored failure => exact skipValid_not_failure d offset item rest _ valid failure
  | selected valid =>
    cases failure with
    | selected failure => exact intsValid_not_rejected d consumed offset _ item rest _ _ bound valid failure

theorem indexesRecordValid_not_open_failure (d : Nat) (initial final : IndexesValue)
    (item rest : List w8) (reason : FeltFailureReason)
    (valid : IndexesRecordValid d initial item final)
    (failure : StructOpenFailure d (item ++ rest) reason) : False := by
  cases valid with
  | nullish hn =>
    have excluded := structOpenFailure_not_nullish d _ reason failure
    rcases hn with rfl | rfl
    · exact excluded.1 rfl
    · exact excluded.2 rfl
  | map enc major length depth pairs =>
    have count := indexesPairs_count_bound _ _ _ _ _ _ pairs
    refine structOpenFailure_excludes_map d _ _ _ _ reason enc major depth ?_
      (by simpa only [List.append_assoc] using failure)
    simp only [List.length_append]
    omega

/-- A successful Indexes pair sequence cannot reject a key, child
    value or later pair on the same input. -/
theorem indexesPairsValid_not_rejected (d n consumed offset extraDepth : Nat) (fields : List Bool)
    (initial final retained : IndexesValue) (item rest : List w8) (reason : IndexesRecordFailureReason)
    (bound : (item ++ rest).length < 2^64) (valid : IndexesPairsValid d n fields initial item final)
    (failure : IndexesPairsRejected d n fields initial (item ++ rest) retained consumed offset extraDepth reason) : False := by
  generalize eqWire : item ++ rest = input at failure
  induction valid generalizing input rest retained consumed offset extraDepth reason with
  | nil fields initial => cases failure
  | @step n fields choice initial middle final key value tail keyValid valueValid validTail ih =>
    cases failure with
    | key failure =>
      apply integerKeyValid_not_rejected key (value ++ tail ++ rest) choice _ _ keyValid
      rw [← eqWire] at failure
      simpa only [List.append_assoc] using failure
    | @value n2 consumed2 offset2 extraDepth2 fields2 choice2 initial2 key2 tail2 reason2 keyValid2 failure =>
      obtain ⟨keyEq, choiceEq, tailEq⟩ := integerKeyValid_prefix_unique
        key key2 (value ++ tail ++ rest) tail2 choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst choice2; subst tail2
      have valueBound : (value ++ (tail ++ rest)).length < 2^64 := by
        simp only [List.length_append] at bound ⊢; omega
      exact indexesItemValid_not_rejected d consumed2 offset2 extraDepth (indexesChoice fields choice) initial middle value
        (tail ++ rest) reason2 valueBound valueValid (by simpa only [List.append_assoc] using failure)
    | @next n2 consumed2 offset2 extraDepth2 fields2 choice2 initial2 middle2 final2 key2 value2 tail2 reason2 keyValid2 valueValid2 failure =>
      obtain ⟨keyEq, choiceEq, valuesEq⟩ := integerKeyValid_prefix_unique
        key key2 (value ++ tail ++ rest) (value2 ++ tail2) choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst choice2
      obtain ⟨valueEq, middleEq, tailsEq⟩ := indexesItemValid_prefix_unique d (indexesChoice fields choice) initial middle middle2
        value value2 (tail ++ rest) tail2 valueValid valueValid2
        (by simpa only [List.append_assoc] using valuesEq)
      subst middle2
      have tailBound : (tail ++ rest).length < 2^64 := by
        simp only [List.length_append] at bound ⊢; omega
      exact ih _ _ _ _ _ _ tailBound _ tailsEq failure

/-- Complete Indexes records have disjoint successful and failed outcomes. -/
theorem indexesRecordValid_not_rejected (d consumed offset extraDepth : Nat)
    (initial final retained : IndexesValue) (item rest : List w8) (reason : IndexesRecordFailureReason)
    (bound : (item ++ rest).length < 2^64) (valid : IndexesRecordValid d initial item final)
    (failure : IndexesRecordRejected d initial (item ++ rest) retained consumed offset extraDepth reason) : False := by
  generalize eqWire : item ++ rest = input at failure
  cases failure with
  | opening failure =>
    rw [← eqWire] at failure
    exact indexesRecordValid_not_open_failure d initial final item rest _ valid failure
  | @map head payload body argument count consumed offset extraDepth final2 reason enc major length depth fit failure =>
    cases valid with
    | nullish hn =>
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      rcases hn with rfl | rfl <;> word
    | @map head2 payload2 body2 argument2 count2 final3 enc2 major2 length2 depth2 valid =>
      obtain ⟨headEq, payloadEq, argumentEq, tailEq⟩ := encodedHead_prefix_unique head2 head payload2 payload
        (body2 ++ rest) body argument2 argument enc2 enc (by simpa only [List.append_assoc] using eqWire)
      have counts : count2 = count := by rw [argumentEq] at length2; omega
      subst count2; subst body
      have bodyBound : (body2 ++ rest).length < 2^64 := by
        simp only [List.length_append, List.length_cons] at bound ⊢; omega
      exact indexesPairsValid_not_rejected (d + 1) count consumed offset extraDepth [] initial _ _ body2 rest reason
        bodyBound valid failure

/-- First decoding cannot specify both success and failure on the same
    representable input and initial Indexes, including destination-guard errors. -/
theorem indexesFirstSpec_success_not_failure (old value failed : IndexesValue) (wire rest : List w8)
    (error : IndexesAPIError) (bound : wire.length < 2^64)
    (success : IndexesFirstSpec old wire (.success value rest))
    (failure : IndexesFirstSpec old wire (.failure failed error)) : False := by
  obtain ⟨zero, item, split, valid⟩ := indexesFirstSpec_success old value wire rest success
  subst old
  cases failure with
  | nonzero old wire nonzero => exact nonzero rfl
  | rejected failure =>
    subst wire
    exact indexesRecordValid_not_rejected _ _ _ _ _ _ _ _ _ _ bound valid failure

/-- Full decoding success excludes parse/guard errors and trailing-data errors.
    The latter uses the unique First suffix, which must be empty on success. -/
theorem indexesFullSpec_success_not_failure (old value failed : IndexesValue) (wire : List w8)
    (error : IndexesAPIError) (bound : wire.length < 2^64)
    (success : IndexesFullSpec old wire (.success value))
    (failure : IndexesFullSpec old wire (.failure failed error)) : False := by
  cases success with
  | success first =>
    cases failure with
    | firstError failure => exact indexesFirstSpec_success_not_failure old value failed wire [] error bound first failure
    | trailing second nonempty =>
      have suffix := (indexesFirstSpec_success_unique old _ _ wire _ _ first second).2
      exact nonempty suffix.symm

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesItemValid_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesPairsValid_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesRecordValid_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesFirstSpec_success_not_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesFullSpec_success_not_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesRecordValid_not_open_failure
