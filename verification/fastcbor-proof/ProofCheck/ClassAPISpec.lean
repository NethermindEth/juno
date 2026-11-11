import ProofCheck.ClassResultSpec

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive ClassAPIError where
  | nonzero
  | decode (offset : Nat) (detail : ClassDispatchResult)
  | trailing (extra : Nat)

inductive ClassFirstResult where
  | success (value : ClassValue) (rest : List w8)
  | failure (value : GoInterface) (error : ClassAPIError)

/-- Independent caller contract: nonzero destinations are rejected unchanged;
    parsing starts from zero, success exposes the unread suffix, and any record
    failure clears all partially published fields before returning its error. -/
inductive ClassFirstSpec (tags : CBORClassTags) :
    GoInterface → List w8 → ClassFirstResult → Prop where
  | nonzero {old wire} (nonzero : old ≠ interface.nil) :
      ClassFirstSpec tags old wire (.failure old .nonzero)
  | success {wire final rest}
      (valid : ClassSpec 0 tags wire (.success final rest)) :
      ClassFirstSpec tags interface.nil wire (.success final rest)
  | rejected {wire detail consumed offset extraDepth}
      (failure : ClassSpec 0 tags wire (.failure detail consumed offset extraDepth)) :
      ClassFirstSpec tags interface.nil wire (.failure interface.nil (.decode offset detail))

/-- Every initial destination and input has a specified First outcome, without
    an implementation or helper-correctness premise. -/
theorem classFirst_classify (tags : CBORClassTags) (distinct : ClassTagsDistinct tags) (old : GoInterface) (wire : List w8) :
    ∃ result, ClassFirstSpec tags old wire result := by
  classical
  by_cases zero : old = interface.nil
  · subst old
    obtain ⟨result, spec, _⟩ := classSpec_total_unique 0 tags wire (by decide) distinct
    cases result with
    | success final rest => exact ⟨_, .success spec⟩
    | failure detail consumed offset extraDepth => exact ⟨_, .rejected spec⟩
  · exact ⟨_, .nonzero zero⟩

/-- Uniqueness includes decoded values, exact suffix and public error
    reason/offset; retained partial records stay private to the error ownership. -/
theorem classFirst_unique (tags : CBORClassTags) (distinct : ClassTagsDistinct tags) (old : GoInterface) (wire : List w8)
    (a b : ClassFirstResult) (first : ClassFirstSpec tags old wire a)
    (second : ClassFirstSpec tags old wire b) : a = b := by
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
      have same := classSpec_unique 0 tags wire distinct _ _ valid valid2
      cases same
      rfl
    | rejected failure =>
      have same := classSpec_unique 0 tags wire distinct _ _ valid failure
      cases same
  | rejected failure =>
    cases second with
    | nonzero nonzero => exact False.elim (nonzero rfl)
    | success valid =>
      have same := classSpec_unique 0 tags wire distinct _ _ failure valid
      cases same
    | rejected failure2 =>
      have same := classSpec_unique 0 tags wire distinct _ _ failure failure2
      cases same
      rfl

/-- A rejected nonzero destination is preserved; all parser failures clear it. -/
theorem classFirst_cleanup (tags : CBORClassTags) (old final : GoInterface)
    (wire : List w8) (error : ClassAPIError)
    (spec : ClassFirstSpec tags old wire (.failure final error)) :
    (final = old ∧ error = .nonzero) ∨ (old = interface.nil ∧ final = interface.nil) := by
  cases spec with
  | nonzero => exact Or.inl ⟨rfl, rfl⟩
  | rejected => exact Or.inr ⟨rfl, rfl⟩

/-- Successful First calls start from zero and satisfy the complete independent
    record relation, rather than a weaker wrapper-specific encoding rule. -/
theorem classFirst_success (tags : CBORClassTags) (old : GoInterface) (final : ClassValue)
    (wire rest : List w8) (spec : ClassFirstSpec tags old wire (.success final rest)) :
    old = interface.nil ∧ ClassSpec 0 tags wire (.success final rest) := by
  cases spec with
  | success valid => exact ⟨rfl, valid⟩

