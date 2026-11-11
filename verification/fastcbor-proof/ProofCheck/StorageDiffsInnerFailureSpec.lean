import ProofCheck.StorageDiffsSpec
import ProofCheck.StateMapFirstKeyFailureSpec
import ProofCheck.FeltMapFirstFailureSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 3200000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Any independently specified failure in the first inner map propagates
    through the outer map with the parsed outer key retained privately. -/
theorem storageDiffs_first_inner_failure_spec (d count consumed offset entered : Nat)
    (head : w8) (payload keyItem wire : List w8) (key : FeltMapKey) (reason : FeltMapFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (positive : 0 < count) (countBound : count < 2^63)
    (fit : 2 * count ≤ (keyItem ++ wire).length) (depth : d < 32)
    (valid : FeltMapKeyValid (d + 1) keyItem key)
    (failure : FeltMapSpec (d + 1) wire (.failure consumed offset entered reason)) :
    StorageDiffsSpec d ((head :: payload) ++ keyItem ++ wire)
      (.failure (1 + payload.length + keyItem.length + consumed)
        (1 + payload.length + keyItem.length + offset) (1 + entered) (.element (.value reason))) := by
  have size : uint.Z (W64 count) = (count : Int) := by word
  obtain ⟨n, rfl⟩ : ∃ n, count = n + 1 := ⟨count - 1, by omega⟩
  simpa only [List.append_assoc, Nat.add_assoc] using
    (StorageDiffsSpec.element (count := n + 1) enc major size fit depth
      (StorageDiffsEntriesRejected.here (n := n) (StorageDiffsEntryRejected.value valid failure)))

/-- Exact nested first-key error, with both map headers still entered. -/
theorem storageDiffs_first_inner_key_failure_spec (d count innerCount offset : Nat)
    (head innerHead : w8) (payload keyItem wire innerPayload tail : List w8)
    (key : FeltMapKey) (reason : FeltFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (positive : 0 < count) (countBound : count < 2^63)
    (fit : 2 * count ≤ (keyItem ++ wire).length) (depth : d + 1 < 32)
    (valid : FeltMapKeyValid (d + 1) keyItem key)
    (innerEnc : HeadArgument innerHead innerPayload (W64 innerCount))
    (innerMajor : innerHead &&& W8 224 = W8 160) (innerPositive : 0 < innerCount)
    (innerBound : innerCount < 2^63) (innerFit : 2 * innerCount ≤ tail.length)
    (split : wire = (innerHead :: innerPayload) ++ tail)
    (failure : LimbsFailure (d + 1 + 1) tail offset reason) :
    StorageDiffsSpec d ((head :: payload) ++ keyItem ++ wire)
      (.failure (1 + payload.length + keyItem.length + 1 + innerPayload.length)
        (1 + payload.length + keyItem.length + 1 + innerPayload.length + offset) 2
        (.element (.value (.element (.key reason))))) := by
  have child := feltMap_first_key_failure_spec (d + 1) innerCount offset innerHead innerPayload tail reason
    innerEnc innerMajor innerPositive innerBound innerFit depth failure
  rw [← split] at child
  simpa only [Nat.add_assoc] using
    storageDiffs_first_inner_failure_spec d count _ _ _ head payload keyItem wire key _
      enc major positive countBound fit (by omega) valid child

/-- Exact nested first-value error, after both keys and both map headers. -/
theorem storageDiffs_first_inner_value_failure_spec (d count innerCount offset : Nat)
    (head innerHead : w8) (payload keyItem wire innerPayload innerKeyItem tail : List w8)
    (key innerKey : FeltMapKey) (reason : FeltFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (positive : 0 < count) (countBound : count < 2^63)
    (fit : 2 * count ≤ (keyItem ++ wire).length) (depth : d + 1 < 32)
    (valid : FeltMapKeyValid (d + 1) keyItem key)
    (innerEnc : HeadArgument innerHead innerPayload (W64 innerCount))
    (innerMajor : innerHead &&& W8 224 = W8 160) (innerPositive : 0 < innerCount)
    (innerBound : innerCount < 2^63) (innerFit : 2 * innerCount ≤ (innerKeyItem ++ tail).length)
    (split : wire = (innerHead :: innerPayload) ++ innerKeyItem ++ tail)
    (innerValid : FeltMapKeyValid (d + 1 + 1) innerKeyItem innerKey)
    (failure : FeltFailure (d + 1 + 1) tail offset reason) :
    StorageDiffsSpec d ((head :: payload) ++ keyItem ++ wire)
      (.failure (1 + payload.length + keyItem.length + 1 + innerPayload.length + innerKeyItem.length)
        (1 + payload.length + keyItem.length + 1 + innerPayload.length + innerKeyItem.length + offset) 2
        (.element (.value (.element (.value reason))))) := by
  have child := feltMap_first_value_failure_spec (d + 1) innerCount offset innerHead innerPayload innerKeyItem tail innerKey reason
    innerEnc innerMajor innerPositive innerBound innerFit depth innerValid failure
  rw [← split] at child
  simpa only [Nat.add_assoc] using
    storageDiffs_first_inner_failure_spec d count _ _ _ head payload keyItem wire key _
      enc major positive countBound fit (by omega) valid child

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffs_first_inner_failure_spec
#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffs_first_inner_key_failure_spec
#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffs_first_inner_value_failure_spec
