import ProofCheck.EventRecordUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Pointer elements consume null/undefined before allocating. Every other
    element decodes into an initially zero Event; an empty map is a nonnil
    zero Event, distinct from a nil element. The real suffix remains explicit. -/
inductive EventPointerValid (d : Nat) : List w8 → List w8 → Option EventValue → Prop where
  | nullish {head : w8} (rest : List w8) (hn : head = W8 246 ∨ head = W8 247) :
      EventPointerValid d [head] rest none
  | record {item rest value}
      (hn : (item ++ rest).head? ≠ some (W8 246)) (hu : (item ++ rest).head? ≠ some (W8 247))
      (valid : EventRecordValid d eventZero item rest value) :
      EventPointerValid d item rest (some value)

/-- Failed nonnil elements retain their private partial Event and map depth;
    the enclosing slice never publishes that element or its temporary backing. -/
inductive EventPointerRejected (d : Nat) : List w8 → EventValue → Nat → Nat → Bool → EventFailureReason → Prop where
  | record {wire retained consumed offset entered reason}
      (hn : wire.head? ≠ some (W8 246)) (hu : wire.head? ≠ some (W8 247))
      (failure : EventRecordRejected d eventZero wire retained consumed offset entered reason) :
      EventPointerRejected d wire retained consumed offset entered reason

theorem eventPointer_classify (d : Nat) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest value, wire = item ++ rest ∧ EventPointerValid d item rest value) ∨
    (∃ retained consumed offset entered reason, EventPointerRejected d wire retained consumed offset entered reason) := by
  by_cases nullish : wire.head? = some (W8 246) ∨ wire.head? = some (W8 247)
  · cases wire with
    | nil => simp at nullish
    | cons head tail => exact Or.inl ⟨[head], tail, none, rfl, .nullish tail (by simpa using nullish)⟩
  have hn : wire.head? ≠ some (W8 246) := fun eq => nullish (Or.inl eq)
  have hu : wire.head? ≠ some (W8 247) := fun eq => nullish (Or.inr eq)
  rcases eventRecord_classify d eventZero wire depth with
    ⟨item, rest, final, split, valid⟩ | ⟨final, consumed, offset, entered, reason, failure⟩
  · rw [split] at hn hu
    exact Or.inl ⟨item, rest, some final, split, .record hn hu valid⟩
  · exact Or.inr ⟨final, consumed, offset, entered, reason, .record hn hu failure⟩

theorem eventPointer_nonempty (d : Nat) (item rest : List w8) (value : Option EventValue)
    (valid : EventPointerValid d item rest value) : 0 < item.length := by
  cases valid with
  | nullish => simp
  | record hn hu valid => exact eventRecord_nonempty _ _ _ _ _ valid

theorem eventPointerRejected_bounds (d consumed offset : Nat) (wire : List w8) (retained : EventValue)
    (entered : Bool) (reason : EventFailureReason)
    (failure : EventPointerRejected d wire retained consumed offset entered reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | record hn hu failure => exact eventRecordRejected_bounds _ _ _ _ _ _ _ _ failure

/-- Successful element sequences retain the entire later input when checking
    each record, including the suffix beyond the enclosing array. -/
inductive EventPointerItemsValid (d : Nat) : Nat → List w8 → List w8 → List (Option EventValue) → Prop where
  | nil (suffix : List w8) : EventPointerItemsValid d 0 [] suffix []
  | step {n item tail suffix value values}
      (valid : EventPointerValid d item (tail ++ suffix) value)
      (rest : EventPointerItemsValid d n tail suffix values) :
      EventPointerItemsValid d (n + 1) (item ++ tail) suffix (value :: values)

inductive EventPointerItemsRejected (d : Nat) : Nat → List w8 → List (Option EventValue) → EventValue →
    Nat → Nat → Bool → EventFailureReason → Prop where
  | here {n wire retained consumed offset entered reason}
      (failure : EventPointerRejected d wire retained consumed offset entered reason) :
      EventPointerItemsRejected d (n + 1) wire [] retained consumed offset entered reason
  | next {n item tail value done retained consumed offset entered reason}
      (valid : EventPointerValid d item tail value)
      (failure : EventPointerItemsRejected d n tail done retained consumed offset entered reason) :
      EventPointerItemsRejected d (n + 1) (item ++ tail) (value :: done) retained
        (item.length + consumed) (item.length + offset) entered reason

/-- Every fixed-length element sequence succeeds or fails at a candidate
    element; uniqueness itself is proved separately. -/
theorem eventPointerItems_classify (d n : Nat) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest values, wire = item ++ rest ∧ EventPointerItemsValid d n item rest values) ∨
    (∃ done retained consumed offset entered reason,
      EventPointerItemsRejected d n wire done retained consumed offset entered reason) := by
  induction n generalizing wire with
  | zero => exact Or.inl ⟨[], wire, [], rfl, .nil wire⟩
  | succ n ih =>
    rcases eventPointer_classify d wire depth with
      ⟨item, tail, value, split, valid⟩ | ⟨retained, consumed, offset, entered, reason, failure⟩
    · subst wire
      rcases ih tail with ⟨items, rest, values, split, restValid⟩ |
        ⟨done, retained, consumed, offset, entered, reason, failure⟩
      · subst tail
        exact Or.inl ⟨item ++ items, rest, value :: values, (List.append_assoc _ _ _).symm, .step valid restValid⟩
      · exact Or.inr ⟨value :: done, retained, item.length + consumed, item.length + offset,
          entered, reason, .next valid failure⟩
    · exact Or.inr ⟨[], retained, consumed, offset, entered, reason, .here failure⟩

theorem eventPointerItems_count (d n : Nat) (item rest : List w8) (values : List (Option EventValue))
    (valid : EventPointerItemsValid d n item rest values) : values.length = n ∧ n ≤ item.length := by
  induction valid with
  | nil => simp
  | step valid rest ih =>
    have nonempty := eventPointer_nonempty _ _ _ _ valid
    simp only [List.length_cons, List.length_append]
    omega

theorem eventPointerItemsRejected_bounds (d n consumed offset : Nat) (wire : List w8)
    (done : List (Option EventValue)) (retained : EventValue) (entered : Bool) (reason : EventFailureReason)
    (failure : EventPointerItemsRejected d n wire done retained consumed offset entered reason) :
    done.length < n ∧ consumed ≤ offset ∧ offset ≤ wire.length := by
  induction failure with
  | here failure =>
    have bounds := eventPointerRejected_bounds _ _ _ _ _ _ _ failure
    simp only [List.length_nil]
    omega
  | next valid failure ih => simp only [List.length_cons, List.length_append]; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.eventPointer_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.eventPointer_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.eventPointerRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.eventPointerItems_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.eventPointerItems_count
#print axioms Perennial.github_com.NethermindEth.juno.core.eventPointerItemsRejected_bounds
