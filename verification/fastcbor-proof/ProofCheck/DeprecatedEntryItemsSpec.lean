import ProofCheck.DeprecatedEntryPointRecordUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Value elements decode directly into a fresh zero record. Null/undefined
    elements therefore produce a zero record, without an extra pointer wrapper. -/
abbrev DeprecatedEntryItemValid (d : Nat) := DeprecatedEntryPointRecordValid d deprecatedEntryPointZero
abbrev DeprecatedEntryItemRejected (d : Nat) := DeprecatedEntryPointRecordRejected d deprecatedEntryPointZero

theorem deprecatedEntryItem_classify (d : Nat) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest value, wire = item ++ rest ∧ DeprecatedEntryItemValid d item rest value) ∨
    (∃ retained consumed offset entered reason, DeprecatedEntryItemRejected d wire retained consumed offset entered reason) :=
  deprecatedEntryPointRecord_classify d deprecatedEntryPointZero wire depth

theorem deprecatedEntryItem_nonempty (d : Nat) (item rest : List w8) (value : DeprecatedEntryPointValue)
    (valid : DeprecatedEntryItemValid d item rest value) : 0 < item.length :=
  deprecatedEntryPointRecord_nonempty d deprecatedEntryPointZero value item rest valid

theorem deprecatedEntryItemRejected_bounds (d consumed offset : Nat) (wire : List w8) (retained : DeprecatedEntryPointValue)
    (entered : Bool) (reason : DeprecatedEntryPointFailureReason)
    (failure : DeprecatedEntryItemRejected d wire retained consumed offset entered reason) :
    consumed ≤ offset ∧ offset ≤ wire.length :=
  deprecatedEntryPointRecordRejected_bounds d consumed offset deprecatedEntryPointZero retained wire entered reason failure

/-- Successful element sequences retain the entire later input when checking
    each record, including the suffix beyond the enclosing array. -/
inductive DeprecatedEntryItemsValid (d : Nat) : Nat → List w8 → List w8 → List DeprecatedEntryPointValue → Prop where
  | nil (suffix : List w8) : DeprecatedEntryItemsValid d 0 [] suffix []
  | step {n item tail suffix value values}
      (valid : DeprecatedEntryItemValid d item (tail ++ suffix) value)
      (rest : DeprecatedEntryItemsValid d n tail suffix values) :
      DeprecatedEntryItemsValid d (n + 1) (item ++ tail) suffix (value :: values)

inductive DeprecatedEntryItemsRejected (d : Nat) : Nat → List w8 → List DeprecatedEntryPointValue → DeprecatedEntryPointValue →
    Nat → Nat → Bool → DeprecatedEntryPointFailureReason → Prop where
  | here {n wire retained consumed offset entered reason}
      (failure : DeprecatedEntryItemRejected d wire retained consumed offset entered reason) :
      DeprecatedEntryItemsRejected d (n + 1) wire [] retained consumed offset entered reason
  | next {n item tail value done retained consumed offset entered reason}
      (valid : DeprecatedEntryItemValid d item tail value)
      (failure : DeprecatedEntryItemsRejected d n tail done retained consumed offset entered reason) :
      DeprecatedEntryItemsRejected d (n + 1) (item ++ tail) (value :: done) retained
        (item.length + consumed) (item.length + offset) entered reason

/-- Every fixed-length element sequence succeeds or fails at a candidate
    element; uniqueness itself is proved separately. -/
theorem deprecatedEntryItems_classify (d n : Nat) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest values, wire = item ++ rest ∧ DeprecatedEntryItemsValid d n item rest values) ∨
    (∃ done retained consumed offset entered reason,
      DeprecatedEntryItemsRejected d n wire done retained consumed offset entered reason) := by
  induction n generalizing wire with
  | zero => exact Or.inl ⟨[], wire, [], rfl, .nil wire⟩
  | succ n ih =>
    rcases deprecatedEntryItem_classify d wire depth with
      ⟨item, tail, value, split, valid⟩ | ⟨retained, consumed, offset, entered, reason, failure⟩
    · subst wire
      rcases ih tail with ⟨items, rest, values, split, restValid⟩ |
        ⟨done, retained, consumed, offset, entered, reason, failure⟩
      · subst tail
        exact Or.inl ⟨item ++ items, rest, value :: values, (List.append_assoc _ _ _).symm, .step valid restValid⟩
      · exact Or.inr ⟨value :: done, retained, item.length + consumed, item.length + offset,
          entered, reason, .next valid failure⟩
    · exact Or.inr ⟨[], retained, consumed, offset, entered, reason, .here failure⟩

theorem deprecatedEntryItems_count (d n : Nat) (item rest : List w8) (values : List DeprecatedEntryPointValue)
    (valid : DeprecatedEntryItemsValid d n item rest values) : values.length = n ∧ n ≤ item.length := by
  induction valid with
  | nil => simp
  | step valid rest ih =>
    have nonempty := deprecatedEntryItem_nonempty _ _ _ _ valid
    simp only [List.length_cons, List.length_append]
    omega

theorem deprecatedEntryItemsRejected_bounds (d n consumed offset : Nat) (wire : List w8)
    (done : List DeprecatedEntryPointValue) (retained : DeprecatedEntryPointValue) (entered : Bool) (reason : DeprecatedEntryPointFailureReason)
    (failure : DeprecatedEntryItemsRejected d n wire done retained consumed offset entered reason) :
    done.length < n ∧ consumed ≤ offset ∧ offset ≤ wire.length := by
  induction failure with
  | here failure =>
    have bounds := deprecatedEntryItemRejected_bounds _ _ _ _ _ _ _ failure
    simp only [List.length_nil]
    omega
  | next valid failure ih => simp only [List.length_cons, List.length_append]; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryItem_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryItem_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryItemRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryItems_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryItems_count
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryItemsRejected_bounds
