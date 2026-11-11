import ProofCheck.CasmEntryPointRecordUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Value elements decode directly into a fresh zero record. Null/undefined
    elements therefore produce a zero record, without an extra pointer wrapper. -/
abbrev CasmEntryItemValid (d : Nat) := CasmEntryPointRecordValid d casmEntryPointZero
abbrev CasmEntryItemRejected (d : Nat) := CasmEntryPointRecordRejected d casmEntryPointZero

theorem casmEntryItem_classify (d : Nat) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest value, wire = item ++ rest ∧ CasmEntryItemValid d item rest value) ∨
    (∃ retained consumed offset childDepth reason, CasmEntryItemRejected d wire retained consumed offset childDepth reason) :=
  casmEntryPointRecord_classify d casmEntryPointZero wire depth

theorem casmEntryItem_nonempty (d : Nat) (item rest : List w8) (value : CasmEntryPointValue)
    (valid : CasmEntryItemValid d item rest value) : 0 < item.length :=
  casmEntryPointRecord_nonempty d casmEntryPointZero value item rest valid

theorem casmEntryItemRejected_bounds (d consumed offset : Nat) (wire : List w8) (retained : CasmEntryPointValue)
    (childDepth : Nat) (reason : CasmEntryPointFailureReason)
    (failure : CasmEntryItemRejected d wire retained consumed offset childDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length :=
  casmEntryPointRecordRejected_bounds d consumed offset casmEntryPointZero retained wire childDepth reason failure

/-- Successful element sequences retain the entire later input when checking
    each record, including the suffix beyond the enclosing array. -/
inductive CasmEntryItemsValid (d : Nat) : Nat → List w8 → List w8 → List CasmEntryPointValue → Prop where
  | nil (suffix : List w8) : CasmEntryItemsValid d 0 [] suffix []
  | step {n item tail suffix value values}
      (valid : CasmEntryItemValid d item (tail ++ suffix) value)
      (rest : CasmEntryItemsValid d n tail suffix values) :
      CasmEntryItemsValid d (n + 1) (item ++ tail) suffix (value :: values)

inductive CasmEntryItemsRejected (d : Nat) : Nat → List w8 → List CasmEntryPointValue → CasmEntryPointValue →
    Nat → Nat → Nat → CasmEntryPointFailureReason → Prop where
  | here {n wire retained consumed offset childDepth reason}
      (failure : CasmEntryItemRejected d wire retained consumed offset childDepth reason) :
      CasmEntryItemsRejected d (n + 1) wire [] retained consumed offset childDepth reason
  | next {n item tail value done retained consumed offset childDepth reason}
      (valid : CasmEntryItemValid d item tail value)
      (failure : CasmEntryItemsRejected d n tail done retained consumed offset childDepth reason) :
      CasmEntryItemsRejected d (n + 1) (item ++ tail) (value :: done) retained
        (item.length + consumed) (item.length + offset) childDepth reason

/-- Every fixed-length element sequence succeeds or fails at a candidate
    element; uniqueness itself is proved separately. -/
theorem casmEntryItems_classify (d n : Nat) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest values, wire = item ++ rest ∧ CasmEntryItemsValid d n item rest values) ∨
    (∃ done retained consumed offset childDepth reason,
      CasmEntryItemsRejected d n wire done retained consumed offset childDepth reason) := by
  induction n generalizing wire with
  | zero => exact Or.inl ⟨[], wire, [], rfl, .nil wire⟩
  | succ n ih =>
    rcases casmEntryItem_classify d wire depth with
      ⟨item, tail, value, split, valid⟩ | ⟨retained, consumed, offset, childDepth, reason, failure⟩
    · subst wire
      rcases ih tail with ⟨items, rest, values, split, restValid⟩ |
        ⟨done, retained, consumed, offset, childDepth, reason, failure⟩
      · subst tail
        exact Or.inl ⟨item ++ items, rest, value :: values, (List.append_assoc _ _ _).symm, .step valid restValid⟩
      · exact Or.inr ⟨value :: done, retained, item.length + consumed, item.length + offset,
          childDepth, reason, .next valid failure⟩
    · exact Or.inr ⟨[], retained, consumed, offset, childDepth, reason, .here failure⟩

theorem casmEntryItems_count (d n : Nat) (item rest : List w8) (values : List CasmEntryPointValue)
    (valid : CasmEntryItemsValid d n item rest values) : values.length = n ∧ n ≤ item.length := by
  induction valid with
  | nil => simp
  | step valid rest ih =>
    have nonempty := casmEntryItem_nonempty _ _ _ _ valid
    simp only [List.length_cons, List.length_append]
    omega

theorem casmEntryItemsRejected_bounds (d n consumed offset : Nat) (wire : List w8)
    (done : List CasmEntryPointValue) (retained : CasmEntryPointValue) (childDepth : Nat) (reason : CasmEntryPointFailureReason)
    (failure : CasmEntryItemsRejected d n wire done retained consumed offset childDepth reason) :
    done.length < n ∧ consumed ≤ offset ∧ offset ≤ wire.length := by
  induction failure with
  | here failure =>
    have bounds := casmEntryItemRejected_bounds _ _ _ _ _ _ _ failure
    simp only [List.length_nil]
    omega
  | next valid failure ih => simp only [List.length_cons, List.length_append]; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryItem_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryItem_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryItemRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryItems_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryItems_count
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryItemsRejected_bounds
