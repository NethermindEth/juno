import ProofCheck.ResourceMapKeySpec
import ProofCheck.ResourceBoundsRecordUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- One entry always decodes its value from zero, even when its key appeared
    earlier. The suffix includes later entries and bytes after the outer map. -/
inductive ResourceMapEntryValid (d : Nat) : List w8 → List w8 → (w32 × ResourceBoundsValue) → Prop where
  | pair {keyItem valueItem rest key value}
      (keyValid : ResourceMapKeyValid keyItem key)
      (valueValid : ResourceBoundsRecordValid d resourceBoundsZero valueItem rest value) :
      ResourceMapEntryValid d (keyItem ++ valueItem) rest (key, value)

inductive ResourceMapEntryFailureReason where
  | key (reason : ResourceMapKeyFailureReason)
  | value (reason : ResourceBoundsFailureReason)

/-- A failed value is private. Key failures have no partial value and retain no
    child-map depth. Bare key sentinels use their consumed boundary as offset. -/
inductive ResourceMapEntryRejected (d : Nat) : List w8 → ResourceBoundsValue →
    Nat → Nat → Bool → ResourceMapEntryFailureReason → Prop where
  | key {wire consumed reason} (failure : ResourceMapKeyRejected wire consumed reason) :
      ResourceMapEntryRejected d wire resourceBoundsZero consumed consumed false (.key reason)
  | value {keyItem wire key retained consumed offset entered reason}
      (valid : ResourceMapKeyValid keyItem key)
      (failure : ResourceBoundsRecordRejected d resourceBoundsZero wire retained consumed offset entered reason) :
      ResourceMapEntryRejected d (keyItem ++ wire) retained (keyItem.length + consumed)
        (keyItem.length + offset) entered (.value reason)

theorem resourceMapEntry_classify (d : Nat) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest value, wire = item ++ rest ∧ ResourceMapEntryValid d item rest value) ∨
    (∃ retained consumed offset entered reason, ResourceMapEntryRejected d wire retained consumed offset entered reason) := by
  rcases resourceMapKey_classify wire with ⟨keyItem, tail, key, split, valid⟩ | ⟨consumed, reason, failure⟩
  · subst wire
    rcases resourceBoundsRecord_classify d resourceBoundsZero tail depth with
      ⟨item, rest, value, split, valueValid⟩ | ⟨retained, consumed, offset, entered, reason, failure⟩
    · subst tail
      exact Or.inl ⟨keyItem ++ item, rest, (key, value), (List.append_assoc _ _ _).symm, .pair valid valueValid⟩
    · exact Or.inr ⟨retained, keyItem.length + consumed, keyItem.length + offset, entered, .value reason,
        .value valid failure⟩
  · exact Or.inr ⟨resourceBoundsZero, consumed, consumed, false, .key reason, .key failure⟩

theorem resourceMapEntry_min_length (d : Nat) (item rest : List w8) (value : w32 × ResourceBoundsValue)
    (valid : ResourceMapEntryValid d item rest value) : 2 ≤ item.length := by
  cases valid with
  | pair keyValid valueValid =>
    have keyNonempty := resourceMapKeyValid_nonempty _ _ keyValid
    have valueNonempty := resourceBoundsRecord_nonempty _ _ _ _ _ valueValid
    simp only [List.length_append]
    omega

theorem resourceMapEntryRejected_bounds (d consumed offset : Nat) (wire : List w8) (retained : ResourceBoundsValue)
    (entered : Bool) (reason : ResourceMapEntryFailureReason)
    (failure : ResourceMapEntryRejected d wire retained consumed offset entered reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | key failure => exact ⟨Nat.le_refl _, resourceMapKeyRejected_bound _ _ _ failure⟩
  | value valid failure =>
    have bounds := resourceBoundsRecordRejected_bounds _ _ _ _ _ _ _ _ failure
    simp only [List.length_append]
    omega

theorem resourceMapEntryRejected_depth (d consumed offset : Nat) (wire : List w8) (retained : ResourceBoundsValue)
    (entered : Bool) (reason : ResourceMapEntryFailureReason) (depth : d ≤ 32)
    (failure : ResourceMapEntryRejected d wire retained consumed offset entered reason) :
    d + (if entered then 1 else 0) ≤ 32 := by
  cases failure with
  | key => simpa using depth
  | value valid failure =>
    cases failure with
    | opening => simpa using depth
    | map enc major size outer fit failure => simp only [ite_true]; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMapEntry_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMapEntry_min_length
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMapEntryRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMapEntryRejected_depth
