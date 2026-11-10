import ProofCheck.GasPriceDeterminism
import ProofCheck.HeaderAPISpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Equal schema identities choose one field reader and produce one complete
    Header update, including every nested value and exact byte boundary. -/
theorem headerItemValid_prefix_unique (d : Nat) (choice choice2 : Option HeaderField)
    (initial a b : HeaderValue) (left right restLeft restRight : List w8)
    (first : HeaderItemValid d choice initial left a) (second : HeaderItemValid d choice2 initial right b)
    (choiceEq : choice = choice2) (same : left ++ restLeft = right ++ restRight) :
    left = right ∧ a = b ∧ restLeft = restRight := by
  cases first with
  | felt field valid =>
    cases second with
    | felt field2 valid2 =>
      have fieldEq : field = field2 := by
        cases field <;> cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
      subst field2
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique d _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
    | scalar field2 valid2 =>
      cases field <;> cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | signatures valid2 =>
      cases field <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | string valid2 =>
      cases field <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | gas field2 valid2 =>
      cases field <;> cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | ignored valid2 =>
      cases field <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
  | scalar field valid =>
    cases second with
    | felt field2 valid2 =>
      cases field <;> cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | scalar field2 valid2 =>
      have fieldEq : field = field2 := by
        cases field <;> cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
      subst field2
      obtain ⟨rfl, rfl, rfl⟩ := scalarValid_prefix_unique (headerScalarValue field initial) _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
    | signatures valid2 =>
      cases field <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | string valid2 =>
      cases field <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | gas field2 valid2 =>
      cases field <;> cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | ignored valid2 =>
      cases field <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
  | signatures valid =>
    cases second with
    | felt field2 valid2 =>
      cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | scalar field2 valid2 =>
      cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | signatures valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := signaturesValid_prefix_unique d _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
    | string valid2 =>
      simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | gas field2 valid2 =>
      cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | ignored valid2 =>
      simp_all [headerFeltKey, headerScalarKey, headerGasKey]
  | string valid =>
    cases second with
    | felt field2 valid2 =>
      cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | scalar field2 valid2 =>
      cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | signatures valid2 =>
      simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | string valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := stringValid_prefix_unique initial.protocolVersion _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
    | gas field2 valid2 =>
      cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | ignored valid2 =>
      simp_all [headerFeltKey, headerScalarKey, headerGasKey]
  | gas field valid =>
    cases second with
    | felt field2 valid2 =>
      cases field <;> cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | scalar field2 valid2 =>
      cases field <;> cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | signatures valid2 =>
      cases field <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | string valid2 =>
      cases field <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | gas field2 valid2 =>
      have fieldEq : field = field2 := by
        cases field <;> cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
      subst field2
      obtain ⟨rfl, rfl, rfl⟩ := gasPointerValid_prefix_unique d _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
    | ignored valid2 =>
      cases field <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
  | ignored valid =>
    cases second with
    | felt field2 valid2 =>
      cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | scalar field2 valid2 =>
      cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | signatures valid2 =>
      simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | string valid2 =>
      simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | gas field2 valid2 =>
      cases field2 <;> simp_all [headerFeltKey, headerScalarKey, headerGasKey]
    | ignored valid2 =>
      obtain ⟨rfl, rfl⟩ := skipValid_prefix_unique d _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

/-- Key and child determinism compose through arbitrary successful pair order,
    preserving the complete Header value and exact final suffix. -/