inductive ClassDecodeResult where
  | success (value : ClassValue)
  | failure (value : GoInterface) (error : ClassAPIError)

/-- Full decoding rejects a nonempty suffix and clears the decoded class. -/
inductive ClassFullSpec (tags : CBORClassTags) (old : GoInterface) (wire : List w8) : ClassDecodeResult → Prop where
  | firstError {value error} (first : ClassFirstSpec tags old wire (.failure value error)) :
      ClassFullSpec tags old wire (.failure value error)
  | success {value} (first : ClassFirstSpec tags old wire (.success value [])) :
      ClassFullSpec tags old wire (.success value)
  | trailing {value rest} (first : ClassFirstSpec tags old wire (.success value rest)) (nonempty : rest ≠ []) :
      ClassFullSpec tags old wire (.failure interface.nil (.trailing rest.length))

theorem classFull_classify (tags : CBORClassTags) (distinct : ClassTagsDistinct tags) (old : GoInterface) (wire : List w8) :
    ∃ result, ClassFullSpec tags old wire result := by
  obtain ⟨result, first⟩ := classFirst_classify tags distinct old wire
  cases result with
  | failure value error => exact ⟨_, .firstError first⟩
  | success value rest =>
    by_cases empty : rest = []
    · subst rest; exact ⟨_, .success first⟩
    · exact ⟨_, .trailing first empty⟩

theorem classFull_cleanup (tags : CBORClassTags) (old value : GoInterface) (wire : List w8) (error : ClassAPIError)
    (spec : ClassFullSpec tags old wire (.failure value error)) :
    (value = old ∧ error = .nonzero) ∨ value = interface.nil := by
  cases spec with
  | firstError first =>
    rcases classFirst_cleanup tags old value wire error first with guard | cleared
    · exact Or.inl guard
    · exact Or.inr cleared.2
  | trailing => exact Or.inr rfl

theorem classFull_success (tags : CBORClassTags) (old : GoInterface) (value : ClassValue)
    (wire : List w8) (spec : ClassFullSpec tags old wire (.success value)) :
    old = interface.nil ∧ ClassSpec 0 tags wire (.success value []) := by
  cases spec with
  | success first => exact classFirst_success tags old value wire [] first

theorem classFull_unique (tags : CBORClassTags) (distinct : ClassTagsDistinct tags) (old : GoInterface) (wire : List w8) (a b : ClassDecodeResult)
    (first : ClassFullSpec tags old wire a) (second : ClassFullSpec tags old wire b) : a = b := by
  cases first with
  | firstError f => cases second with
    | firstError s => have h := classFirst_unique tags distinct old wire _ _ f s; cases h; rfl
    | success s => have h := classFirst_unique tags distinct old wire _ _ f s; cases h
    | trailing s _ => have h := classFirst_unique tags distinct old wire _ _ f s; cases h
  | success f => cases second with
    | firstError s => have h := classFirst_unique tags distinct old wire _ _ f s; cases h
    | success s => have h := classFirst_unique tags distinct old wire _ _ f s; cases h; rfl
    | trailing s ne => have h := classFirst_unique tags distinct old wire _ _ f s; cases h; exact False.elim (ne rfl)
  | trailing f ne => cases second with
    | firstError s => have h := classFirst_unique tags distinct old wire _ _ f s; cases h
    | success s => have h := classFirst_unique tags distinct old wire _ _ f s; cases h; exact False.elim (ne rfl)
    | trailing s _ => have h := classFirst_unique tags distinct old wire _ _ f s; cases h; rfl

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.classFirst_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.classFirst_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.classFirst_cleanup
#print axioms Perennial.github_com.NethermindEth.juno.core.classFirst_success
#print axioms Perennial.github_com.NethermindEth.juno.core.classFull_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.classFull_cleanup
#print axioms Perennial.github_com.NethermindEth.juno.core.classFull_success
#print axioms Perennial.github_com.NethermindEth.juno.core.classFull_unique
