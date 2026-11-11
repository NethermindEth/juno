import ProofCheck.L1MessageRecordUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Pointer elements consume null/undefined before allocating. Every other
    element decodes into an initially zero L1ToL2Message; an empty map is a nonnil
    zero L1ToL2Message, distinct from a nil element. The real suffix remains explicit. -/
inductive L1MessagePointerValid (d : Nat) : List w8 → List w8 → Option L1MessageValue → Prop where
  | nullish {head : w8} (rest : List w8) (hn : head = W8 246 ∨ head = W8 247) :
      L1MessagePointerValid d [head] rest none
  | record {item rest value}
      (hn : (item ++ rest).head? ≠ some (W8 246)) (hu : (item ++ rest).head? ≠ some (W8 247))
      (valid : L1MessageRecordValid d l1MessageZero item rest value) :
      L1MessagePointerValid d item rest (some value)

/-- Failed nonnil elements retain their private partial L1ToL2Message and map depth;
    the pointer wrapper never publishes that failed allocation. -/
inductive L1MessagePointerRejected (d : Nat) : List w8 → L1MessageValue → Nat → Nat → Bool → L1MessageFailureReason → Prop where
  | record {wire retained consumed offset entered reason}
      (hn : wire.head? ≠ some (W8 246)) (hu : wire.head? ≠ some (W8 247))
      (failure : L1MessageRecordRejected d l1MessageZero wire retained consumed offset entered reason) :
      L1MessagePointerRejected d wire retained consumed offset entered reason

theorem l1MessagePointer_classify (d : Nat) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest value, wire = item ++ rest ∧ L1MessagePointerValid d item rest value) ∨
    (∃ retained consumed offset entered reason, L1MessagePointerRejected d wire retained consumed offset entered reason) := by
  by_cases nullish : wire.head? = some (W8 246) ∨ wire.head? = some (W8 247)
  · cases wire with
    | nil => simp at nullish
    | cons head tail => exact Or.inl ⟨[head], tail, none, rfl, .nullish tail (by simpa using nullish)⟩
  have hn : wire.head? ≠ some (W8 246) := fun eq => nullish (Or.inl eq)
  have hu : wire.head? ≠ some (W8 247) := fun eq => nullish (Or.inr eq)
  rcases l1MessageRecord_classify d l1MessageZero wire depth with
    ⟨item, rest, final, split, valid⟩ | ⟨final, consumed, offset, entered, reason, failure⟩
  · rw [split] at hn hu
    exact Or.inl ⟨item, rest, some final, split, .record hn hu valid⟩
  · exact Or.inr ⟨final, consumed, offset, entered, reason, .record hn hu failure⟩

theorem l1MessagePointer_nonempty (d : Nat) (item rest : List w8) (value : Option L1MessageValue)
    (valid : L1MessagePointerValid d item rest value) : 0 < item.length := by
  cases valid with
  | nullish => simp
  | record hn hu valid => exact l1MessageRecord_nonempty _ _ _ _ _ valid

