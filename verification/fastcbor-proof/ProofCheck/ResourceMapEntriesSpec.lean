import ProofCheck.ResourceMapEntrySpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Successful entry sequences retain the entire later input when checking
    each record, including the suffix beyond the enclosing array. -/
inductive ResourceMapEntriesValid (d : Nat) : Nat → List w8 → List w8 → List (w32 × ResourceBoundsValue) → Prop where
  | nil (suffix : List w8) : ResourceMapEntriesValid d 0 [] suffix []
  | step {n item tail suffix value values}
      (valid : ResourceMapEntryValid d item (tail ++ suffix) value)
      (rest : ResourceMapEntriesValid d n tail suffix values) :
      ResourceMapEntriesValid d (n + 1) (item ++ tail) suffix (value :: values)

inductive ResourceMapEntriesRejected (d : Nat) : Nat → List w8 → List (w32 × ResourceBoundsValue) → ResourceBoundsValue →
    Nat → Nat → Bool → ResourceMapEntryFailureReason → Prop where
  | here {n wire retained consumed offset entered reason}
      (failure : ResourceMapEntryRejected d wire retained consumed offset entered reason) :
      ResourceMapEntriesRejected d (n + 1) wire [] retained consumed offset entered reason
  | next {n item tail value done retained consumed offset entered reason}
      (valid : ResourceMapEntryValid d item tail value)
      (failure : ResourceMapEntriesRejected d n tail done retained consumed offset entered reason) :
      ResourceMapEntriesRejected d (n + 1) (item ++ tail) (value :: done) retained
        (item.length + consumed) (item.length + offset) entered reason

/-- Every fixed-length element sequence succeeds or fails at a candidate
    entry; uniqueness itself is proved separately. -/
theorem resourceMapEntries_classify (d n : Nat) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest values, wire = item ++ rest ∧ ResourceMapEntriesValid d n item rest values) ∨
    (∃ done retained consumed offset entered reason,
      ResourceMapEntriesRejected d n wire done retained consumed offset entered reason) := by
  induction n generalizing wire with
  | zero => exact Or.inl ⟨[], wire, [], rfl, .nil wire⟩
  | succ n ih =>
    rcases resourceMapEntry_classify d wire depth with
      ⟨item, tail, value, split, valid⟩ | ⟨retained, consumed, offset, entered, reason, failure⟩
    · subst wire
      rcases ih tail with ⟨items, rest, values, split, restValid⟩ |
        ⟨done, retained, consumed, offset, entered, reason, failure⟩
      · subst tail
        exact Or.inl ⟨item ++ items, rest, value :: values, (List.append_assoc _ _ _).symm, .step valid restValid⟩
      · exact Or.inr ⟨value :: done, retained, item.length + consumed, item.length + offset,
          entered, reason, .next valid failure⟩
    · exact Or.inr ⟨[], retained, consumed, offset, entered, reason, .here failure⟩

theorem resourceMapEntries_count (d n : Nat) (item rest : List w8) (values : List (w32 × ResourceBoundsValue))
    (valid : ResourceMapEntriesValid d n item rest values) : values.length = n ∧ 2 * n ≤ item.length := by
  induction valid with
  | nil => simp
  | step valid rest ih =>
    have nonempty := resourceMapEntry_min_length _ _ _ _ valid
    simp only [List.length_cons, List.length_append]
    omega

theorem resourceMapEntriesRejected_bounds (d n consumed offset : Nat) (wire : List w8)
    (done : List (w32 × ResourceBoundsValue)) (retained : ResourceBoundsValue) (entered : Bool) (reason : ResourceMapEntryFailureReason)
    (failure : ResourceMapEntriesRejected d n wire done retained consumed offset entered reason) :
    done.length < n ∧ consumed ≤ offset ∧ offset ≤ wire.length := by
  induction failure with
  | here failure =>
    have bounds := resourceMapEntryRejected_bounds _ _ _ _ _ _ _ failure
    simp only [List.length_nil]
    omega
  | next valid failure ih => simp only [List.length_cons, List.length_append]; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMapEntries_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMapEntries_count
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMapEntriesRejected_bounds
