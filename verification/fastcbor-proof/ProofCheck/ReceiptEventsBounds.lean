import ProofCheck.ReceiptEventsRecord

set_option linter.unusedSectionVars false

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

theorem receiptEventsItemRejected_bounds (d consumed offset extraDepth : Nat)
    (seen : List Nat) (choice : Option Nat) (wire : List w8) (reason : ReceiptEventsFailure)
    (failure : ReceiptEventsItemRejected d seen choice wire consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | ignored ignored failure => exact ⟨Nat.zero_le _, skipFailure_offset _ _ _ _ failure⟩
  | selected fresh lookup failure => exact receiptEventsField_rejected_bounds _ _ _ _ _ _ _ failure

theorem receiptEventsItemRejected_depth (d consumed offset extraDepth : Nat)
    (seen : List Nat) (choice : Option Nat) (wire : List w8) (reason : ReceiptEventsFailure) (depth : d ≤ 32)
    (failure : ReceiptEventsItemRejected d seen choice wire consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  cases failure with
  | ignored ignored failure => simpa using depth
  | selected fresh lookup failure => exact receiptEventsField_rejected_depth _ _ _ _ _ _ _ depth failure

/-- Rejection requires at least one unconsumed pair. Consumption never exceeds
    the diagnostic offset, including through nested Event-array failures. -/
theorem receiptEventsPairsRejected_bounds (d n consumed offset extraDepth : Nat)
    (seen : List Nat) (old final : ReceiptEventsValue) (wire : List w8) (reason : ReceiptEventsFailure)
    (failure : ReceiptEventsPairsRejected d n seen old wire final consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length ∧ 0 < n := by
  induction failure with
  | key => simp
  | value keyValid failure =>
    have bound := receiptEventsItemRejected_bounds _ _ _ _ _ _ _ _ failure
    simp only [List.length_append]; omega
  | next keyValid valueValid failure ih => simp only [List.length_append]; omega

theorem receiptEventsPairsRejected_depth (d n consumed offset extraDepth : Nat)
    (seen : List Nat) (old final : ReceiptEventsValue) (wire : List w8) (reason : ReceiptEventsFailure)
    (depth : d ≤ 32)
    (failure : ReceiptEventsPairsRejected d n seen old wire final consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  induction failure with
  | key => simpa using depth
  | value keyValid failure => exact receiptEventsItemRejected_depth _ _ _ _ _ _ _ _ depth failure
  | next keyValid valueValid failure ih => exact ih

theorem receiptEventsBodyRejected_bounds (d consumed offset extraDepth : Nat)
    (old final : ReceiptEventsValue) (wire : List w8) (reason : ReceiptEventsRecordFailure)
    (failure : ReceiptEventsBodyRejected d old wire final consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | opening => simp
  | map enc major length depth fit failure =>
    have bound := receiptEventsPairsRejected_bounds _ _ _ _ _ _ _ _ _ _ failure
    simp only [List.length_append, List.length_cons]; omega

theorem receiptEventsBodyRejected_depth (d consumed offset extraDepth : Nat)
    (old final : ReceiptEventsValue) (wire : List w8) (reason : ReceiptEventsRecordFailure)
    (depth : d ≤ 32)
    (failure : ReceiptEventsBodyRejected d old wire final consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  cases failure with
  | opening => simpa using depth
  | map enc major length entered fit failure =>
    have bound := receiptEventsPairsRejected_depth _ _ _ _ _ _ _ _ _ _ (by omega) failure
    omega

/-- The tag prelude adds bytes, never nesting levels, to the body outcome. -/
theorem receiptEventsRecord_failure_bounds (d consumed offset extraDepth : Nat)
    (old final : ReceiptEventsValue) (wire : List w8) (reason : ReceiptEventsRecordFailure)
    (spec : ReceiptEventsRecordSpec d old wire (.failure final consumed offset extraDepth reason)) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases spec with
  | malformed => simp
  | unsupported tag =>
    obtain ⟨item, split, length⟩ := projectionTag_suffix _ _ _ _ tag
    subst wire
    simp only [List.length_append]; omega
  | rejected tag failure =>
    obtain ⟨item, split, length⟩ := projectionTag_suffix _ _ _ _ tag
    have bound := receiptEventsBodyRejected_bounds _ _ _ _ _ _ _ _ failure
    subst wire
    simp only [List.length_append]; omega

theorem receiptEventsRecord_failure_depth (d consumed offset extraDepth : Nat)
    (old final : ReceiptEventsValue) (wire : List w8) (reason : ReceiptEventsRecordFailure)
    (depth : d ≤ 32)
    (spec : ReceiptEventsRecordSpec d old wire (.failure final consumed offset extraDepth reason)) : d + extraDepth ≤ 32 := by
  cases spec with
  | malformed => simpa using depth
  | unsupported => simpa using depth
  | rejected tag failure => exact receiptEventsBodyRejected_depth _ _ _ _ _ _ _ _ depth failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsItemRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsItemRejected_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsPairsRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsPairsRejected_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsBodyRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsBodyRejected_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsRecord_failure_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsRecord_failure_depth
