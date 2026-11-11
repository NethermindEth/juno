import ProofCheck.GasUsedRecordUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Null/undefined clears the destination pointer and preserves supplied storage.
    Otherwise decode into its arbitrary initial value and publish the supplied address. -/
inductive GasUsedSlotValid (d : Nat) (initial : GasUsedValue) : List w8 → List w8 → Option GasUsedValue → Prop where
  | nullish {head : w8} (rest : List w8) (hn : head = W8 246 ∨ head = W8 247) :
      GasUsedSlotValid d initial [head] rest none
  | record {item rest value}
      (hn : (item ++ rest).head? ≠ some (W8 246)) (hu : (item ++ rest).head? ≠ some (W8 247))
      (valid : GasUsedRecordValid d initial item rest value) :
      GasUsedSlotValid d initial item rest (some value)

/-- Failure preserves the old destination pointer and retains the partial supplied
    storage. Existing aliases of that storage are not claimed unchanged. -/
inductive GasUsedSlotRejected (d : Nat) (initial : GasUsedValue) : List w8 → GasUsedValue → Nat → Nat → Bool → GasUsedFailureReason → Prop where
  | record {wire retained consumed offset entered reason}
      (hn : wire.head? ≠ some (W8 246)) (hu : wire.head? ≠ some (W8 247))
      (failure : GasUsedRecordRejected d initial wire retained consumed offset entered reason) :
      GasUsedSlotRejected d initial wire retained consumed offset entered reason

theorem gasUsedSlot_classify (d : Nat) (initial : GasUsedValue) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest value, wire = item ++ rest ∧ GasUsedSlotValid d initial item rest value) ∨
    (∃ retained consumed offset entered reason, GasUsedSlotRejected d initial wire retained consumed offset entered reason) := by
  by_cases nullish : wire.head? = some (W8 246) ∨ wire.head? = some (W8 247)
  · cases wire with
    | nil => simp at nullish
    | cons head tail => exact Or.inl ⟨[head], tail, none, rfl, .nullish tail (by simpa using nullish)⟩
  have hn : wire.head? ≠ some (W8 246) := fun eq => nullish (Or.inl eq)
  have hu : wire.head? ≠ some (W8 247) := fun eq => nullish (Or.inr eq)
  rcases gasUsedRecord_classify d initial wire depth with
    ⟨item, rest, final, split, valid⟩ | ⟨final, consumed, offset, entered, reason, failure⟩
  · rw [split] at hn hu
    exact Or.inl ⟨item, rest, some final, split, .record hn hu valid⟩
  · exact Or.inr ⟨final, consumed, offset, entered, reason, .record hn hu failure⟩

theorem gasUsedSlot_nonempty (d : Nat) (initial : GasUsedValue) (item rest : List w8) (value : Option GasUsedValue)
    (valid : GasUsedSlotValid d initial item rest value) : 0 < item.length := by
  cases valid with
  | nullish => simp
  | record hn hu valid => exact gasUsedRecord_nonempty _ _ _ _ _ valid

