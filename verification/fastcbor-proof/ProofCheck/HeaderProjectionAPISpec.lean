import ProofCheck.HeaderProjectionRecordUnique
import ProofCheck.HeaderProjectionBounds

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive HeaderProjectionAPIError where
  | nonzero
  | decode (offset : Nat) (reason : HeaderProjectionRecordFailure)

inductive HeaderProjectionFirstResult where
  | success (value : HeaderProjectionValue) (rest : List w8)
  | failure (value : HeaderProjectionValue) (error : HeaderProjectionAPIError)

/-- Independent caller contract: nonzero destinations are rejected unchanged;
    parsing starts from zero, success exposes the unread suffix, and any record
    failure clears all partially published fields before returning its error. -/
inductive HeaderProjectionFirstSpec (kind : HeaderProjectionKind) :
    HeaderProjectionValue → List w8 → HeaderProjectionFirstResult → Prop where
  | nonzero {old wire} (nonzero : old ≠ headerProjectionZero) :
      HeaderProjectionFirstSpec kind old wire (.failure old .nonzero)
  | success {wire final rest}
      (valid : HeaderProjectionRecordSpec 0 kind headerProjectionZero wire (.success final rest)) :
      HeaderProjectionFirstSpec kind headerProjectionZero wire (.success final rest)
  | rejected {wire retained consumed offset entered reason}
      (failure : HeaderProjectionRecordSpec 0 kind headerProjectionZero wire (.failure retained consumed offset entered reason)) :
      HeaderProjectionFirstSpec kind headerProjectionZero wire (.failure headerProjectionZero (.decode offset reason))

/-- Every initial destination and input has a specified First outcome, without
    an implementation or helper-correctness premise. -/
theorem headerProjectionFirst_classify (kind : HeaderProjectionKind) (old : HeaderProjectionValue) (wire : List w8) :
    ∃ result, HeaderProjectionFirstSpec kind old wire result := by
  classical
  by_cases zero : old = headerProjectionZero
  · subst old
    obtain ⟨result, spec⟩ := headerProjectionRecord_classify 0 kind headerProjectionZero wire (by decide)
    cases result with
    | success final rest => exact ⟨_, .success spec⟩
    | failure retained consumed offset entered reason => exact ⟨_, .rejected spec⟩
  · exact ⟨_, .nonzero zero⟩

/-- Uniqueness includes first-occurrence values, exact suffix and public error
    reason/offset; cleanup hides the record reader's retained partial value. -/
theorem headerProjectionFirst_unique (kind : HeaderProjectionKind) (old : HeaderProjectionValue) (wire : List w8)
    (a b : HeaderProjectionFirstResult) (first : HeaderProjectionFirstSpec kind old wire a)
    (second : HeaderProjectionFirstSpec kind old wire b) : a = b := by
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
      have same := headerProjectionRecord_unique 0 kind headerProjectionZero wire _ _ valid valid2
      cases same
      rfl
    | rejected failure =>
      have same := headerProjectionRecord_unique 0 kind headerProjectionZero wire _ _ valid failure
      cases same
  | rejected failure =>
    cases second with
    | nonzero nonzero => exact False.elim (nonzero rfl)
    | success valid =>
      have same := headerProjectionRecord_unique 0 kind headerProjectionZero wire _ _ failure valid
      cases same
    | rejected failure2 =>
      have same := headerProjectionRecord_unique 0 kind headerProjectionZero wire _ _ failure failure2
      cases same
      rfl

/-- A rejected nonzero destination is preserved; all parser failures clear it. -/
theorem headerProjectionFirst_cleanup (kind : HeaderProjectionKind) (old final : HeaderProjectionValue)
    (wire : List w8) (error : HeaderProjectionAPIError)
    (spec : HeaderProjectionFirstSpec kind old wire (.failure final error)) :
    (final = old ∧ error = .nonzero) ∨ (old = headerProjectionZero ∧ final = headerProjectionZero) := by
  cases spec with
  | nonzero => exact Or.inl ⟨rfl, rfl⟩
  | rejected => exact Or.inr ⟨rfl, rfl⟩

/-- Successful First calls start from zero and satisfy the complete independent
    tagged record relation, rather than a weaker wrapper-specific encoding rule. -/
theorem headerProjectionFirst_success (kind : HeaderProjectionKind) (old final : HeaderProjectionValue)
    (wire rest : List w8) (spec : HeaderProjectionFirstSpec kind old wire (.success final rest)) :
    old = headerProjectionZero ∧ HeaderProjectionRecordSpec 0 kind headerProjectionZero wire (.success final rest) := by
  cases spec with
  | success valid => exact ⟨rfl, valid⟩

/-- The reported parser offset is within the supplied input or exactly at EOF,
    including bare unsupported-tag rejection positions. -/
theorem headerProjectionFirst_diagnostic_bound (kind : HeaderProjectionKind) (old final : HeaderProjectionValue)
    (wire : List w8) (offset : Nat) (reason : HeaderProjectionRecordFailure)
    (spec : HeaderProjectionFirstSpec kind old wire (.failure final (.decode offset reason))) :
    offset ≤ wire.length := by
  cases spec with
  | rejected failure => exact (headerProjectionRecord_failure_bounds _ _ _ _ _ _ _ _ _ failure).2

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionFirst_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionFirst_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionFirst_cleanup
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionFirst_success

#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionFirst_diagnostic_bound
