import ProofCheck.FeltMapSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 3200000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- A first-value error is an instance of the complete independent map relation:
    the map head and key remain consumed, one outer depth level stays entered,
    and the failing value is not published. -/
theorem feltMap_first_value_failure_spec (d count offset : Nat) (head : w8)
    (payload keyItem wire : List w8) (key : FeltMapKey) (reason : FeltFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (positive : 0 < count) (countBound : count < 2^63)
    (fit : 2 * count ≤ (keyItem ++ wire).length) (depth : d < 32)
    (valid : FeltMapKeyValid (d + 1) keyItem key)
    (failure : FeltFailure (d + 1) wire offset reason) :
    FeltMapSpec d ((head :: payload) ++ keyItem ++ wire)
      (.failure (1 + payload.length + keyItem.length) (1 + payload.length + keyItem.length + offset)
        1 (.element (.value reason))) := by
  have size : uint.Z (W64 count) = (count : Int) := by word
  obtain ⟨n, rfl⟩ : ∃ n, count = n + 1 := ⟨count - 1, by omega⟩
  simpa only [List.append_assoc, Bool.false_eq_true, ite_false, Nat.add_zero, Nat.add_assoc] using
    (FeltMapSpec.element (count := n + 1) enc major size fit depth
      (FeltMapEntriesRejected.here (n := n) (FeltMapEntryRejected.value valid failure)))

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMap_first_value_failure_spec