theorem l1MessagePointerRejected_bounds (d consumed offset : Nat) (wire : List w8) (retained : L1MessageValue)
    (entered : Bool) (reason : L1MessageFailureReason)
    (failure : L1MessagePointerRejected d wire retained consumed offset entered reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | record hn hu failure => exact l1MessageRecordRejected_bounds _ _ _ _ _ _ _ _ failure

theorem l1MessagePointer_prefix_unique (d : Nat) (left right restA restB : List w8) (a b : Option L1MessageValue)
    (first : L1MessagePointerValid d left restA a) (second : L1MessagePointerValid d right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | nullish suffix hn =>
    cases second with
    | nullish suffix2 hn2 =>
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, rfl⟩ := same
      exact ⟨rfl, rfl, rfl⟩
    | record hn2 hu2 valid2 =>
      rw [← same] at hn2 hu2
      rcases hn with rfl | rfl
      · exact False.elim (hn2 rfl)
      · exact False.elim (hu2 rfl)
  | record hn hu valid =>
    cases second with
    | nullish suffix2 hn2 =>
      rw [same] at hn hu
      rcases hn2 with rfl | rfl
      · exact False.elim (hn rfl)
      · exact False.elim (hu rfl)
    | record hn2 hu2 valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := l1MessageRecord_prefix_unique _ _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem l1MessagePointer_not_rejected (d consumed offset : Nat) (item rest : List w8) (value : Option L1MessageValue)
    (retained : L1MessageValue) (entered : Bool) (reason : L1MessageFailureReason)
    (valid : L1MessagePointerValid d item rest value)
    (failure : L1MessagePointerRejected d (item ++ rest) retained consumed offset entered reason) : False := by
  cases failure with
  | record hn hu failure =>
    cases valid with
    | nullish suffix nullish =>
      rcases nullish with rfl | rfl
      · exact hn rfl
      · exact hu rfl
    | record hn2 hu2 valid => exact l1MessageRecord_not_rejected _ _ _ _ _ _ _ _ _ _ valid failure

theorem l1MessagePointerRejected_unique (d consumedA offsetA consumedB offsetB : Nat) (wire : List w8)
    (a b : L1MessageValue) (enteredA enteredB : Bool) (reasonA reasonB : L1MessageFailureReason)
    (first : L1MessagePointerRejected d wire a consumedA offsetA enteredA reasonA)
    (second : L1MessagePointerRejected d wire b consumedB offsetB enteredB reasonB) :
    a = b ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ enteredA = enteredB ∧ reasonA = reasonB := by
  cases first with
  | record hn hu failure => cases second with
    | record hn2 hu2 failure2 => exact l1MessageRecordRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ _ failure failure2

inductive L1MessagePointerOutcome where
  | success (value : Option L1MessageValue) (rest : List w8)
  | failure (retained : L1MessageValue) (consumed offset : Nat) (entered : Bool) (reason : L1MessageFailureReason)

def L1MessagePointerSpec (d : Nat) (wire : List w8) : L1MessagePointerOutcome → Prop
  | .success value rest => ∃ item, wire = item ++ rest ∧ L1MessagePointerValid d item rest value
  | .failure retained consumed offset entered reason => L1MessagePointerRejected d wire retained consumed offset entered reason

theorem l1MessagePointerSpec_unique (d : Nat) (wire : List w8) (a b : L1MessagePointerOutcome)
    (first : L1MessagePointerSpec d wire a) (second : L1MessagePointerSpec d wire b) : a = b := by
  cases a with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := first
    cases b with
    | success value2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      obtain ⟨_, rfl, rfl⟩ := l1MessagePointer_prefix_unique d item item2 rest rest2 value value2 valid valid2 (split.symm.trans split2)
      rfl
    | failure retained consumed offset entered reason =>
      subst wire
      exact False.elim (l1MessagePointer_not_rejected _ _ _ _ _ _ _ _ _ valid second)
  | failure retained consumed offset entered reason =>
    cases b with
    | success value rest =>
      obtain ⟨item, split, valid⟩ := second
      subst wire
      exact False.elim (l1MessagePointer_not_rejected _ _ _ _ _ _ _ _ _ valid first)
    | failure retained2 consumed2 offset2 entered2 reason2 =>
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := l1MessagePointerRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ first second
      rfl

theorem l1MessagePointerSpec_total_unique (d : Nat) (wire : List w8) (depth : d ≤ 32) :
    ∃ outcome, L1MessagePointerSpec d wire outcome ∧
      ∀ other, L1MessagePointerSpec d wire other → other = outcome := by
  have existsOutcome : ∃ outcome, L1MessagePointerSpec d wire outcome := by
    rcases l1MessagePointer_classify d wire depth with
      ⟨item, rest, value, split, valid⟩ | ⟨retained, consumed, offset, entered, reason, failure⟩
    · exact ⟨.success value rest, item, split, valid⟩
    · exact ⟨.failure retained consumed offset entered reason, failure⟩
  obtain ⟨outcome, spec⟩ := existsOutcome
  exact ⟨outcome, spec, fun other otherSpec => l1MessagePointerSpec_unique d wire other outcome otherSpec spec⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessagePointer_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessagePointer_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessagePointerRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessagePointer_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessagePointer_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessagePointerRejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessagePointerSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessagePointerSpec_total_unique
