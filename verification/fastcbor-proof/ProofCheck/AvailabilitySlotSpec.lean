import ProofCheck.AvailabilityRecordUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Null/undefined clears the destination pointer and preserves supplied storage.
    Otherwise decode into its arbitrary initial value and publish the supplied address. -/
inductive AvailabilitySlotValid (d : Nat) (initial : AvailabilityValue) : List w8 → List w8 → Option AvailabilityValue → Prop where
  | nullish {head : w8} (rest : List w8) (hn : head = W8 246 ∨ head = W8 247) :
      AvailabilitySlotValid d initial [head] rest none
  | record {item rest value}
      (hn : (item ++ rest).head? ≠ some (W8 246)) (hu : (item ++ rest).head? ≠ some (W8 247))
      (valid : AvailabilityRecordValid d initial item rest value) :
      AvailabilitySlotValid d initial item rest (some value)

/-- Failure preserves the old destination pointer and retains the partial supplied
    storage. Existing aliases of that storage are not claimed unchanged. -/
inductive AvailabilitySlotRejected (d : Nat) (initial : AvailabilityValue) : List w8 → AvailabilityValue → Nat → Nat → Bool → AvailabilityFailureReason → Prop where
  | record {wire retained consumed offset entered reason}
      (hn : wire.head? ≠ some (W8 246)) (hu : wire.head? ≠ some (W8 247))
      (failure : AvailabilityRecordRejected d initial wire retained consumed offset entered reason) :
      AvailabilitySlotRejected d initial wire retained consumed offset entered reason

theorem availabilitySlot_classify (d : Nat) (initial : AvailabilityValue) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest value, wire = item ++ rest ∧ AvailabilitySlotValid d initial item rest value) ∨
    (∃ retained consumed offset entered reason, AvailabilitySlotRejected d initial wire retained consumed offset entered reason) := by
  by_cases nullish : wire.head? = some (W8 246) ∨ wire.head? = some (W8 247)
  · cases wire with
    | nil => simp at nullish
    | cons head tail => exact Or.inl ⟨[head], tail, none, rfl, .nullish tail (by simpa using nullish)⟩
  have hn : wire.head? ≠ some (W8 246) := fun eq => nullish (Or.inl eq)
  have hu : wire.head? ≠ some (W8 247) := fun eq => nullish (Or.inr eq)
  rcases availabilityRecord_classify d initial wire depth with
    ⟨item, rest, final, split, valid⟩ | ⟨final, consumed, offset, entered, reason, failure⟩
  · rw [split] at hn hu
    exact Or.inl ⟨item, rest, some final, split, .record hn hu valid⟩
  · exact Or.inr ⟨final, consumed, offset, entered, reason, .record hn hu failure⟩

theorem availabilitySlot_nonempty (d : Nat) (initial : AvailabilityValue) (item rest : List w8) (value : Option AvailabilityValue)
    (valid : AvailabilitySlotValid d initial item rest value) : 0 < item.length := by
  cases valid with
  | nullish => simp
  | record hn hu valid => exact availabilityRecord_nonempty _ _ _ _ _ valid

