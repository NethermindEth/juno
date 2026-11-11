import ProofCheck.StorageDiffsValues

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- The inner map's count guard observes the complete remaining input.
    A later outer duplicate replaces the whole inner map, rather than merging it. -/
inductive StorageDiffsEntryValid (d : Nat) : List w8 → List w8 → (FeltMapKey × StorageDiffsValue) → Prop where
  | pair {keyItem valueItem rest key value}
      (keyValid : FeltMapKeyValid d keyItem key)
      (valueValid : FeltMapSpec d (valueItem ++ rest) (.success value rest)) :
      StorageDiffsEntryValid d (keyItem ++ valueItem) rest (key, value)

inductive StorageDiffsEntryFailureReason where
  | key (reason : FeltFailureReason)
  | value (reason : FeltMapFailureReason)

/-- Key errors roll back. Failed inner maps retain their consumed bytes and
    entered depth; the parsed outer key and all built inner entries stay private. -/
inductive StorageDiffsEntryRejected (d : Nat) : List w8 → Option FeltMapKey →
    Nat → Nat → Nat → StorageDiffsEntryFailureReason → Prop where
  | key {wire offset reason} (failure : LimbsFailure d wire offset reason) :
      StorageDiffsEntryRejected d wire none 0 offset 0 (.key reason)
  | value {keyItem wire key consumed offset extraDepth reason} (valid : FeltMapKeyValid d keyItem key)
      (failure : FeltMapSpec d wire (.failure consumed offset extraDepth reason)) :
      StorageDiffsEntryRejected d (keyItem ++ wire) (some key) (keyItem.length + consumed)
        (keyItem.length + offset) extraDepth (.value reason)

theorem storageDiffsEntry_classify (d : Nat) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest value, wire = item ++ rest ∧ StorageDiffsEntryValid d item rest value) ∨
    (∃ retained consumed offset entered reason, StorageDiffsEntryRejected d wire retained consumed offset entered reason) := by
  rcases feltMapKey_classify d wire depth with ⟨keyItem, tail, key, split, valid⟩ | ⟨offset, reason, failure⟩
  · subst wire
    obtain ⟨outcome, spec⟩ := feltMap_classify d tail depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, nonempty⟩ := feltMap_success_suffix d tail rest value spec
      subst tail
      exact Or.inl ⟨keyItem ++ item, rest, (key, value), (List.append_assoc _ _ _).symm, .pair valid spec⟩
    | failure consumed offset extraDepth reason =>
      exact Or.inr ⟨some key, keyItem.length + consumed, keyItem.length + offset,
        extraDepth, .value reason, .value valid spec⟩
  · exact Or.inr ⟨none, 0, offset, 0, .key reason, .key failure⟩

theorem storageDiffsEntry_min_length (d : Nat) (item rest : List w8) (value : FeltMapKey × StorageDiffsValue)
    (valid : StorageDiffsEntryValid d item rest value) : 2 ≤ item.length := by
  cases valid with
  | @pair keyItem valueItem rest key value keyValid valueValid =>
    have keyNonempty := feltMapKeyValid_nonempty _ _ _ keyValid
    obtain ⟨item, split, nonempty⟩ := feltMap_success_suffix _ _ _ _ valueValid
    have eq : valueItem = item := List.append_cancel_right split
    subst item
    simp only [List.length_append]
    omega

theorem storageDiffsEntryRejected_bounds (d consumed offset : Nat) (wire : List w8) (retained : Option FeltMapKey)
    (entered : Nat) (reason : StorageDiffsEntryFailureReason)
    (failure : StorageDiffsEntryRejected d wire retained consumed offset entered reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | key failure => exact ⟨Nat.zero_le _, limbsFailure_offset _ _ _ _ failure⟩
  | value valid failure =>
    have bounds := feltMap_failure_bounds _ _ _ _ _ _ failure
    simp only [List.length_append]
    omega

theorem storageDiffsEntryRejected_depth_shape (d consumed offset : Nat) (wire : List w8) (retained : Option FeltMapKey)
    (entered : Nat) (reason : StorageDiffsEntryFailureReason)
    (failure : StorageDiffsEntryRejected d wire retained consumed offset entered reason) : entered = 0 ∨ entered = 1 := by
  cases failure with
  | key => exact Or.inl rfl
  | value valid failure => exact feltMap_failure_depth_shape _ _ _ _ _ _ failure

theorem storageDiffsEntryRejected_depth (d consumed offset : Nat) (wire : List w8) (retained : Option FeltMapKey)
    (entered : Nat) (reason : StorageDiffsEntryFailureReason) (depth : d ≤ 32)
    (failure : StorageDiffsEntryRejected d wire retained consumed offset entered reason) :
    d + entered ≤ 32 := by
  cases failure with
  | key => simpa using depth
  | value valid failure => exact feltMap_failure_depth _ _ _ _ _ _ depth failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffsEntry_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffsEntry_min_length
#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffsEntryRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffsEntryRejected_depth_shape
#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffsEntryRejected_depth
