import ProofCheck.ReceiptStatusRecord

set_option linter.unusedSectionVars false

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- A selected-field diagnostic lies inside its input or exactly at EOF. -/
theorem receiptStatusFieldRejected_bound (offset : Nat) (field : ReceiptStatusField)
    (wire : List w8) (reason : ReceiptStatusFieldFailure)
    (failure : ReceiptStatusFieldRejected field wire offset reason) : offset ≤ wire.length := by
  rw [receiptStatusField_rejected_offset field wire offset reason failure]
  exact Nat.zero_le _

theorem receiptStatusItemRejected_bound (d offset : Nat)
    (seen : List Nat) (choice : Option Nat) (wire : List w8) (reason : ReceiptStatusFailure)
    (failure : ReceiptStatusItemRejected d seen choice wire offset reason) : offset ≤ wire.length := by
  cases failure with
  | ignored ignored failure => exact skipFailure_offset _ _ _ _ failure
  | selected fresh lookup failure => exact receiptStatusFieldRejected_bound _ _ _ _ failure

/-- Retained parser positions and diagnostic offsets are independently bounded;
    rejection is impossible once every declared pair has been consumed. -/
theorem receiptStatusPairsRejected_bounds (d n consumed offset : Nat)
    (seen : List Nat) (old final : ReceiptStatusValue) (wire : List w8) (reason : ReceiptStatusFailure)
    (failure : ReceiptStatusPairsRejected d n seen old wire final consumed offset reason) :
    consumed ≤ wire.length ∧ offset ≤ wire.length ∧ 0 < n := by
  induction failure with
  | key => simp
  | value keyValid failure =>
    have bound := receiptStatusItemRejected_bound _ _ _ _ _ _ failure
    simp only [List.length_append]
    omega
  | next keyValid valueValid failure ih =>
    simp only [List.length_append]
    omega

theorem receiptStatusBodyRejected_bounds (d consumed offset : Nat)
    (old final : ReceiptStatusValue) (wire : List w8) (entered : Bool) (reason : ReceiptStatusRecordFailure)
    (failure : ReceiptStatusBodyRejected d old wire final consumed offset entered reason) :
    consumed ≤ wire.length ∧ offset ≤ wire.length := by
  cases failure with
  | opening => simp
  | map enc major length depth fit failure =>
    have bound := receiptStatusPairsRejected_bounds _ _ _ _ _ _ _ _ _ failure
    simp only [List.length_append, List.length_cons]
    omega

/-- Adding the optional tag cannot move either failure position beyond the
    original record input. This is independent of the implementation contracts. -/
theorem receiptStatusRecord_failure_bounds (d consumed offset : Nat)
    (old final : ReceiptStatusValue) (wire : List w8) (entered : Bool) (reason : ReceiptStatusRecordFailure)
    (spec : ReceiptStatusRecordSpec d old wire (.failure final consumed offset entered reason)) :
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
    have bound := receiptStatusBodyRejected_bounds _ _ _ _ _ _ _ _ failure
    subst wire
    simp only [List.length_append]
    omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusFieldRejected_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusItemRejected_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusPairsRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusBodyRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusRecord_failure_bounds
