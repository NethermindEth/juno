import ProofCheck.TransactionResultSpec

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive TransactionAPIError where
  | nonzero
  | decode (offset : Nat) (detail : TransactionDispatchResult)
  | trailing (extra : Nat)

inductive TransactionFirstResult where
  | success (value : TransactionValue) (rest : List w8)
  | failure (value : GoInterface) (error : TransactionAPIError)

/-- Independent caller contract: nonzero destinations are rejected unchanged;
    parsing starts from zero, success exposes the unread suffix, and any record
    failure clears all partially published fields before returning its error. -/
inductive TransactionFirstSpec (tags : CBORTransactionTags) :
    GoInterface → List w8 → TransactionFirstResult → Prop where
  | nonzero {old wire} (nonzero : old ≠ interface.nil) :
      TransactionFirstSpec tags old wire (.failure old .nonzero)
  | success {wire final rest}
      (valid : TransactionSpec 0 tags wire (.success final rest)) :
      TransactionFirstSpec tags interface.nil wire (.success final rest)
  | rejected {wire detail consumed offset extraDepth}
      (failure : TransactionSpec 0 tags wire (.failure detail consumed offset extraDepth)) :
      TransactionFirstSpec tags interface.nil wire (.failure interface.nil (.decode offset detail))

/-- Every initial destination and input has a specified First outcome, without
    an implementation or helper-correctness premise. -/
theorem transactionFirst_classify (tags : CBORTransactionTags) (distinct : TransactionTagsDistinct tags) (old : GoInterface) (wire : List w8) :
    ∃ result, TransactionFirstSpec tags old wire result := by
  classical
  by_cases zero : old = interface.nil
  · subst old
    obtain ⟨result, spec, _⟩ := transactionSpec_total_unique 0 tags wire (by decide) distinct
    cases result with
    | success final rest => exact ⟨_, .success spec⟩
    | failure detail consumed offset extraDepth => exact ⟨_, .rejected spec⟩
  · exact ⟨_, .nonzero zero⟩

/-- Uniqueness includes decoded values, exact suffix and public error
    reason/offset; retained partial records stay private to the error ownership. -/
theorem transactionFirst_unique (tags : CBORTransactionTags) (distinct : TransactionTagsDistinct tags) (old : GoInterface) (wire : List w8)
    (a b : TransactionFirstResult) (first : TransactionFirstSpec tags old wire a)
    (second : TransactionFirstSpec tags old wire b) : a = b := by
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
      have same := transactionSpec_unique 0 tags wire distinct _ _ valid valid2
      cases same
      rfl
    | rejected failure =>
      have same := transactionSpec_unique 0 tags wire distinct _ _ valid failure
      cases same
  | rejected failure =>
    cases second with
    | nonzero nonzero => exact False.elim (nonzero rfl)
    | success valid =>
      have same := transactionSpec_unique 0 tags wire distinct _ _ failure valid
      cases same
    | rejected failure2 =>
      have same := transactionSpec_unique 0 tags wire distinct _ _ failure failure2
      cases same
      rfl

/-- A rejected nonzero destination is preserved; all parser failures clear it. -/
theorem transactionFirst_cleanup (tags : CBORTransactionTags) (old final : GoInterface)
    (wire : List w8) (error : TransactionAPIError)
    (spec : TransactionFirstSpec tags old wire (.failure final error)) :
    (final = old ∧ error = .nonzero) ∨ (old = interface.nil ∧ final = interface.nil) := by
  cases spec with
  | nonzero => exact Or.inl ⟨rfl, rfl⟩
  | rejected => exact Or.inr ⟨rfl, rfl⟩

/-- Successful First calls start from zero and satisfy the complete independent
    record relation, rather than a weaker wrapper-specific encoding rule. -/
theorem transactionFirst_success (tags : CBORTransactionTags) (old : GoInterface) (final : TransactionValue)
    (wire rest : List w8) (spec : TransactionFirstSpec tags old wire (.success final rest)) :
    old = interface.nil ∧ TransactionSpec 0 tags wire (.success final rest) := by
  cases spec with
  | success valid => exact ⟨rfl, valid⟩

