import ProofCheck.ReceiptPointerSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive ReceiptPointerAPIError where
  | nonzero
  | decode (offset : Nat) (reason : ReceiptFailureReason)
  | trailing (extra : Nat)

inductive ReceiptPointerFirstResult where
  | success (value : Option ReceiptValue) (rest : List w8)
  | failure (value : Option ReceiptValue) (error : ReceiptPointerAPIError)

/-- Independent caller contract: nonzero destinations are rejected unchanged;
    parsing starts from zero, success exposes the unread suffix, and any record
    failure clears all partially published fields before returning its error. -/
inductive ReceiptPointerFirstSpec :
    Option ReceiptValue → List w8 → ReceiptPointerFirstResult → Prop where
  | nonzero {old wire} (nonzero : old ≠ none) :
      ReceiptPointerFirstSpec old wire (.failure old .nonzero)
  | success {wire final rest}
      (valid : ReceiptPointerSpec 0 wire (.success final rest)) :
      ReceiptPointerFirstSpec none wire (.success final rest)
  | rejected {wire retained consumed offset extraDepth reason}
      (failure : ReceiptPointerSpec 0 wire (.failure retained consumed offset extraDepth reason)) :
      ReceiptPointerFirstSpec none wire (.failure none (.decode offset reason))

/-- Every initial destination and input has a specified First outcome, without
    an implementation or helper-correctness premise. -/
theorem receiptPointerFirst_classify (old : Option ReceiptValue) (wire : List w8) :
    ∃ result, ReceiptPointerFirstSpec old wire result := by
  classical
  by_cases zero : old = none
  · subst old
    obtain ⟨result, spec, _⟩ := receiptPointerSpec_total_unique 0 wire (by decide)
    cases result with
    | success final rest => exact ⟨_, .success spec⟩
    | failure retained consumed offset extraDepth reason => exact ⟨_, .rejected spec⟩
  · exact ⟨_, .nonzero zero⟩

/-- Uniqueness includes decoded values, exact suffix and public error
    reason/offset; cleanup hides the record reader's retained partial value. -/
theorem receiptPointerFirst_unique (old : Option ReceiptValue) (wire : List w8)
    (a b : ReceiptPointerFirstResult) (first : ReceiptPointerFirstSpec old wire a)
    (second : ReceiptPointerFirstSpec old wire b) : a = b := by
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
      have same := receiptPointerSpec_unique 0 wire _ _ valid valid2
      cases same
      rfl
    | rejected failure =>
      have same := receiptPointerSpec_unique 0 wire _ _ valid failure
      cases same
  | rejected failure =>
    cases second with
    | nonzero nonzero => exact False.elim (nonzero rfl)
    | success valid =>
      have same := receiptPointerSpec_unique 0 wire _ _ failure valid
      cases same
    | rejected failure2 =>
      have same := receiptPointerSpec_unique 0 wire _ _ failure failure2
      cases same
      rfl

/-- A rejected nonzero destination is preserved; all parser failures clear it. -/
theorem receiptPointerFirst_cleanup (old final : Option ReceiptValue)
    (wire : List w8) (error : ReceiptPointerAPIError)
    (spec : ReceiptPointerFirstSpec old wire (.failure final error)) :
    (final = old ∧ error = .nonzero) ∨ (old = none ∧ final = none) := by
  cases spec with
  | nonzero => exact Or.inl ⟨rfl, rfl⟩
  | rejected => exact Or.inr ⟨rfl, rfl⟩

/-- Successful First calls start from zero and satisfy the complete independent
    record relation, rather than a weaker wrapper-specific encoding rule. -/
theorem receiptPointerFirst_success (old final : Option ReceiptValue)
    (wire rest : List w8) (spec : ReceiptPointerFirstSpec old wire (.success final rest)) :
    old = none ∧ ReceiptPointerSpec 0 wire (.success final rest) := by
  cases spec with
  | success valid => exact ⟨rfl, valid⟩

/-- The reported parser offset is within the supplied input or exactly at EOF,
    including the consumed-value boundary for bare FeeUnit overflow. -/
