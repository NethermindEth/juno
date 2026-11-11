import ProofCheck.ReceiptStatusRecordUnique
import ProofCheck.ReceiptStatusBounds

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive ReceiptStatusAPIError where
  | nonzero
  | decode (offset : Nat) (reason : ReceiptStatusRecordFailure)

inductive ReceiptStatusFirstResult where
  | success (value : ReceiptStatusValue) (rest : List w8)
  | failure (value : ReceiptStatusValue) (error : ReceiptStatusAPIError)

/-- Independent caller contract: nonzero destinations are rejected unchanged;
    parsing starts from zero, success exposes the unread suffix, and any record
    failure clears all partially published fields before returning its error. -/
inductive ReceiptStatusFirstSpec :
    ReceiptStatusValue → List w8 → ReceiptStatusFirstResult → Prop where
  | nonzero {old wire} (nonzero : old ≠ receiptStatusZero) :
      ReceiptStatusFirstSpec old wire (.failure old .nonzero)
  | success {wire final rest}
      (valid : ReceiptStatusRecordSpec 0 receiptStatusZero wire (.success final rest)) :
      ReceiptStatusFirstSpec receiptStatusZero wire (.success final rest)
  | rejected {wire retained consumed offset entered reason}
      (failure : ReceiptStatusRecordSpec 0 receiptStatusZero wire (.failure retained consumed offset entered reason)) :
      ReceiptStatusFirstSpec receiptStatusZero wire (.failure receiptStatusZero (.decode offset reason))

/-- Every initial destination and input has a specified First outcome, without
    an implementation or helper-correctness premise. -/
theorem receiptStatusFirst_classify (old : ReceiptStatusValue) (wire : List w8) :
    ∃ result, ReceiptStatusFirstSpec old wire result := by
  classical
  by_cases zero : old = receiptStatusZero
  · subst old
    obtain ⟨result, spec⟩ := receiptStatusRecord_classify 0 receiptStatusZero wire (by decide)
    cases result with
    | success final rest => exact ⟨_, .success spec⟩
    | failure retained consumed offset entered reason => exact ⟨_, .rejected spec⟩
  · exact ⟨_, .nonzero zero⟩

/-- Uniqueness includes first-occurrence values, exact suffix and public error
    reason/offset; cleanup hides the record reader's retained partial value. -/
theorem receiptStatusFirst_unique (old : ReceiptStatusValue) (wire : List w8)
    (a b : ReceiptStatusFirstResult) (first : ReceiptStatusFirstSpec old wire a)
    (second : ReceiptStatusFirstSpec old wire b) : a = b := by
  cases first with
  | nonzero nonzero =>
    cases second with
    | nonzero => rfl
    | success => exact False.elim (nonzero rfl)
    | rejected => exact False.elim (nonzero rfl)
  | success valid =>
    cases second with
    | nonzero nonzero => exact False.elim (nonzero rfl)
    | success valid2 =>
      have same := receiptStatusRecord_unique 0 receiptStatusZero wire _ _ valid valid2
      cases same
      rfl
    | rejected failure =>
      have same := receiptStatusRecord_unique 0 receiptStatusZero wire _ _ valid failure
      cases same
  | rejected failure =>
    cases second with
    | nonzero nonzero => exact False.elim (nonzero rfl)
    | success valid =>
      have same := receiptStatusRecord_unique 0 receiptStatusZero wire _ _ failure valid
      cases same
    | rejected failure2 =>
      have same := receiptStatusRecord_unique 0 receiptStatusZero wire _ _ failure failure2
      cases same
      rfl

/-- A rejected nonzero destination is preserved; all parser failures clear it. -/
theorem receiptStatusFirst_cleanup (old final : ReceiptStatusValue)
    (wire : List w8) (error : ReceiptStatusAPIError)
    (spec : ReceiptStatusFirstSpec old wire (.failure final error)) :
    (final = old ∧ error = .nonzero) ∨ (old = receiptStatusZero ∧ final = receiptStatusZero) := by
  cases spec with
  | nonzero => exact Or.inl ⟨rfl, rfl⟩
  | rejected => exact Or.inr ⟨rfl, rfl⟩

/-- Successful First calls start from zero and satisfy the complete independent
    tagged record relation, rather than a weaker wrapper-specific encoding rule. -/
theorem receiptStatusFirst_success (old final : ReceiptStatusValue)
    (wire rest : List w8) (spec : ReceiptStatusFirstSpec old wire (.success final rest)) :
    old = receiptStatusZero ∧ ReceiptStatusRecordSpec 0 receiptStatusZero wire (.success final rest) := by
  cases spec with
  | success valid => exact ⟨rfl, valid⟩

/-- The reported parser offset is within the supplied input or exactly at EOF,
    including bare unsupported-tag rejection positions. -/
theorem receiptStatusFirst_diagnostic_bound (old final : ReceiptStatusValue)
    (wire : List w8) (offset : Nat) (reason : ReceiptStatusRecordFailure)
    (spec : ReceiptStatusFirstSpec old wire (.failure final (.decode offset reason))) :
    offset ≤ wire.length := by
  cases spec with
  | rejected failure => exact (receiptStatusRecord_failure_bounds _ _ _ _ _ _ _ _ failure).2

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusFirst_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusFirst_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusFirst_cleanup
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusFirst_success

#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusFirst_diagnostic_bound
