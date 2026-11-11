import ProofCheck.DeprecatedEntryPointValues
import ProofCheck.SchemaKeySpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def DeprecatedEntryPointKeyMeaning (key : GoString) : Option DeprecatedEntryPointField → Prop
  | some field => key.map fieldFoldByte = (deprecatedEntryPointFieldName field).map fieldFoldByte
  | none => ∀ field, key.map fieldFoldByte ≠ (deprecatedEntryPointFieldName field).map fieldFoldByte

/-- The two DeprecatedEntryPoint names remain distinct under the decoder's ASCII folding. -/
theorem deprecatedEntryPointField_fold_injective (a b : DeprecatedEntryPointField)
    (same : (deprecatedEntryPointFieldName a).map fieldFoldByte = (deprecatedEntryPointFieldName b).map fieldFoldByte) : a = b := by
  revert same
  cases a <;> cases b <;> decide

theorem deprecatedEntryPointKeyMeaning_total (key : GoString) : ∃ choice, DeprecatedEntryPointKeyMeaning key choice := by
  classical
  by_cases known : ∃ field, key.map fieldFoldByte = (deprecatedEntryPointFieldName field).map fieldFoldByte
  · obtain ⟨field, found⟩ := known
    exact ⟨some field, found⟩
  · exact ⟨none, fun field same => known ⟨field, same⟩⟩

theorem deprecatedEntryPointKeyMeaning_unique (key : GoString) (a b : Option DeprecatedEntryPointField)
    (first : DeprecatedEntryPointKeyMeaning key a) (second : DeprecatedEntryPointKeyMeaning key b) : a = b := by
  cases a with
  | none => cases b with
    | none => rfl
    | some field => exact False.elim (first field second)
  | some field => cases b with
    | none => exact False.elim (second field first)
    | some other => exact congrArg some (deprecatedEntryPointField_fold_injective field other (first.symm.trans second))

abbrev DeprecatedEntryPointKeyValid := SchemaKeyValid DeprecatedEntryPointKeyMeaning

theorem deprecatedEntryPointKey_classify (wire : List w8) :
    (∃ item rest choice, wire = item ++ rest ∧ DeprecatedEntryPointKeyValid item choice) ∨
    (∃ reason, KeyRejected wire reason) :=
  schemaKey_classify DeprecatedEntryPointKeyMeaning deprecatedEntryPointKeyMeaning_total wire

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointField_fold_injective
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointKeyMeaning_total
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointKeyMeaning_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointKey_classify
