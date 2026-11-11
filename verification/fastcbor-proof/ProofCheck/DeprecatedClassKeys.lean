import ProofCheck.DeprecatedClassValues
import ProofCheck.SchemaKeySpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def DeprecatedClassKeyMeaning (key : GoString) : Option DeprecatedClassField → Prop
  | some field => key.map fieldFoldByte = (deprecatedClassFieldName field).map fieldFoldByte
  | none => ∀ field, key.map fieldFoldByte ≠ (deprecatedClassFieldName field).map fieldFoldByte

/-- The five DeprecatedClass names remain distinct under the decoder's ASCII folding. -/
theorem deprecatedClassField_fold_injective (a b : DeprecatedClassField)
    (same : (deprecatedClassFieldName a).map fieldFoldByte = (deprecatedClassFieldName b).map fieldFoldByte) : a = b := by
  revert same
  cases a <;> cases b <;> decide

theorem deprecatedClassKeyMeaning_total (key : GoString) : ∃ choice, DeprecatedClassKeyMeaning key choice := by
  classical
  by_cases known : ∃ field, key.map fieldFoldByte = (deprecatedClassFieldName field).map fieldFoldByte
  · obtain ⟨field, found⟩ := known
    exact ⟨some field, found⟩
  · exact ⟨none, fun field same => known ⟨field, same⟩⟩

theorem deprecatedClassKeyMeaning_unique (key : GoString) (a b : Option DeprecatedClassField)
    (first : DeprecatedClassKeyMeaning key a) (second : DeprecatedClassKeyMeaning key b) : a = b := by
  cases a with
  | none => cases b with
    | none => rfl
    | some field => exact False.elim (first field second)
  | some field => cases b with
    | none => exact False.elim (second field first)
    | some other => exact congrArg some (deprecatedClassField_fold_injective field other (first.symm.trans second))

abbrev DeprecatedClassKeyValid := SchemaKeyValid DeprecatedClassKeyMeaning

theorem deprecatedClassKey_classify (wire : List w8) :
    (∃ item rest choice, wire = item ++ rest ∧ DeprecatedClassKeyValid item choice) ∨
    (∃ reason, KeyRejected wire reason) :=
  schemaKey_classify DeprecatedClassKeyMeaning deprecatedClassKeyMeaning_total wire

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassField_fold_injective
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassKeyMeaning_total
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassKeyMeaning_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassKey_classify
