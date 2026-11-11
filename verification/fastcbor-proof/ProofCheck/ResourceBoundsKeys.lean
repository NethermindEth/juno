import ProofCheck.ResourceBoundsValues
import ProofCheck.SchemaKeySpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def ResourceBoundsKeyMeaning (key : GoString) : Option ResourceBoundsField → Prop
  | some field => key.map fieldFoldByte = (resourceBoundsFieldName field).map fieldFoldByte
  | none => ∀ field, key.map fieldFoldByte ≠ (resourceBoundsFieldName field).map fieldFoldByte

/-- The two ResourceBounds names remain distinct under the decoder's ASCII folding. -/
theorem resourceBoundsField_fold_injective (a b : ResourceBoundsField)
    (same : (resourceBoundsFieldName a).map fieldFoldByte = (resourceBoundsFieldName b).map fieldFoldByte) : a = b := by
  revert same
  cases a <;> cases b <;> decide

theorem resourceBoundsKeyMeaning_total (key : GoString) : ∃ choice, ResourceBoundsKeyMeaning key choice := by
  classical
  by_cases known : ∃ field, key.map fieldFoldByte = (resourceBoundsFieldName field).map fieldFoldByte
  · obtain ⟨field, found⟩ := known
    exact ⟨some field, found⟩
  · exact ⟨none, fun field same => known ⟨field, same⟩⟩

theorem resourceBoundsKeyMeaning_unique (key : GoString) (a b : Option ResourceBoundsField)
    (first : ResourceBoundsKeyMeaning key a) (second : ResourceBoundsKeyMeaning key b) : a = b := by
  cases a with
  | none => cases b with
    | none => rfl
    | some field => exact False.elim (first field second)
  | some field => cases b with
    | none => exact False.elim (second field first)
    | some other => exact congrArg some (resourceBoundsField_fold_injective field other (first.symm.trans second))

abbrev ResourceBoundsKeyValid := SchemaKeyValid ResourceBoundsKeyMeaning

theorem resourceBoundsKey_classify (wire : List w8) :
    (∃ item rest choice, wire = item ++ rest ∧ ResourceBoundsKeyValid item choice) ∨
    (∃ reason, KeyRejected wire reason) :=
  schemaKey_classify ResourceBoundsKeyMeaning resourceBoundsKeyMeaning_total wire

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsField_fold_injective
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsKeyMeaning_total
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsKeyMeaning_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsKey_classify
