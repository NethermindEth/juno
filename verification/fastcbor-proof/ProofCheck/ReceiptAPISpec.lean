import ProofCheck.ReceiptRecordUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive ReceiptAPIError where
  | nonzero
  | decode (offset : Nat) (reason : ReceiptFailureReason)
  | trailing (extra : Nat)

inductive ReceiptFirstResult where
  | success (value : ReceiptValue) (rest : List w8)
  | failure (value : ReceiptValue) (error : ReceiptAPIError)

/-- Independent caller contract: nonzero destinations are rejected unchanged;
    parsing starts from zero, success exposes the unread suffix, and any record
    failure clears all partially published fields before returning its error. -/
inductive ReceiptFirstSpec :
    ReceiptValue → List w8 → ReceiptFirstResult → Prop where
  | nonzero {old wire} (nonzero : old ≠ receiptZero) :
      ReceiptFirstSpec old wire (.failure old .nonzero)
  | success {wire final rest}
      (valid : ReceiptRecordSpec 0 receiptZero wire (.success final rest)) :
      ReceiptFirstSpec receiptZero wire (.success final rest)
  | rejected {wire retained consumed offset extraDepth reason}
      (failure : ReceiptRecordSpec 0 receiptZero wire (.failure retained consumed offset extraDepth reason)) :
      ReceiptFirstSpec receiptZero wire (.failure receiptZero (.decode offset reason))

/-- Every initial destination and input has a specified First outcome, without
    an implementation or helper-correctness premise. -/
theorem receiptFirst_classify (old : ReceiptValue) (wire : List w8) :
    ∃ result, ReceiptFirstSpec old wire result := by
  classical
  by_cases zero : old = receiptZero
  · subst old
    obtain ⟨result, spec, _⟩ := receiptRecordSpec_total_unique 0 receiptZero wire (by decide)
    cases result with
    | success final rest => exact ⟨_, .success spec⟩
    | failure retained consumed offset extraDepth reason => exact ⟨_, .rejected spec⟩
  · exact ⟨_, .nonzero zero⟩

/-- Uniqueness includes decoded values, exact suffix and public error
    reason/offset; cleanup hides the record reader's retained partial value. -/
theorem receiptFirst_unique (old : ReceiptValue) (wire : List w8)
    (a b : ReceiptFirstResult) (first : ReceiptFirstSpec old wire a)
    (second : ReceiptFirstSpec old wire b) : a = b := by
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
      have same := receiptRecordSpec_unique 0 receiptZero wire _ _ valid valid2
      cases same
      rfl
    | rejected failure =>
      have same := receiptRecordSpec_unique 0 receiptZero wire _ _ valid failure
      cases same
  | rejected failure =>
    cases second with
    | nonzero nonzero => exact False.elim (nonzero rfl)
    | success valid =>
      have same := receiptRecordSpec_unique 0 receiptZero wire _ _ failure valid
      cases same
    | rejected failure2 =>
      have same := receiptRecordSpec_unique 0 receiptZero wire _ _ failure failure2
      cases same
      rfl

/-- A rejected nonzero destination is preserved; all parser failures clear it. -/
theorem receiptFirst_cleanup (old final : ReceiptValue)
    (wire : List w8) (error : ReceiptAPIError)
    (spec : ReceiptFirstSpec old wire (.failure final error)) :
    (final = old ∧ error = .nonzero) ∨ (old = receiptZero ∧ final = receiptZero) := by
  cases spec with
  | nonzero => exact Or.inl ⟨rfl, rfl⟩
  | rejected => exact Or.inr ⟨rfl, rfl⟩

/-- Successful First calls start from zero and satisfy the complete independent
    record relation, rather than a weaker wrapper-specific encoding rule. -/
theorem receiptFirst_success (old final : ReceiptValue)
    (wire rest : List w8) (spec : ReceiptFirstSpec old wire (.success final rest)) :
    old = receiptZero ∧ ReceiptRecordSpec 0 receiptZero wire (.success final rest) := by
  cases spec with
  | success valid => exact ⟨rfl, valid⟩

/-- The reported parser offset is within the supplied input or exactly at EOF,
    including the consumed-value boundary for bare FeeUnit overflow. -/
