import ProofCheck.FeltMapSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Failed felt reads restore their own depth, even when ignored extras fail.
    Only the enclosing map remains entered after an entry failure. -/
theorem feltMapEntriesRejected_no_depth (d n consumed offset : Nat) (wire : List w8)
    (done : List (FeltMapKey × FeltMapValue)) (retained : Option FeltMapKey)
    (entered : Bool) (reason : FeltMapEntryFailureReason)
    (failure : FeltMapEntriesRejected d n wire done retained consumed offset entered reason) : entered = false := by
  induction failure with
  | here failure => exact feltMapEntryRejected_no_depth _ _ _ _ _ _ _ failure
  | next valid failure ih => exact ih

theorem feltMap_failure_depth_shape (d consumed offset extraDepth : Nat) (wire : List w8)
    (reason : FeltMapFailureReason) (spec : FeltMapSpec d wire (.failure consumed offset extraDepth reason)) :
    extraDepth = 0 ∨ extraDepth = 1 := by
  cases spec with
  | outer => exact Or.inl rfl
  | count => exact Or.inl rfl
  | depth => exact Or.inl rfl
  | element enc major size fit depth failure =>
    have noDepth := feltMapEntriesRejected_no_depth _ _ _ _ _ _ _ _ _ failure
    simp only [noDepth, Bool.false_eq_true, ite_false, Nat.add_zero]
    exact Or.inr trivial

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMapEntriesRejected_no_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMap_failure_depth_shape