theorem availabilitySlotRejected_bounds (d : Nat) (initial : AvailabilityValue) (consumed offset : Nat) (wire : List w8) (retained : AvailabilityValue)
    (entered : Bool) (reason : AvailabilityFailureReason)
    (failure : AvailabilitySlotRejected d initial wire retained consumed offset entered reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | record hn hu failure => exact availabilityRecordRejected_bounds _ _ _ _ _ _ _ _ failure

theorem availabilitySlot_prefix_unique (d : Nat) (initial : AvailabilityValue) (left right restA restB : List w8) (a b : Option AvailabilityValue)
    (first : AvailabilitySlotValid d initial left restA a) (second : AvailabilitySlotValid d initial right restB b)
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
      obtain ⟨rfl, rfl, rfl⟩ := availabilityRecord_prefix_unique _ _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem availabilitySlot_not_rejected (d : Nat) (initial : AvailabilityValue) (consumed offset : Nat) (item rest : List w8) (value : Option AvailabilityValue)
    (retained : AvailabilityValue) (entered : Bool) (reason : AvailabilityFailureReason)
    (valid : AvailabilitySlotValid d initial item rest value)
    (failure : AvailabilitySlotRejected d initial (item ++ rest) retained consumed offset entered reason) : False := by
  cases failure with
  | record hn hu failure =>
    cases valid with
    | nullish suffix nullish =>
      rcases nullish with rfl | rfl
      · exact hn rfl
      · exact hu rfl
    | record hn2 hu2 valid => exact availabilityRecord_not_rejected _ _ _ _ _ _ _ _ _ _ valid failure

theorem availabilitySlotRejected_unique (d : Nat) (initial : AvailabilityValue) (consumedA offsetA consumedB offsetB : Nat) (wire : List w8)
    (a b : AvailabilityValue) (enteredA enteredB : Bool) (reasonA reasonB : AvailabilityFailureReason)
    (first : AvailabilitySlotRejected d initial wire a consumedA offsetA enteredA reasonA)
    (second : AvailabilitySlotRejected d initial wire b consumedB offsetB enteredB reasonB) :
    a = b ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ enteredA = enteredB ∧ reasonA = reasonB := by
  cases first with
  | record hn hu failure => cases second with
    | record hn2 hu2 failure2 => exact availabilityRecordRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ _ failure failure2

inductive AvailabilitySlotOutcome where
  | success (value : Option AvailabilityValue) (rest : List w8)
  | failure (retained : AvailabilityValue) (consumed offset : Nat) (entered : Bool) (reason : AvailabilityFailureReason)

def AvailabilitySlotSpec (d : Nat) (initial : AvailabilityValue) (wire : List w8) : AvailabilitySlotOutcome → Prop
  | .success value rest => ∃ item, wire = item ++ rest ∧ AvailabilitySlotValid d initial item rest value
  | .failure retained consumed offset entered reason => AvailabilitySlotRejected d initial wire retained consumed offset entered reason

theorem availabilitySlotSpec_unique (d : Nat) (initial : AvailabilityValue) (wire : List w8) (a b : AvailabilitySlotOutcome)
    (first : AvailabilitySlotSpec d initial wire a) (second : AvailabilitySlotSpec d initial wire b) : a = b := by
  cases a with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := first
    cases b with
    | success value2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      obtain ⟨_, rfl, rfl⟩ := availabilitySlot_prefix_unique d initial item item2 rest rest2 value value2 valid valid2 (split.symm.trans split2)
      rfl
    | failure retained consumed offset entered reason =>
      subst wire
      exact False.elim (availabilitySlot_not_rejected _ initial _ _ _ _ _ _ _ _ valid second)
  | failure retained consumed offset entered reason =>
    cases b with
    | success value rest =>
      obtain ⟨item, split, valid⟩ := second
      subst wire
      exact False.elim (availabilitySlot_not_rejected _ initial _ _ _ _ _ _ _ _ valid first)
    | failure retained2 consumed2 offset2 entered2 reason2 =>
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := availabilitySlotRejected_unique _ initial _ _ _ _ _ _ _ _ _ _ _ first second
      rfl

theorem availabilitySlotSpec_total_unique (d : Nat) (initial : AvailabilityValue) (wire : List w8) (depth : d ≤ 32) :
    ∃ outcome, AvailabilitySlotSpec d initial wire outcome ∧
      ∀ other, AvailabilitySlotSpec d initial wire other → other = outcome := by
  have existsOutcome : ∃ outcome, AvailabilitySlotSpec d initial wire outcome := by
    rcases availabilitySlot_classify d initial wire depth with
      ⟨item, rest, value, split, valid⟩ | ⟨retained, consumed, offset, entered, reason, failure⟩
    · exact ⟨.success value rest, item, split, valid⟩
    · exact ⟨.failure retained consumed offset entered reason, failure⟩
  obtain ⟨outcome, spec⟩ := existsOutcome
  exact ⟨outcome, spec, fun other otherSpec => availabilitySlotSpec_unique d initial wire other outcome otherSpec spec⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilitySlot_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilitySlot_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilitySlotRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilitySlot_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilitySlot_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilitySlotRejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilitySlotSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilitySlotSpec_total_unique
