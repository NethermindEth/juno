import ProofCheck.FeltMapKeySpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Keys require arrays; nullable pointer values use the independent felt
    relation. Later duplicate entries replace the entire earlier pointer. -/
inductive FeltMapEntryValid (d : Nat) : List w8 → List w8 → (FeltMapKey × FeltMapValue) → Prop where
  | pair {keyItem valueItem rest key value}
      (keyValid : FeltMapKeyValid d keyItem key) (valueValid : FeltValid d valueItem value) :
      FeltMapEntryValid d (keyItem ++ valueItem) rest (key, value)

inductive FeltMapEntryFailureReason where
  | key (reason : FeltFailureReason)
  | value (reason : FeltFailureReason)

/-- Key errors roll back to entry start. Value errors retain the parsed key and
    consumed key bytes; their own felt read rolls back. Neither retains extra
    cursor depth. The optional retained value here is the private parsed key. -/
inductive FeltMapEntryRejected (d : Nat) : List w8 → Option FeltMapKey →
    Nat → Nat → Bool → FeltMapEntryFailureReason → Prop where
  | key {wire offset reason} (failure : LimbsFailure d wire offset reason) :
      FeltMapEntryRejected d wire none 0 offset false (.key reason)
  | value {keyItem wire key offset reason} (valid : FeltMapKeyValid d keyItem key)
      (failure : FeltFailure d wire offset reason) :
      FeltMapEntryRejected d (keyItem ++ wire) (some key) keyItem.length
        (keyItem.length + offset) false (.value reason)

theorem feltMapEntry_classify (d : Nat) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest value, wire = item ++ rest ∧ FeltMapEntryValid d item rest value) ∨
    (∃ retained consumed offset entered reason, FeltMapEntryRejected d wire retained consumed offset entered reason) := by
  rcases feltMapKey_classify d wire depth with ⟨keyItem, tail, key, split, valid⟩ | ⟨offset, reason, failure⟩
  · subst wire
    rcases felt_classify d tail depth with ⟨item, rest, value, split, valueValid⟩ | ⟨offset, reason, failure⟩
    · subst tail
      exact Or.inl ⟨keyItem ++ item, rest, (key, value), (List.append_assoc _ _ _).symm, .pair valid valueValid⟩
    · exact Or.inr ⟨some key, keyItem.length, keyItem.length + offset, false, .value reason, .value valid failure⟩
  · exact Or.inr ⟨none, 0, offset, false, .key reason, .key failure⟩

theorem feltMapEntry_min_length (d : Nat) (item rest : List w8) (value : FeltMapKey × FeltMapValue)
    (valid : FeltMapEntryValid d item rest value) : 2 ≤ item.length := by
  cases valid with
  | @pair keyItem valueItem rest key value keyValid valueValid =>
    have keyNonempty := feltMapKeyValid_nonempty _ _ _ keyValid
    have valueNonempty : 0 < valueItem.length := by cases valueValid <;> simp
    simp only [List.length_append]
    omega

theorem feltMapEntryRejected_bounds (d consumed offset : Nat) (wire : List w8) (retained : Option FeltMapKey)
    (entered : Bool) (reason : FeltMapEntryFailureReason)
    (failure : FeltMapEntryRejected d wire retained consumed offset entered reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | key failure => exact ⟨Nat.zero_le _, limbsFailure_offset _ _ _ _ failure⟩
  | value valid failure =>
    have bounds := feltFailure_offset _ _ _ _ failure
    simp only [List.length_append]
    omega

theorem feltMapEntryRejected_no_depth (d consumed offset : Nat) (wire : List w8) (retained : Option FeltMapKey)
    (entered : Bool) (reason : FeltMapEntryFailureReason)
    (failure : FeltMapEntryRejected d wire retained consumed offset entered reason) : entered = false := by
  cases failure <;> rfl

theorem feltMapEntryRejected_depth (d consumed offset : Nat) (wire : List w8) (retained : Option FeltMapKey)
    (entered : Bool) (reason : FeltMapEntryFailureReason) (depth : d ≤ 32)
    (failure : FeltMapEntryRejected d wire retained consumed offset entered reason) :
    d + (if entered then 1 else 0) ≤ 32 := by
  cases failure <;> simpa using depth

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMapEntry_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMapEntry_min_length
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMapEntryRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMapEntryRejected_no_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMapEntryRejected_depth