theorem headerPairsValid_prefix_unique (d n : Nat) (fields : List HeaderField) (initial a b : HeaderValue)
    (left right restLeft restRight : List w8)
    (first : HeaderPairsValid d n fields initial left a) (second : HeaderPairsValid d n fields initial right b)
    (same : left ++ restLeft = right ++ restRight) :
    left = right ∧ a = b ∧ restLeft = restRight := by
  induction first generalizing right b restLeft restRight with
  | nil fields initial =>
    cases second
    simpa using same
  | @step n fields choice initial middle final key value tail keyValid fresh valueValid rest ih =>
    cases second with
    | @step n2 fields2 choice2 initial2 middle2 final2 key2 value2 tail2 keyValid2 fresh2 valueValid2 rest2 =>
      obtain ⟨keyEq, choiceEq, valuesEq⟩ := schemaKey_prefix_unique HeaderKeyTextMeaning headerKeyMeaning_unique
        key key2 (value ++ tail ++ restLeft) (value2 ++ tail2 ++ restRight) choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using same)
      subst key2
      subst choice2
      obtain ⟨valueEq, middleEq, tailsEq⟩ := headerItemValid_prefix_unique d choice choice initial middle middle2
        value value2 (tail ++ restLeft) (tail2 ++ restRight) valueValid valueValid2 rfl
        (by simpa only [List.append_assoc] using valuesEq)
      subst value2
      subst middle2
      obtain ⟨tailEq, finalEq, suffixEq⟩ := ih _ _ _ _ rest2 tailsEq
      exact ⟨by rw [tailEq], finalEq, suffixEq⟩

/-- The complete headerRecordValid success relation has one value and exact suffix
    for a given input, independent of map field ordering and head widths. -/
theorem headerRecordValid_prefix_unique (d : Nat) (initial : HeaderValue) (a b : HeaderValue)
    (left right restLeft restRight : List w8)
    (first : HeaderRecordValid d initial left a) (second : HeaderRecordValid d initial right b)
    (same : left ++ restLeft = right ++ restRight) :
    left = right ∧ a = b ∧ restLeft = restRight := by
  cases first with
  | @nullish head hn =>
    cases second with
    | @nullish other ho =>
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, rfl⟩ := same
      exact ⟨rfl, rfl, rfl⟩
    | @map other payload body argument count final enc major size depth valid =>
      simp only [List.append_assoc, List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn with rfl | rfl <;> contradiction
  | @map head payload body argument count final enc major size depth valid =>
    cases second with
    | @nullish other ho =>
      simp only [List.append_assoc, List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases ho with rfl | rfl <;> contradiction
    | @map other payload2 body2 argument2 count2 final2 enc2 major2 size2 depth2 valid2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      obtain ⟨payloadEq, argumentEq, tailsEq⟩ := headArgument_prefix_unique head payload payload2
        (body ++ restLeft) (body2 ++ restRight) argument argument2 enc enc2 same
      subst payload2
      subst argument2
      have counts : count = count2 := by omega
      subst count2
      obtain ⟨bodyEq, valueEq, restEq⟩ := headerPairsValid_prefix_unique (d + 1) count [] _ _ _
        body body2 restLeft restRight valid valid2 tailsEq
      exact ⟨by rw [bodyEq], valueEq, restEq⟩

/-- Successful public First outcomes have one decoded Header and exact suffix.
    This is specification determinism, not a general production WP theorem. -/
theorem headerFirstSpec_success_unique (old a b : HeaderValue) (wire restA restB : List w8)
    (first : HeaderFirstSpec old wire (.success a restA))
    (second : HeaderFirstSpec old wire (.success b restB)) : a = b ∧ restA = restB := by
  obtain ⟨zero, left, split, valid⟩ := headerFirstSpec_success old a wire restA first
  obtain ⟨_, right, split2, valid2⟩ := headerFirstSpec_success old b wire restB second
  exact (headerRecordValid_prefix_unique 0 headerZeroValue a b left right restA restB
    valid valid2 (split.symm.trans split2)).2

/-- Successful full decoding has exactly one application value. -/
theorem headerFullSpec_success_unique (old a b : HeaderValue) (wire : List w8)
    (first : HeaderFullSpec old wire (.success a)) (second : HeaderFullSpec old wire (.success b)) : a = b := by
  obtain ⟨_, valid⟩ := headerFullSpec_success old a wire first
  obtain ⟨_, valid2⟩ := headerFullSpec_success old b wire second
  exact (headerRecordValid_prefix_unique 0 headerZeroValue a b wire wire [] [] valid valid2 rfl).2.1

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerItemValid_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.headerPairsValid_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.headerRecordValid_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.headerFirstSpec_success_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.headerFullSpec_success_unique
