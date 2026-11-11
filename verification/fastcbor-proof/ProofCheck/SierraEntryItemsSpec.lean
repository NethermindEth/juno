import ProofCheck.SierraEntryPointRecordUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Value elements decode directly into a fresh zero record. Null/undefined
    elements therefore produce a zero record, without an extra pointer wrapper. -/
abbrev SierraEntryItemValid (d : Nat) := SierraEntryPointRecordValid d sierraEntryPointZero
abbrev SierraEntryItemRejected (d : Nat) := SierraEntryPointRecordRejected d sierraEntryPointZero

theorem sierraEntryItem_classify (d : Nat) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest value, wire = item ++ rest ∧ SierraEntryItemValid d item rest value) ∨
    (∃ retained consumed offset entered reason, SierraEntryItemRejected d wire retained consumed offset entered reason) :=
  sierraEntryPointRecord_classify d sierraEntryPointZero wire depth

theorem sierraEntryItem_nonempty (d : Nat) (item rest : List w8) (value : SierraEntryPointValue)
    (valid : SierraEntryItemValid d item rest value) : 0 < item.length :=
  sierraEntryPointRecord_nonempty d sierraEntryPointZero value item rest valid

theorem sierraEntryItemRejected_bounds (d consumed offset : Nat) (wire : List w8) (retained : SierraEntryPointValue)
    (entered : Bool) (reason : SierraEntryPointFailureReason)
    (failure : SierraEntryItemRejected d wire retained consumed offset entered reason) :
    consumed ≤ offset ∧ offset ≤ wire.length :=
  sierraEntryPointRecordRejected_bounds d consumed offset sierraEntryPointZero retained wire entered reason failure

/-- Successful element sequences retain the entire later input when checking
    each record, including the suffix beyond the enclosing array. -/
inductive SierraEntryItemsValid (d : Nat) : Nat → List w8 → List w8 → List SierraEntryPointValue → Prop where
  | nil (suffix : List w8) : SierraEntryItemsValid d 0 [] suffix []
  | step {n item tail suffix value values}
      (valid : SierraEntryItemValid d item (tail ++ suffix) value)
      (rest : SierraEntryItemsValid d n tail suffix values) :
      SierraEntryItemsValid d (n + 1) (item ++ tail) suffix (value :: values)

inductive SierraEntryItemsRejected (d : Nat) : Nat → List w8 → List SierraEntryPointValue → SierraEntryPointValue →
    Nat → Nat → Bool → SierraEntryPointFailureReason → Prop where
  | here {n wire retained consumed offset entered reason}
      (failure : SierraEntryItemRejected d wire retained consumed offset entered reason) :
      SierraEntryItemsRejected d (n + 1) wire [] retained consumed offset entered reason
  | next {n item tail value done retained consumed offset entered reason}
      (valid : SierraEntryItemValid d item tail value)
      (failure : SierraEntryItemsRejected d n tail done retained consumed offset entered reason) :
      SierraEntryItemsRejected d (n + 1) (item ++ tail) (value :: done) retained
        (item.length + consumed) (item.length + offset) entered reason

/-- Every fixed-length element sequence succeeds or fails at a candidate
    element; uniqueness itself is proved separately. -/
theorem sierraEntryItems_classify (d n : Nat) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest values, wire = item ++ rest ∧ SierraEntryItemsValid d n item rest values) ∨
    (∃ done retained consumed offset entered reason,
      SierraEntryItemsRejected d n wire done retained consumed offset entered reason) := by
  induction n generalizing wire with
  | zero => exact Or.inl ⟨[], wire, [], rfl, .nil wire⟩
  | succ n ih =>
    rcases sierraEntryItem_classify d wire depth with
      ⟨item, tail, value, split, valid⟩ | ⟨retained, consumed, offset, entered, reason, failure⟩
    · subst wire
      rcases ih tail with ⟨items, rest, values, split, restValid⟩ |
        ⟨done, retained, consumed, offset, entered, reason, failure⟩
      · subst tail
        exact Or.inl ⟨item ++ items, rest, value :: values, (List.append_assoc _ _ _).symm, .step valid restValid⟩
      · exact Or.inr ⟨value :: done, retained, item.length + consumed, item.length + offset,
          entered, reason, .next valid failure⟩
    · exact Or.inr ⟨[], retained, consumed, offset, entered, reason, .here failure⟩

theorem sierraEntryItems_count (d n : Nat) (item rest : List w8) (values : List SierraEntryPointValue)
    (valid : SierraEntryItemsValid d n item rest values) : values.length = n ∧ n ≤ item.length := by
  induction valid with
  | nil => simp
  | step valid rest ih =>
    have nonempty := sierraEntryItem_nonempty _ _ _ _ valid
    simp only [List.length_cons, List.length_append]
    omega

theorem sierraEntryItemsRejected_bounds (d n consumed offset : Nat) (wire : List w8)
    (done : List SierraEntryPointValue) (retained : SierraEntryPointValue) (entered : Bool) (reason : SierraEntryPointFailureReason)
    (failure : SierraEntryItemsRejected d n wire done retained consumed offset entered reason) :
    done.length < n ∧ consumed ≤ offset ∧ offset ≤ wire.length := by
  induction failure with
  | here failure =>
    have bounds := sierraEntryItemRejected_bounds _ _ _ _ _ _ _ failure
    simp only [List.length_nil]
    omega
  | next valid failure ih => simp only [List.length_cons, List.length_append]; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryItem_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryItem_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryItemRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryItems_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryItems_count
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryItemsRejected_bounds
