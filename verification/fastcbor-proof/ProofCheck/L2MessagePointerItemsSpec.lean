import ProofCheck.L2MessageRecordUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Pointer elements consume null/undefined before allocating. Every other
    element decodes into an initially zero L2ToL1Message; an empty map is a nonnil
    zero L2ToL1Message, distinct from a nil element. The real suffix remains explicit. -/
inductive L2MessagePointerValid (d : Nat) : List w8 → List w8 → Option L2MessageValue → Prop where
  | nullish {head : w8} (rest : List w8) (hn : head = W8 246 ∨ head = W8 247) :
      L2MessagePointerValid d [head] rest none
  | record {item rest value}
      (hn : (item ++ rest).head? ≠ some (W8 246)) (hu : (item ++ rest).head? ≠ some (W8 247))
      (valid : L2MessageRecordValid d l2MessageZero item rest value) :
      L2MessagePointerValid d item rest (some value)

/-- Failed nonnil elements retain their private partial L2ToL1Message and map depth;
    the enclosing slice never publishes that element or its temporary backing. -/
inductive L2MessagePointerRejected (d : Nat) : List w8 → L2MessageValue → Nat → Nat → Bool → L2MessageFailureReason → Prop where
  | record {wire retained consumed offset entered reason}
      (hn : wire.head? ≠ some (W8 246)) (hu : wire.head? ≠ some (W8 247))
      (failure : L2MessageRecordRejected d l2MessageZero wire retained consumed offset entered reason) :
      L2MessagePointerRejected d wire retained consumed offset entered reason

theorem l2MessagePointer_classify (d : Nat) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest value, wire = item ++ rest ∧ L2MessagePointerValid d item rest value) ∨
    (∃ retained consumed offset entered reason, L2MessagePointerRejected d wire retained consumed offset entered reason) := by
  by_cases nullish : wire.head? = some (W8 246) ∨ wire.head? = some (W8 247)
  · cases wire with
    | nil => simp at nullish
    | cons head tail => exact Or.inl ⟨[head], tail, none, rfl, .nullish tail (by simpa using nullish)⟩
  have hn : wire.head? ≠ some (W8 246) := fun eq => nullish (Or.inl eq)
  have hu : wire.head? ≠ some (W8 247) := fun eq => nullish (Or.inr eq)
  rcases l2MessageRecord_classify d l2MessageZero wire depth with
    ⟨item, rest, final, split, valid⟩ | ⟨final, consumed, offset, entered, reason, failure⟩
  · rw [split] at hn hu
    exact Or.inl ⟨item, rest, some final, split, .record hn hu valid⟩
  · exact Or.inr ⟨final, consumed, offset, entered, reason, .record hn hu failure⟩

theorem l2MessagePointer_nonempty (d : Nat) (item rest : List w8) (value : Option L2MessageValue)
    (valid : L2MessagePointerValid d item rest value) : 0 < item.length := by
  cases valid with
  | nullish => simp
  | record hn hu valid => exact l2MessageRecord_nonempty _ _ _ _ _ valid

theorem l2MessagePointerRejected_bounds (d consumed offset : Nat) (wire : List w8) (retained : L2MessageValue)
    (entered : Bool) (reason : L2MessageFailureReason)
    (failure : L2MessagePointerRejected d wire retained consumed offset entered reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | record hn hu failure => exact l2MessageRecordRejected_bounds _ _ _ _ _ _ _ _ failure

/-- Successful element sequences retain the entire later input when checking
    each record, including the suffix beyond the enclosing array. -/
inductive L2MessagePointerItemsValid (d : Nat) : Nat → List w8 → List w8 → List (Option L2MessageValue) → Prop where
  | nil (suffix : List w8) : L2MessagePointerItemsValid d 0 [] suffix []
  | step {n item tail suffix value values}
      (valid : L2MessagePointerValid d item (tail ++ suffix) value)
      (rest : L2MessagePointerItemsValid d n tail suffix values) :
      L2MessagePointerItemsValid d (n + 1) (item ++ tail) suffix (value :: values)

inductive L2MessagePointerItemsRejected (d : Nat) : Nat → List w8 → List (Option L2MessageValue) → L2MessageValue →
    Nat → Nat → Bool → L2MessageFailureReason → Prop where
  | here {n wire retained consumed offset entered reason}
      (failure : L2MessagePointerRejected d wire retained consumed offset entered reason) :
      L2MessagePointerItemsRejected d (n + 1) wire [] retained consumed offset entered reason
  | next {n item tail value done retained consumed offset entered reason}
      (valid : L2MessagePointerValid d item tail value)
      (failure : L2MessagePointerItemsRejected d n tail done retained consumed offset entered reason) :
      L2MessagePointerItemsRejected d (n + 1) (item ++ tail) (value :: done) retained
        (item.length + consumed) (item.length + offset) entered reason

/-- Every fixed-length element sequence succeeds or fails at a candidate
    element; uniqueness itself is proved separately. -/
theorem l2MessagePointerItems_classify (d n : Nat) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest values, wire = item ++ rest ∧ L2MessagePointerItemsValid d n item rest values) ∨
    (∃ done retained consumed offset entered reason,
      L2MessagePointerItemsRejected d n wire done retained consumed offset entered reason) := by
  induction n generalizing wire with
  | zero => exact Or.inl ⟨[], wire, [], rfl, .nil wire⟩
  | succ n ih =>
    rcases l2MessagePointer_classify d wire depth with
      ⟨item, tail, value, split, valid⟩ | ⟨retained, consumed, offset, entered, reason, failure⟩
    · subst wire
      rcases ih tail with ⟨items, rest, values, split, restValid⟩ |
        ⟨done, retained, consumed, offset, entered, reason, failure⟩
      · subst tail
        exact Or.inl ⟨item ++ items, rest, value :: values, (List.append_assoc _ _ _).symm, .step valid restValid⟩
      · exact Or.inr ⟨value :: done, retained, item.length + consumed, item.length + offset,
          entered, reason, .next valid failure⟩
    · exact Or.inr ⟨[], retained, consumed, offset, entered, reason, .here failure⟩

theorem l2MessagePointerItems_count (d n : Nat) (item rest : List w8) (values : List (Option L2MessageValue))
    (valid : L2MessagePointerItemsValid d n item rest values) : values.length = n ∧ n ≤ item.length := by
  induction valid with
  | nil => simp
  | step valid rest ih =>
    have nonempty := l2MessagePointer_nonempty _ _ _ _ valid
    simp only [List.length_cons, List.length_append]
    omega

theorem l2MessagePointerItemsRejected_bounds (d n consumed offset : Nat) (wire : List w8)
    (done : List (Option L2MessageValue)) (retained : L2MessageValue) (entered : Bool) (reason : L2MessageFailureReason)
    (failure : L2MessagePointerItemsRejected d n wire done retained consumed offset entered reason) :
    done.length < n ∧ consumed ≤ offset ∧ offset ≤ wire.length := by
  induction failure with
  | here failure =>
    have bounds := l2MessagePointerRejected_bounds _ _ _ _ _ _ _ failure
    simp only [List.length_nil]
    omega
  | next valid failure ih => simp only [List.length_cons, List.length_append]; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessagePointer_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessagePointer_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessagePointerRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessagePointerItems_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessagePointerItems_count
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessagePointerItemsRejected_bounds
