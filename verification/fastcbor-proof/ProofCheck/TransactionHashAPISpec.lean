import ProofCheck.TransactionHashRecordUnique
import ProofCheck.TransactionHashBounds

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive TransactionHashAPIError where
  | nonzero
  | decode (offset : Nat) (reason : TransactionHashRecordFailure)

inductive TransactionHashFirstResult where
  | success (value : TransactionHashValue) (rest : List w8)
  | failure (value : TransactionHashValue) (error : TransactionHashAPIError)

/-- Independent caller contract: nonzero destinations are rejected unchanged;
    parsing starts from zero, success exposes the unread suffix, and any record
    failure clears all partially published fields before returning its error. -/
inductive TransactionHashFirstSpec :
    TransactionHashValue → List w8 → TransactionHashFirstResult → Prop where
  | nonzero {old wire} (nonzero : old ≠ transactionHashZero) :
      TransactionHashFirstSpec old wire (.failure old .nonzero)
  | success {wire final rest}
      (valid : TransactionHashRecordSpec 0 transactionHashZero wire (.success final rest)) :
      TransactionHashFirstSpec transactionHashZero wire (.success final rest)
  | rejected {wire retained consumed offset entered reason}
      (failure : TransactionHashRecordSpec 0 transactionHashZero wire (.failure retained consumed offset entered reason)) :
      TransactionHashFirstSpec transactionHashZero wire (.failure transactionHashZero (.decode offset reason))

/-- Every initial destination and input has a specified First outcome, without
    an implementation or helper-correctness premise. -/
theorem transactionHashFirst_classify (old : TransactionHashValue) (wire : List w8) :
    ∃ result, TransactionHashFirstSpec old wire result := by
  classical
  by_cases zero : old = transactionHashZero
  · subst old
    obtain ⟨result, spec⟩ := transactionHashRecord_classify 0 transactionHashZero wire (by decide)
    cases result with
    | success final rest => exact ⟨_, .success spec⟩
    | failure retained consumed offset entered reason => exact ⟨_, .rejected spec⟩
  · exact ⟨_, .nonzero zero⟩

/-- Uniqueness includes first-occurrence values, exact suffix and public error
    reason/offset; cleanup hides the record reader's retained partial value. -/
theorem transactionHashFirst_unique (old : TransactionHashValue) (wire : List w8)
    (a b : TransactionHashFirstResult) (first : TransactionHashFirstSpec old wire a)
    (second : TransactionHashFirstSpec old wire b) : a = b := by
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
      have same := transactionHashRecord_unique 0 transactionHashZero wire _ _ valid valid2
      cases same
      rfl
    | rejected failure =>
      have same := transactionHashRecord_unique 0 transactionHashZero wire _ _ valid failure
      cases same
  | rejected failure =>
    cases second with
    | nonzero nonzero => exact False.elim (nonzero rfl)
    | success valid =>
      have same := transactionHashRecord_unique 0 transactionHashZero wire _ _ failure valid
      cases same
    | rejected failure2 =>
      have same := transactionHashRecord_unique 0 transactionHashZero wire _ _ failure failure2
      cases same
      rfl

/-- A rejected nonzero destination is preserved; all parser failures clear it. -/
theorem transactionHashFirst_cleanup (old final : TransactionHashValue)
    (wire : List w8) (error : TransactionHashAPIError)
    (spec : TransactionHashFirstSpec old wire (.failure final error)) :
    (final = old ∧ error = .nonzero) ∨ (old = transactionHashZero ∧ final = transactionHashZero) := by
  cases spec with
  | nonzero => exact Or.inl ⟨rfl, rfl⟩
  | rejected => exact Or.inr ⟨rfl, rfl⟩

/-- Successful First calls start from zero and satisfy the complete independent
    tagged record relation, rather than a weaker wrapper-specific encoding rule. -/
theorem transactionHashFirst_success (old final : TransactionHashValue)
    (wire rest : List w8) (spec : TransactionHashFirstSpec old wire (.success final rest)) :
    old = transactionHashZero ∧ TransactionHashRecordSpec 0 transactionHashZero wire (.success final rest) := by
  cases spec with
  | success valid => exact ⟨rfl, valid⟩

/-- The reported parser offset is within the supplied input or exactly at EOF,
    including bare unsupported-tag rejection positions. -/
theorem transactionHashFirst_diagnostic_bound (old final : TransactionHashValue)
    (wire : List w8) (offset : Nat) (reason : TransactionHashRecordFailure)
    (spec : TransactionHashFirstSpec old wire (.failure final (.decode offset reason))) :
    offset ≤ wire.length := by
  cases spec with
  | rejected failure => exact (transactionHashRecord_failure_bounds _ _ _ _ _ _ _ _ failure).2

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashFirst_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashFirst_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashFirst_cleanup
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashFirst_success

#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashFirst_diagnostic_bound