theorem receiptFirst_diagnostic_bound (old final : ReceiptValue)
    (wire : List w8) (offset : Nat) (reason : ReceiptFailureReason)
    (spec : ReceiptFirstSpec old wire (.failure final (.decode offset reason))) :
    offset ≤ wire.length := by
  cases spec with
  | rejected failure => exact (receiptRecordRejected_bounds _ _ _ _ _ _ _ _ failure).2


inductive ReceiptDecodeResult where
  | success (value : ReceiptValue)
  | failure (value : ReceiptValue) (error : ReceiptAPIError)

/-- Full decoding rejects a nonempty suffix and clears the decoded receipt. -/
inductive ReceiptFullSpec (old : ReceiptValue) (wire : List w8) : ReceiptDecodeResult → Prop where
  | firstError {value error} (first : ReceiptFirstSpec old wire (.failure value error)) :
      ReceiptFullSpec old wire (.failure value error)
  | success {value} (first : ReceiptFirstSpec old wire (.success value [])) :
      ReceiptFullSpec old wire (.success value)
  | trailing {value rest} (first : ReceiptFirstSpec old wire (.success value rest)) (nonempty : rest ≠ []) :
      ReceiptFullSpec old wire (.failure receiptZero (.trailing rest.length))

theorem receiptFull_classify (old : ReceiptValue) (wire : List w8) :
    ∃ result, ReceiptFullSpec old wire result := by
  obtain ⟨result, first⟩ := receiptFirst_classify old wire
  cases result with
  | failure value error => exact ⟨_, .firstError first⟩
  | success value rest =>
    by_cases empty : rest = []
    · subst rest; exact ⟨_, .success first⟩
    · exact ⟨_, .trailing first empty⟩

theorem receiptFull_cleanup (old value : ReceiptValue) (wire : List w8) (error : ReceiptAPIError)
    (spec : ReceiptFullSpec old wire (.failure value error)) :
    (value = old ∧ error = .nonzero) ∨ value = receiptZero := by
  cases spec with
  | firstError first =>
    rcases receiptFirst_cleanup old value wire error first with guard | cleared
    · exact Or.inl guard
    · exact Or.inr cleared.2
  | trailing => exact Or.inr rfl

theorem receiptFull_success (old value : ReceiptValue) (wire : List w8)
    (spec : ReceiptFullSpec old wire (.success value)) :
    old = receiptZero ∧ ReceiptRecordValid 0 receiptZero wire [] value := by
  cases spec with
  | success first =>
    obtain ⟨zero, item, split, valid⟩ := receiptFirst_success old value wire [] first
    simp only [List.append_nil] at split
    subst wire
    exact ⟨zero, valid⟩

theorem receiptFull_unique (old : ReceiptValue) (wire : List w8) (a b : ReceiptDecodeResult)
    (first : ReceiptFullSpec old wire a) (second : ReceiptFullSpec old wire b) : a = b := by
  cases first with
  | firstError f => cases second with
    | firstError s => have h := receiptFirst_unique old wire _ _ f s; cases h; rfl
    | success s => have h := receiptFirst_unique old wire _ _ f s; cases h
    | trailing s _ => have h := receiptFirst_unique old wire _ _ f s; cases h
  | success f => cases second with
    | firstError s => have h := receiptFirst_unique old wire _ _ f s; cases h
    | success s => have h := receiptFirst_unique old wire _ _ f s; cases h; rfl
    | trailing s ne => have h := receiptFirst_unique old wire _ _ f s; cases h; exact False.elim (ne rfl)
  | trailing f ne => cases second with
    | firstError s => have h := receiptFirst_unique old wire _ _ f s; cases h
    | success s => have h := receiptFirst_unique old wire _ _ f s; cases h; exact False.elim (ne rfl)
    | trailing s _ => have h := receiptFirst_unique old wire _ _ f s; cases h; rfl

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptFirst_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptFirst_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptFirst_cleanup
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptFirst_success

#print axioms Perennial.github_com.NethermindEth.juno.core.receiptFirst_diagnostic_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptFull_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptFull_cleanup
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptFull_success
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptFull_unique
