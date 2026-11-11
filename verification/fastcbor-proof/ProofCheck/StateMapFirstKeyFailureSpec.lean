import ProofCheck.FeltMapSpec
import ProofCheck.StorageDiffsSpec
import ProofCheck.MigratedClassesSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 3200000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- First-key failure is an instance of the independent FeltMap relation,
    including null/undefined key rejection and retained outer depth. -/
theorem feltMap_first_key_failure_spec (d count offset : Nat) (head : w8)
    (payload wire : List w8) (reason : FeltFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (positive : 0 < count) (countBound : count < 2^63)
    (fit : 2 * count ≤ wire.length) (depth : d < 32)
    (failure : LimbsFailure (d + 1) wire offset reason) :
    FeltMapSpec d ((head :: payload) ++ wire)
      (.failure (1 + payload.length) (1 + payload.length + offset) 1 (.element (.key reason))) := by
  have size : uint.Z (W64 count) = (count : Int) := by word
  obtain ⟨n, rfl⟩ : ∃ n, count = n + 1 := ⟨count - 1, by omega⟩
  simpa only [Bool.false_eq_true, ite_false, Nat.add_zero] using
    (FeltMapSpec.element (count := n + 1) enc major size fit depth
      (FeltMapEntriesRejected.here (n := n) (FeltMapEntryRejected.key failure)))

/-- First-key failure is an instance of the independent StorageDiffs relation,
    including null/undefined key rejection and retained outer depth. -/
theorem storageDiffs_first_key_failure_spec (d count offset : Nat) (head : w8)
    (payload wire : List w8) (reason : FeltFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (positive : 0 < count) (countBound : count < 2^63)
    (fit : 2 * count ≤ wire.length) (depth : d < 32)
    (failure : LimbsFailure (d + 1) wire offset reason) :
    StorageDiffsSpec d ((head :: payload) ++ wire)
      (.failure (1 + payload.length) (1 + payload.length + offset) 1 (.element (.key reason))) := by
  have size : uint.Z (W64 count) = (count : Int) := by word
  obtain ⟨n, rfl⟩ : ∃ n, count = n + 1 := ⟨count - 1, by omega⟩
  simpa only [Bool.false_eq_true, ite_false, Nat.add_zero] using
    (StorageDiffsSpec.element (count := n + 1) enc major size fit depth
      (StorageDiffsEntriesRejected.here (n := n) (StorageDiffsEntryRejected.key failure)))

/-- First-key failure is an instance of the independent MigratedClasses relation,
    including null/undefined key rejection and retained outer depth. -/
theorem migratedClasses_first_key_failure_spec (d count offset : Nat) (head : w8)
    (payload wire : List w8) (reason : FeltFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (positive : 0 < count) (countBound : count < 2^63)
    (fit : 2 * count ≤ wire.length) (depth : d < 32)
    (failure : LimbsFailure (d + 1) wire offset reason) :
    MigratedClassesSpec d ((head :: payload) ++ wire)
      (.failure (1 + payload.length) (1 + payload.length + offset) 1 (.element (.key reason))) := by
  have size : uint.Z (W64 count) = (count : Int) := by word
  obtain ⟨n, rfl⟩ : ∃ n, count = n + 1 := ⟨count - 1, by omega⟩
  simpa only [Bool.false_eq_true, ite_false, Nat.add_zero] using
    (MigratedClassesSpec.element (count := n + 1) enc major size fit depth
      (MigratedClassesEntriesRejected.here (n := n) (MigratedClassesEntryRejected.key failure)))

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMap_first_key_failure_spec
#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffs_first_key_failure_spec
#print axioms Perennial.github_com.NethermindEth.juno.core.migratedClasses_first_key_failure_spec
