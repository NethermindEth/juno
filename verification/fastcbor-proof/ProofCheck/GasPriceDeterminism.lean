import ProofCheck.SignaturesDeterminism
import ProofCheck.GasPriceSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

theorem gasValueValid_prefix_unique (d : Nat) (choice : Option Bool) (initial a b : GasPriceValue)
    (left right restLeft restRight : List w8)
    (first : GasValueValid d choice initial left a) (second : GasValueValid d choice initial right b)
    (same : left ++ restLeft = right ++ restRight) :
    left = right ∧ a = b ∧ restLeft = restRight := by
  cases first with
  | amount valid =>
    cases second with
    | amount valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique d _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | ignored valid =>
    cases second with
    | ignored valid2 =>
      obtain ⟨rfl, rfl⟩ := skipValid_prefix_unique d _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

/-- Key and child determinism compose through arbitrary successful pair order,
    preserving the complete GasPrice value and exact final suffix. -/
theorem gasPairsValid_prefix_unique (d n : Nat) (fields : List Bool) (initial a b : GasPriceValue)
    (left right restLeft restRight : List w8)
    (first : GasPairsValid d n fields initial left a) (second : GasPairsValid d n fields initial right b)
    (same : left ++ restLeft = right ++ restRight) :
    left = right ∧ a = b ∧ restLeft = restRight := by
  induction first generalizing right b restLeft restRight with
  | nil fields initial =>
    cases second
    simpa using same
  | @step n fields choice initial middle final key value tail keyValid fresh valueValid rest ih =>
    cases second with
    | @step n2 fields2 choice2 initial2 middle2 final2 key2 value2 tail2 keyValid2 fresh2 valueValid2 rest2 =>
      obtain ⟨keyEq, choiceEq, valuesEq⟩ := schemaKey_prefix_unique GasKeyTextMeaning gasKeyMeaning_unique
        key key2 (value ++ tail ++ restLeft) (value2 ++ tail2 ++ restRight) choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using same)
      subst key2
      subst choice2
      obtain ⟨valueEq, middleEq, tailsEq⟩ := gasValueValid_prefix_unique d choice initial middle middle2
        value value2 (tail ++ restLeft) (tail2 ++ restRight) valueValid valueValid2
        (by simpa only [List.append_assoc] using valuesEq)
      subst value2
      subst middle2
      obtain ⟨tailEq, finalEq, suffixEq⟩ := ih _ _ _ _ rest2 tailsEq
      exact ⟨by rw [tailEq], finalEq, suffixEq⟩

/-- The complete gasRecordValid success relation has one value and exact suffix
    for a given input, independent of map field ordering and head widths. -/
theorem gasRecordValid_prefix_unique (d : Nat) (initial : GasPriceValue) (a b : GasPriceValue)
    (left right restLeft restRight : List w8)
    (first : GasRecordValid d initial left a) (second : GasRecordValid d initial right b)
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
      obtain ⟨bodyEq, valueEq, restEq⟩ := gasPairsValid_prefix_unique (d + 1) count [] _ _ _
        body body2 restLeft restRight valid valid2 tailsEq
      exact ⟨by rw [bodyEq], valueEq, restEq⟩

/-- The complete gasPointerValid success relation has one value and exact suffix
    for a given input, independent of map field ordering and head widths. -/
theorem gasPointerValid_prefix_unique (d : Nat) (a b : Option GasPriceValue)
    (left right restLeft restRight : List w8)
    (first : GasPointerValid d left a) (second : GasPointerValid d right b)
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
      obtain ⟨bodyEq, valueEq, restEq⟩ := gasPairsValid_prefix_unique (d + 1) count [] _ _ _
        body body2 restLeft restRight valid valid2 tailsEq
      exact ⟨by rw [bodyEq], congrArg some valueEq, restEq⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.gasValueValid_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.gasPairsValid_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.gasRecordValid_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.gasPointerValid_prefix_unique
