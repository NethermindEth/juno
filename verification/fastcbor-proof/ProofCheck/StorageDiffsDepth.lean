import ProofCheck.StorageDiffsSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- An inner-map entry failure leaves its own map entered; opening failures
    leave only the outer storage map entered. -/
theorem storageDiffsEntriesRejected_depth_shape (d n consumed offset : Nat) (wire : List w8)
    (done : List (FeltMapKey × StorageDiffsValue)) (retained : Option FeltMapKey)
    (entered : Nat) (reason : StorageDiffsEntryFailureReason)
    (failure : StorageDiffsEntriesRejected d n wire done retained consumed offset entered reason) :
    entered = 0 ∨ entered = 1 := by
  induction failure with
  | here failure => exact storageDiffsEntryRejected_depth_shape _ _ _ _ _ _ _ failure
  | next valid failure ih => exact ih

theorem storageDiffs_failure_depth_shape (d consumed offset extraDepth : Nat) (wire : List w8)
    (reason : StorageDiffsFailureReason) (spec : StorageDiffsSpec d wire (.failure consumed offset extraDepth reason)) :
    extraDepth = 0 ∨ extraDepth = 1 ∨ extraDepth = 2 := by
  cases spec with
  | outer => exact Or.inl rfl
  | count => exact Or.inl rfl
  | depth => exact Or.inl rfl
  | element enc major size fit depth failure =>
    have shape := storageDiffsEntriesRejected_depth_shape _ _ _ _ _ _ _ _ _ failure
    omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffsEntriesRejected_depth_shape
#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffs_failure_depth_shape