theorem receiptPointerFirst_diagnostic_bound (old final : Option ReceiptValue)
    (wire : List w8) (offset : Nat) (reason : ReceiptFailureReason)
    (spec : ReceiptPointerFirstSpec old wire (.failure final (.decode offset reason))) :
    offset ≤ wire.length := by
  cases spec with
  | rejected failure => exact (receiptPointerRejected_bounds _ _ _ _ _ _ _ failure).2


inductive ReceiptPointerDecodeResult where
  | success (value : Option ReceiptValue)
  | failure (value : Option ReceiptValue) (error : ReceiptPointerAPIError)

/-- Full decoding rejects a nonempty suffix and clears the decoded receipt. -/
inductive ReceiptPointerFullSpec (old : Option ReceiptValue) (wire : List w8) : ReceiptPointerDecodeResult → Prop where
  | firstError {value error} (first : ReceiptPointerFirstSpec old wire (.failure value error)) :
      ReceiptPointerFullSpec old wire (.failure value error)
  | success {value} (first : ReceiptPointerFirstSpec old wire (.success value [])) :
      ReceiptPointerFullSpec old wire (.success value)
  | trailing {value rest} (first : ReceiptPointerFirstSpec old wire (.success value rest)) (nonempty : rest ≠ []) :
      ReceiptPointerFullSpec old wire (.failure none (.trailing rest.length))

theorem receiptPointerFull_classify (old : Option ReceiptValue) (wire : List w8) :
    ∃ result, ReceiptPointerFullSpec old wire result := by
  obtain ⟨result, first⟩ := receiptPointerFirst_classify old wire
  cases result with
  | failure value error => exact ⟨_, .firstError first⟩
  | success value rest =>
    by_cases empty : rest = []
    · subst rest; exact ⟨_, .success first⟩
    · exact ⟨_, .trailing first empty⟩

theorem receiptPointerFull_cleanup (old value : Option ReceiptValue) (wire : List w8) (error : ReceiptPointerAPIError)
    (spec : ReceiptPointerFullSpec old wire (.failure value error)) :
    (value = old ∧ error = .nonzero) ∨ value = none := by
  cases spec with
  | firstError first =>
    rcases receiptPointerFirst_cleanup old value wire error first with guard | cleared
    · exact Or.inl guard
    · exact Or.inr cleared.2
  | trailing => exact Or.inr rfl

theorem receiptPointerFull_success (old value : Option ReceiptValue) (wire : List w8)
    (spec : ReceiptPointerFullSpec old wire (.success value)) :
    old = none ∧ ReceiptPointerValid 0 wire [] value := by
  cases spec with
  | success first =>
    obtain ⟨zero, item, split, valid⟩ := receiptPointerFirst_success old value wire [] first
    simp only [List.append_nil] at split
    subst wire
    exact ⟨zero, valid⟩

theorem receiptPointerFull_unique (old : Option ReceiptValue) (wire : List w8) (a b : ReceiptPointerDecodeResult)
    (first : ReceiptPointerFullSpec old wire a) (second : ReceiptPointerFullSpec old wire b) : a = b := by
  cases first with
  | firstError f => cases second with
    | firstError s => have h := receiptPointerFirst_unique old wire _ _ f s; cases h; rfl
    | success s => have h := receiptPointerFirst_unique old wire _ _ f s; cases h
    | trailing s _ => have h := receiptPointerFirst_unique old wire _ _ f s; cases h
  | success f => cases second with
    | firstError s => have h := receiptPointerFirst_unique old wire _ _ f s; cases h
    | success s => have h := receiptPointerFirst_unique old wire _ _ f s; cases h; rfl
    | trailing s ne => have h := receiptPointerFirst_unique old wire _ _ f s; cases h; exact False.elim (ne rfl)
  | trailing f ne => cases second with
    | firstError s => have h := receiptPointerFirst_unique old wire _ _ f s; cases h
    | success s => have h := receiptPointerFirst_unique old wire _ _ f s; cases h; exact False.elim (ne rfl)
    | trailing s _ => have h := receiptPointerFirst_unique old wire _ _ f s; cases h; rfl

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptPointerFirst_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptPointerFirst_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptPointerFirst_cleanup
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptPointerFirst_success

#print axioms Perennial.github_com.NethermindEth.juno.core.receiptPointerFirst_diagnostic_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptPointerFull_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptPointerFull_cleanup
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptPointerFull_success
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptPointerFull_unique
