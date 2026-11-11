import ProofCheck.MigratedClassesEntrySpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Successful entry sequences retain the entire later input when checking
    each record, including the suffix beyond the enclosing map. -/
inductive MigratedClassesEntriesValid (d : Nat) : Nat → List w8 → List w8 → List (FeltMapKey × FeltMapKey) → Prop where
  | nil (suffix : List w8) : MigratedClassesEntriesValid d 0 [] suffix []
  | step {n item tail suffix value values}
      (valid : MigratedClassesEntryValid d item (tail ++ suffix) value)
      (rest : MigratedClassesEntriesValid d n tail suffix values) :
      MigratedClassesEntriesValid d (n + 1) (item ++ tail) suffix (value :: values)

inductive MigratedClassesEntriesRejected (d : Nat) : Nat → List w8 → List (FeltMapKey × FeltMapKey) → Option FeltMapKey →
    Nat → Nat → Bool → MigratedClassesEntryFailureReason → Prop where
  | here {n wire retained consumed offset entered reason}
      (failure : MigratedClassesEntryRejected d wire retained consumed offset entered reason) :
      MigratedClassesEntriesRejected d (n + 1) wire [] retained consumed offset entered reason
  | next {n item tail value done retained consumed offset entered reason}
      (valid : MigratedClassesEntryValid d item tail value)
      (failure : MigratedClassesEntriesRejected d n tail done retained consumed offset entered reason) :
      MigratedClassesEntriesRejected d (n + 1) (item ++ tail) (value :: done) retained
        (item.length + consumed) (item.length + offset) entered reason

/-- Every fixed-length element sequence succeeds or fails at a candidate
    entry; uniqueness itself is proved separately. -/
theorem migratedClassesEntries_classify (d n : Nat) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest values, wire = item ++ rest ∧ MigratedClassesEntriesValid d n item rest values) ∨
    (∃ done retained consumed offset entered reason,
      MigratedClassesEntriesRejected d n wire done retained consumed offset entered reason) := by
  induction n generalizing wire with
  | zero => exact Or.inl ⟨[], wire, [], rfl, .nil wire⟩
  | succ n ih =>
    rcases migratedClassesEntry_classify d wire depth with
      ⟨item, tail, value, split, valid⟩ | ⟨retained, consumed, offset, entered, reason, failure⟩
    · subst wire
      rcases ih tail with ⟨items, rest, values, split, restValid⟩ |
        ⟨done, retained, consumed, offset, entered, reason, failure⟩
      · subst tail
        exact Or.inl ⟨item ++ items, rest, value :: values, (List.append_assoc _ _ _).symm, .step valid restValid⟩
      · exact Or.inr ⟨value :: done, retained, item.length + consumed, item.length + offset,
          entered, reason, .next valid failure⟩
    · exact Or.inr ⟨[], retained, consumed, offset, entered, reason, .here failure⟩

theorem migratedClassesEntries_count (d n : Nat) (item rest : List w8) (values : List (FeltMapKey × FeltMapKey))
    (valid : MigratedClassesEntriesValid d n item rest values) : values.length = n ∧ 2 * n ≤ item.length := by
  induction valid with
  | nil => simp
  | step valid rest ih =>
    have nonempty := migratedClassesEntry_min_length _ _ _ _ valid
    simp only [List.length_cons, List.length_append]
    omega

theorem migratedClassesEntriesRejected_bounds (d n consumed offset : Nat) (wire : List w8)
    (done : List (FeltMapKey × FeltMapKey)) (retained : Option FeltMapKey) (entered : Bool) (reason : MigratedClassesEntryFailureReason)
    (failure : MigratedClassesEntriesRejected d n wire done retained consumed offset entered reason) :
    done.length < n ∧ consumed ≤ offset ∧ offset ≤ wire.length := by
  induction failure with
  | here failure =>
    have bounds := migratedClassesEntryRejected_bounds _ _ _ _ _ _ _ failure
    simp only [List.length_nil]
    omega
  | next valid failure ih => simp only [List.length_cons, List.length_append]; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.migratedClassesEntries_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.migratedClassesEntries_count
#print axioms Perennial.github_com.NethermindEth.juno.core.migratedClassesEntriesRejected_bounds