theorem gasUsedSlotRejected_bounds (d : Nat) (initial : GasUsedValue) (consumed offset : Nat) (wire : List w8) (retained : GasUsedValue)
    (entered : Bool) (reason : GasUsedFailureReason)
    (failure : GasUsedSlotRejected d initial wire retained consumed offset entered reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | record hn hu failure => exact gasUsedRecordRejected_bounds _ _ _ _ _ _ _ _ failure

theorem gasUsedSlot_prefix_unique (d : Nat) (initial : GasUsedValue) (left right restA restB : List w8) (a b : Option GasUsedValue)
    (first : GasUsedSlotValid d initial left restA a) (second : GasUsedSlotValid d initial right restB b)
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
      obtain ⟨rfl, rfl, rfl⟩ := gasUsedRecord_prefix_unique _ _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem gasUsedSlot_not_rejected (d : Nat) (initial : GasUsedValue) (consumed offset : Nat) (item rest : List w8) (value : Option GasUsedValue)
    (retained : GasUsedValue) (entered : Bool) (reason : GasUsedFailureReason)
    (valid : GasUsedSlotValid d initial item rest value)
    (failure : GasUsedSlotRejected d initial (item ++ rest) retained consumed offset entered reason) : False := by
  cases failure with
  | record hn hu failure =>
    cases valid with
    | nullish suffix nullish =>
      rcases nullish with rfl | rfl
      · exact hn rfl
      · exact hu rfl
    | record hn2 hu2 valid => exact gasUsedRecord_not_rejected _ _ _ _ _ _ _ _ _ _ valid failure

theorem gasUsedSlotRejected_unique (d : Nat) (initial : GasUsedValue) (consumedA offsetA consumedB offsetB : Nat) (wire : List w8)
    (a b : GasUsedValue) (enteredA enteredB : Bool) (reasonA reasonB : GasUsedFailureReason)
    (first : GasUsedSlotRejected d initial wire a consumedA offsetA enteredA reasonA)
    (second : GasUsedSlotRejected d initial wire b consumedB offsetB enteredB reasonB) :
    a = b ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ enteredA = enteredB ∧ reasonA = reasonB := by
  cases first with
  | record hn hu failure => cases second with
    | record hn2 hu2 failure2 => exact gasUsedRecordRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ _ failure failure2

inductive GasUsedSlotOutcome where
  | success (value : Option GasUsedValue) (rest : List w8)
  | failure (retained : GasUsedValue) (consumed offset : Nat) (entered : Bool) (reason : GasUsedFailureReason)

def GasUsedSlotSpec (d : Nat) (initial : GasUsedValue) (wire : List w8) : GasUsedSlotOutcome → Prop
  | .success value rest => ∃ item, wire = item ++ rest ∧ GasUsedSlotValid d initial item rest value
  | .failure retained consumed offset entered reason => GasUsedSlotRejected d initial wire retained consumed offset entered reason

theorem gasUsedSlotSpec_unique (d : Nat) (initial : GasUsedValue) (wire : List w8) (a b : GasUsedSlotOutcome)
    (first : GasUsedSlotSpec d initial wire a) (second : GasUsedSlotSpec d initial wire b) : a = b := by
  cases a with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := first
    cases b with
    | success value2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      obtain ⟨_, rfl, rfl⟩ := gasUsedSlot_prefix_unique d initial item item2 rest rest2 value value2 valid valid2 (split.symm.trans split2)
      rfl
    | failure retained consumed offset entered reason =>
      subst wire
      exact False.elim (gasUsedSlot_not_rejected _ initial _ _ _ _ _ _ _ _ valid second)
  | failure retained consumed offset entered reason =>
    cases b with
    | success value rest =>
      obtain ⟨item, split, valid⟩ := second
      subst wire
      exact False.elim (gasUsedSlot_not_rejected _ initial _ _ _ _ _ _ _ _ valid first)
    | failure retained2 consumed2 offset2 entered2 reason2 =>
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := gasUsedSlotRejected_unique _ initial _ _ _ _ _ _ _ _ _ _ _ first second
      rfl

theorem gasUsedSlotSpec_total_unique (d : Nat) (initial : GasUsedValue) (wire : List w8) (depth : d ≤ 32) :
    ∃ outcome, GasUsedSlotSpec d initial wire outcome ∧
      ∀ other, GasUsedSlotSpec d initial wire other → other = outcome := by
  have existsOutcome : ∃ outcome, GasUsedSlotSpec d initial wire outcome := by
    rcases gasUsedSlot_classify d initial wire depth with
      ⟨item, rest, value, split, valid⟩ | ⟨retained, consumed, offset, entered, reason, failure⟩
    · exact ⟨.success value rest, item, split, valid⟩
    · exact ⟨.failure retained consumed offset entered reason, failure⟩
  obtain ⟨outcome, spec⟩ := existsOutcome
  exact ⟨outcome, spec, fun other otherSpec => gasUsedSlotSpec_unique d initial wire other outcome otherSpec spec⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedSlot_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedSlot_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedSlotRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedSlot_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedSlot_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedSlotRejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedSlotSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedSlotSpec_total_unique