inductive TransactionDecodeResult where
  | success (value : TransactionValue)
  | failure (value : GoInterface) (error : TransactionAPIError)

/-- Full decoding rejects a nonempty suffix and clears the decoded transaction. -/
inductive TransactionFullSpec (tags : CBORTransactionTags) (old : GoInterface) (wire : List w8) : TransactionDecodeResult → Prop where
  | firstError {value error} (first : TransactionFirstSpec tags old wire (.failure value error)) :
      TransactionFullSpec tags old wire (.failure value error)
  | success {value} (first : TransactionFirstSpec tags old wire (.success value [])) :
      TransactionFullSpec tags old wire (.success value)
  | trailing {value rest} (first : TransactionFirstSpec tags old wire (.success value rest)) (nonempty : rest ≠ []) :
      TransactionFullSpec tags old wire (.failure interface.nil (.trailing rest.length))

theorem transactionFull_classify (tags : CBORTransactionTags) (distinct : TransactionTagsDistinct tags) (old : GoInterface) (wire : List w8) :
    ∃ result, TransactionFullSpec tags old wire result := by
  obtain ⟨result, first⟩ := transactionFirst_classify tags distinct old wire
  cases result with
  | failure value error => exact ⟨_, .firstError first⟩
  | success value rest =>
    by_cases empty : rest = []
    · subst rest; exact ⟨_, .success first⟩
    · exact ⟨_, .trailing first empty⟩

theorem transactionFull_cleanup (tags : CBORTransactionTags) (old value : GoInterface) (wire : List w8) (error : TransactionAPIError)
    (spec : TransactionFullSpec tags old wire (.failure value error)) :
    (value = old ∧ error = .nonzero) ∨ value = interface.nil := by
  cases spec with
  | firstError first =>
    rcases transactionFirst_cleanup tags old value wire error first with guard | cleared
    · exact Or.inl guard
    · exact Or.inr cleared.2
  | trailing => exact Or.inr rfl

theorem transactionFull_success (tags : CBORTransactionTags) (old : GoInterface) (value : TransactionValue)
    (wire : List w8) (spec : TransactionFullSpec tags old wire (.success value)) :
    old = interface.nil ∧ TransactionSpec 0 tags wire (.success value []) := by
  cases spec with
  | success first => exact transactionFirst_success tags old value wire [] first

theorem transactionFull_unique (tags : CBORTransactionTags) (distinct : TransactionTagsDistinct tags) (old : GoInterface) (wire : List w8) (a b : TransactionDecodeResult)
    (first : TransactionFullSpec tags old wire a) (second : TransactionFullSpec tags old wire b) : a = b := by
  cases first with
  | firstError f => cases second with
    | firstError s => have h := transactionFirst_unique tags distinct old wire _ _ f s; cases h; rfl
    | success s => have h := transactionFirst_unique tags distinct old wire _ _ f s; cases h
    | trailing s _ => have h := transactionFirst_unique tags distinct old wire _ _ f s; cases h
  | success f => cases second with
    | firstError s => have h := transactionFirst_unique tags distinct old wire _ _ f s; cases h
    | success s => have h := transactionFirst_unique tags distinct old wire _ _ f s; cases h; rfl
    | trailing s ne => have h := transactionFirst_unique tags distinct old wire _ _ f s; cases h; exact False.elim (ne rfl)
  | trailing f ne => cases second with
    | firstError s => have h := transactionFirst_unique tags distinct old wire _ _ f s; cases h
    | success s => have h := transactionFirst_unique tags distinct old wire _ _ f s; cases h; exact False.elim (ne rfl)
    | trailing s _ => have h := transactionFirst_unique tags distinct old wire _ _ f s; cases h; rfl

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionFirst_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionFirst_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionFirst_cleanup
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionFirst_success
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionFull_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionFull_cleanup
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionFull_success
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionFull_unique
