import ProofCheck.BigIntUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Null and undefined clear the pointer. Other inputs must encode an integer,
    including unsigned/negative machine-width integers and bignum tags 2 or 3. -/
inductive BigIntPointerValid : List w8 → Option Int → Prop where
  | nullish {head : w8} (hn : head = W8 246 ∨ head = W8 247) : BigIntPointerValid [head] none
  | integer {item body} (valid : BigIntValid item body) : BigIntPointerValid item (some body)

def BigIntPointerRejected (wire : List w8) (reason : BigIntFailureReason) : Prop :=
  wire.head? ≠ some (W8 246) ∧ wire.head? ≠ some (W8 247) ∧ BigIntRejected wire reason

theorem bigIntPointer_classify (wire : List w8) :
    (∃ item rest value, wire = item ++ rest ∧ BigIntPointerValid item value) ∨
    (∃ reason, BigIntPointerRejected wire reason) := by
  by_cases hn : wire.head? = some (W8 246) ∨ wire.head? = some (W8 247)
  · cases wire with
    | nil => simp at hn
    | cons head rest =>
      simp only [List.head?_cons, Option.some.injEq] at hn
      exact Or.inl ⟨[head], rest, none, rfl, .nullish hn⟩
  · have hn1 : wire.head? ≠ some (W8 246) := fun h => hn (Or.inl h)
    have hn2 : wire.head? ≠ some (W8 247) := fun h => hn (Or.inr h)
    rcases bigInt_classify wire with ⟨item, rest, body, split, valid⟩ | ⟨reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .integer valid⟩
    · exact Or.inr ⟨reason, hn1, hn2, failure⟩

theorem bigIntPointerValid_nonempty (value : Option Int) (item : List w8)
    (valid : BigIntPointerValid item value) : 0 < item.length := by
  cases valid with
  | nullish => simp
  | integer valid => exact bigIntValid_nonempty _ _ valid

theorem bigIntPointerValid_prefix_unique (a b : Option Int) (left right restA restB : List w8)
    (first : BigIntPointerValid left a) (second : BigIntPointerValid right b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | nullish hn =>
    cases second with
    | nullish ho =>
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, rfl⟩ := same
      exact ⟨rfl, rfl, rfl⟩
    | integer valid =>
      have nonnull := bigIntValid_not_nullish _ restB _ valid
      rcases hn with rfl | rfl
      · exact False.elim (nonnull.1 (by rw [← same]; rfl))
      · exact False.elim (nonnull.2 (by rw [← same]; rfl))
  | integer valid =>
    cases second with
    | nullish hn =>
      have nonnull := bigIntValid_not_nullish _ restA _ valid
      rcases hn with rfl | rfl
      · exact False.elim (nonnull.1 (by rw [same]; rfl))
      · exact False.elim (nonnull.2 (by rw [same]; rfl))
    | integer valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := bigIntValid_prefix_unique _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem bigIntPointerValid_not_rejected (value : Option Int) (item rest : List w8)
    (reason : BigIntFailureReason) (valid : BigIntPointerValid item value)
    (failure : BigIntPointerRejected (item ++ rest) reason) : False := by
  cases valid with
  | nullish hn =>
    rcases hn with rfl | rfl
    · exact failure.1 rfl
    · exact failure.2.1 rfl
  | integer valid => exact bigIntValid_not_rejected _ _ _ _ valid failure.2.2

theorem bigIntPointerRejected_offset_bounds (wire : List w8) (reason : BigIntFailureReason)
    (failure : BigIntPointerRejected wire reason) : bigIntDiagnosticOffset reason ≤ wire.length :=
  bigIntRejected_offset_bounds wire reason failure.2.2

inductive BigIntPointerResult where
  | success (value : Option Int) (rest : List w8)
  | failure (reason : BigIntFailureReason)

def BigIntPointerSpec (wire : List w8) : BigIntPointerResult → Prop
  | .success value rest => ∃ item, wire = item ++ rest ∧ BigIntPointerValid item value
  | .failure reason => BigIntPointerRejected wire reason

theorem bigIntPointerSpec_unique (wire : List w8) (a b : BigIntPointerResult)
    (first : BigIntPointerSpec wire a) (second : BigIntPointerSpec wire b) : a = b := by
  cases a with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := first
    cases b with
    | success value2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      obtain ⟨_, rfl, rfl⟩ := bigIntPointerValid_prefix_unique value value2 item item2 rest rest2 valid valid2 (split.symm.trans split2)
      rfl
    | failure reason =>
      subst wire
      exact False.elim (bigIntPointerValid_not_rejected _ _ _ _ valid second)
  | failure reason =>
    cases b with
    | success value rest =>
      obtain ⟨item, split, valid⟩ := second
      subst wire
      exact False.elim (bigIntPointerValid_not_rejected _ _ _ _ valid first)
    | failure reason2 => exact congrArg BigIntPointerResult.failure (bigIntRejected_unique _ _ _ first.2.2 second.2.2)

theorem bigIntPointerSpec_total_unique (wire : List w8) :
    ∃ outcome, BigIntPointerSpec wire outcome ∧ ∀ other, BigIntPointerSpec wire other → other = outcome := by
  have existsOutcome : ∃ outcome, BigIntPointerSpec wire outcome := by
    rcases bigIntPointer_classify wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
    · exact ⟨.success value rest, item, split, valid⟩
    · exact ⟨.failure reason, failure⟩
  obtain ⟨outcome, spec⟩ := existsOutcome
  exact ⟨outcome, spec, fun other otherSpec => bigIntPointerSpec_unique wire other outcome otherSpec spec⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.bigIntPointer_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.bigIntPointerValid_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.bigIntPointerValid_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.bigIntPointerValid_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.bigIntPointerSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.bigIntPointerSpec_total_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.bigIntPointerRejected_offset_bounds
