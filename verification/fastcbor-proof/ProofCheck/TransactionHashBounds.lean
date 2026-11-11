import ProofCheck.TransactionHashRecord

set_option linter.unusedSectionVars false

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- A selected-field diagnostic lies inside its input or exactly at EOF. -/
theorem transactionHashFieldRejected_bound (d offset : Nat) (field : TransactionHashField)
    (wire : List w8) (reason : TransactionHashFieldFailure)
    (failure : TransactionHashFieldRejected d field wire offset reason) : offset ≤ wire.length := by
  cases failure with
  | hash failure => exact feltFailure_offset _ _ _ _ failure
theorem transactionHashItemRejected_bound (d offset : Nat) 
    (seen : List Nat) (choice : Option Nat) (wire : List w8) (reason : TransactionHashFailure)
    (failure : TransactionHashItemRejected d seen choice wire offset reason) : offset ≤ wire.length := by
  cases failure with
  | ignored ignored failure => exact skipFailure_offset _ _ _ _ failure
  | selected fresh lookup failure => exact transactionHashFieldRejected_bound _ _ _ _ _ failure

/-- Retained parser positions and diagnostic offsets are independently bounded;
    rejection is impossible once every declared pair has been consumed. -/
theorem transactionHashPairsRejected_bounds (d n consumed offset : Nat) 
    (seen : List Nat) (old final : TransactionHashValue) (wire : List w8) (reason : TransactionHashFailure)
    (failure : TransactionHashPairsRejected d n seen old wire final consumed offset reason) :
    consumed ≤ wire.length ∧ offset ≤ wire.length ∧ 0 < n := by
  induction failure with
  | key => simp
  | value keyValid failure =>
    have bound := transactionHashItemRejected_bound _ _ _ _ _ _ failure
    simp only [List.length_append]
    omega
  | next keyValid valueValid failure ih =>
    simp only [List.length_append]
    omega

theorem transactionHashBodyRejected_bounds (d consumed offset : Nat) 
    (old final : TransactionHashValue) (wire : List w8) (entered : Bool) (reason : TransactionHashRecordFailure)
    (failure : TransactionHashBodyRejected d old wire final consumed offset entered reason) :
    consumed ≤ wire.length ∧ offset ≤ wire.length := by
  cases failure with
  | opening => simp
  | map enc major length depth fit failure =>
    have bound := transactionHashPairsRejected_bounds _ _ _ _ _ _ _ _ _ failure
    simp only [List.length_append, List.length_cons]
    omega

/-- Adding the optional tag cannot move either failure position beyond the
    original record input. This is independent of the implementation contracts. -/
theorem transactionHashRecord_failure_bounds (d consumed offset : Nat) 
    (old final : TransactionHashValue) (wire : List w8) (entered : Bool) (reason : TransactionHashRecordFailure)
    (spec : TransactionHashRecordSpec d old wire (.failure final consumed offset entered reason)) :
    consumed ≤ wire.length ∧ offset ≤ wire.length := by
  cases spec with
  | malformed => simp
  | unsupported tag =>
    obtain ⟨item, split, length⟩ := projectionTag_suffix _ _ _ _ tag
    subst wire
    simp only [List.length_append]
    omega
  | rejected tag failure =>
    obtain ⟨item, split, length⟩ := projectionTag_suffix _ _ _ _ tag
    have bound := transactionHashBodyRejected_bounds _ _ _ _ _ _ _ _ failure
    subst wire
    simp only [List.length_append]
    omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashFieldRejected_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashItemRejected_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashPairsRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashBodyRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashRecord_failure_bounds
