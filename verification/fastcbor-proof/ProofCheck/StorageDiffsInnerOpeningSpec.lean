import ProofCheck.StorageDiffsInnerFailureSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 3200000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Exact specified propagation of any first-inner-map opening error,
    including count-before-depth precedence and preservation of the inner input. -/
theorem storageDiffs_first_inner_open_rejected_spec (d count : Nat) (head : w8)
    (payload keyItem wire : List w8) (key : FeltMapKey) (reason : FeltMapFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (positive : 0 < count) (countBound : count < 2^63)
    (fit : 2 * count ≤ (keyItem ++ wire).length) (depth : d < 32)
    (valid : FeltMapKeyValid (d + 1) keyItem key)
    (failure : FeltMapSpec (d + 1) wire (.failure 0 0 0 reason)) :
    StorageDiffsSpec d ((head :: payload) ++ keyItem ++ wire)
      (.failure (1 + payload.length + keyItem.length) (1 + payload.length + keyItem.length)
        1 (.element (.value reason))) := by
  simpa only [Nat.add_zero] using
    storageDiffs_first_inner_failure_spec d count 0 0 0 head payload keyItem wire key reason
      enc major positive countBound fit depth valid failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffs_first_inner_open_rejected_spec
