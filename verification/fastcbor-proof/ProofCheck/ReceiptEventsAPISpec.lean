import ProofCheck.ReceiptEventsRecordUnique
import ProofCheck.ReceiptEventsBounds

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive ReceiptEventsAPIError where
  | nonzero
  | decode (offset : Nat) (reason : ReceiptEventsRecordFailure)

inductive ReceiptEventsFirstResult where
  | success (value : ReceiptEventsValue) (rest : List w8)
  | failure (value : ReceiptEventsValue) (error : ReceiptEventsAPIError)

/-- Independent caller contract: nonzero destinations are rejected unchanged;
    parsing starts from zero, success exposes the unread suffix, and any record
    failure clears all partially published fields before returning its error. -/
inductive ReceiptEventsFirstSpec :
    ReceiptEventsValue → List w8 → ReceiptEventsFirstResult → Prop where
  | nonzero {old wire} (nonzero : old ≠ receiptEventsZero) :
      ReceiptEventsFirstSpec old wire (.failure old .nonzero)
  | success {wire final rest}
      (valid : ReceiptEventsRecordSpec 0 receiptEventsZero wire (.success final rest)) :
      ReceiptEventsFirstSpec receiptEventsZero wire (.success final rest)
  | rejected {wire retained consumed offset extraDepth reason}
      (failure : ReceiptEventsRecordSpec 0 receiptEventsZero wire (.failure retained consumed offset extraDepth reason)) :
      ReceiptEventsFirstSpec receiptEventsZero wire (.failure receiptEventsZero (.decode offset reason))

/-- Every initial destination and input has a specified First outcome, without
    an implementation or helper-correctness premise. -/
theorem receiptEventsFirst_classify (old : ReceiptEventsValue) (wire : List w8) :
    ∃ result, ReceiptEventsFirstSpec old wire result := by
  classical
  by_cases zero : old = receiptEventsZero
  · subst old
    obtain ⟨result, spec⟩ := receiptEventsRecord_classify 0 receiptEventsZero wire (by decide)
    cases result with
    | success final rest => exact ⟨_, .success spec⟩
    | failure retained consumed offset extraDepth reason => exact ⟨_, .rejected spec⟩
  · exact ⟨_, .nonzero zero⟩

/-- Uniqueness includes first-occurrence values, exact suffix and public error
    reason/offset; cleanup hides the record reader's retained partial value. -/
theorem receiptEventsFirst_unique (old : ReceiptEventsValue) (wire : List w8)
    (a b : ReceiptEventsFirstResult) (first : ReceiptEventsFirstSpec old wire a)
    (second : ReceiptEventsFirstSpec old wire b) : a = b := by
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
      have same := receiptEventsRecord_unique 0 receiptEventsZero wire _ _ valid valid2
      cases same
      rfl
    | rejected failure =>
      have same := receiptEventsRecord_unique 0 receiptEventsZero wire _ _ valid failure
      cases same
  | rejected failure =>
    cases second with
    | nonzero nonzero => exact False.elim (nonzero rfl)
    | success valid =>
      have same := receiptEventsRecord_unique 0 receiptEventsZero wire _ _ failure valid
      cases same
    | rejected failure2 =>
      have same := receiptEventsRecord_unique 0 receiptEventsZero wire _ _ failure failure2
      cases same
      rfl

/-- A rejected nonzero destination is preserved; all parser failures clear it. -/
theorem receiptEventsFirst_cleanup (old final : ReceiptEventsValue)
    (wire : List w8) (error : ReceiptEventsAPIError)
    (spec : ReceiptEventsFirstSpec old wire (.failure final error)) :
    (final = old ∧ error = .nonzero) ∨ (old = receiptEventsZero ∧ final = receiptEventsZero) := by
  cases spec with
  | nonzero => exact Or.inl ⟨rfl, rfl⟩
  | rejected => exact Or.inr ⟨rfl, rfl⟩

/-- Successful First calls start from zero and satisfy the complete independent
    tagged record relation, rather than a weaker wrapper-specific encoding rule. -/
theorem receiptEventsFirst_success (old final : ReceiptEventsValue)
    (wire rest : List w8) (spec : ReceiptEventsFirstSpec old wire (.success final rest)) :
    old = receiptEventsZero ∧ ReceiptEventsRecordSpec 0 receiptEventsZero wire (.success final rest) := by
  cases spec with
  | success valid => exact ⟨rfl, valid⟩

/-- The reported parser offset is within the supplied input or exactly at EOF,
    including bare unsupported-tag rejection positions. -/
theorem receiptEventsFirst_diagnostic_bound (old final : ReceiptEventsValue)
    (wire : List w8) (offset : Nat) (reason : ReceiptEventsRecordFailure)
    (spec : ReceiptEventsFirstSpec old wire (.failure final (.decode offset reason))) :
    offset ≤ wire.length := by
  cases spec with
  | rejected failure => exact (receiptEventsRecord_failure_bounds _ _ _ _ _ _ _ _ failure).2

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsFirst_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsFirst_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsFirst_cleanup
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsFirst_success

#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsFirst_diagnostic_bound
