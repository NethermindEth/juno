import ProofCheck.HeaderProjectionRecord

set_option linter.unusedSectionVars false

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- A selected-field diagnostic lies inside its input or exactly at EOF. -/
theorem headerProjectionFieldRejected_bound (d offset : Nat) (field : HeaderProjectionField)
    (wire : List w8) (reason : HeaderProjectionFieldFailure)
    (failure : HeaderProjectionFieldRejected d field wire offset reason) : offset ≤ wire.length := by
  cases failure with
  | hash failure => exact feltFailure_offset _ _ _ _ failure
  | root failure => exact feltFailure_offset _ _ _ _ failure
  | count => omega
  | timestamp => omega

theorem headerProjectionItemRejected_bound (d offset : Nat) (kind : HeaderProjectionKind)
    (seen : List Nat) (choice : Option Nat) (wire : List w8) (reason : HeaderProjectionFailure)
    (failure : HeaderProjectionItemRejected d kind seen choice wire offset reason) : offset ≤ wire.length := by
  cases failure with
  | ignored ignored failure => exact skipFailure_offset _ _ _ _ failure
  | selected fresh lookup failure => exact headerProjectionFieldRejected_bound _ _ _ _ _ failure

/-- Retained parser positions and diagnostic offsets are independently bounded;
    rejection is impossible once every declared pair has been consumed. -/
theorem headerProjectionPairsRejected_bounds (d n consumed offset : Nat) (kind : HeaderProjectionKind)
    (seen : List Nat) (old final : HeaderProjectionValue) (wire : List w8) (reason : HeaderProjectionFailure)
    (failure : HeaderProjectionPairsRejected d kind n seen old wire final consumed offset reason) :
    consumed ≤ wire.length ∧ offset ≤ wire.length ∧ 0 < n := by
  induction failure with
  | key => simp
  | value keyValid failure =>
    have bound := headerProjectionItemRejected_bound _ _ _ _ _ _ _ failure
    simp only [List.length_append]
    omega
  | next keyValid valueValid failure ih =>
    simp only [List.length_append]
    omega

theorem headerProjectionBodyRejected_bounds (d consumed offset : Nat) (kind : HeaderProjectionKind)
    (old final : HeaderProjectionValue) (wire : List w8) (entered : Bool) (reason : HeaderProjectionRecordFailure)
    (failure : HeaderProjectionBodyRejected d kind old wire final consumed offset entered reason) :
    consumed ≤ wire.length ∧ offset ≤ wire.length := by
  cases failure with
  | opening => simp
  | map enc major length depth fit failure =>
    have bound := headerProjectionPairsRejected_bounds _ _ _ _ _ _ _ _ _ _ failure
    simp only [List.length_append, List.length_cons]
    omega

/-- Adding the optional tag cannot move either failure position beyond the
    original record input. This is independent of the implementation contracts. -/
theorem headerProjectionRecord_failure_bounds (d consumed offset : Nat) (kind : HeaderProjectionKind)
    (old final : HeaderProjectionValue) (wire : List w8) (entered : Bool) (reason : HeaderProjectionRecordFailure)
    (spec : HeaderProjectionRecordSpec d kind old wire (.failure final consumed offset entered reason)) :
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
    have bound := headerProjectionBodyRejected_bounds _ _ _ _ _ _ _ _ _ failure
    subst wire
    simp only [List.length_append]
    omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionFieldRejected_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionItemRejected_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionPairsRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionBodyRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionRecord_failure_bounds
