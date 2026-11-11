import ProofCheck.DeployValues
import ProofCheck.SchemaKeySpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def DeployKeyMeaning (key : GoString) : Option DeployField → Prop
  | some field => key.map fieldFoldByte = (deployFieldName field).map fieldFoldByte
  | none => ∀ field, key.map fieldFoldByte ≠ (deployFieldName field).map fieldFoldByte

/-- The six Deploy names remain distinct under the decoder's ASCII folding. -/
theorem deployField_fold_injective (a b : DeployField)
    (same : (deployFieldName a).map fieldFoldByte = (deployFieldName b).map fieldFoldByte) : a = b := by
  revert same
  cases a <;> cases b <;> decide

theorem deployKeyMeaning_total (key : GoString) : ∃ choice, DeployKeyMeaning key choice := by
  classical
  by_cases known : ∃ field, key.map fieldFoldByte = (deployFieldName field).map fieldFoldByte
  · obtain ⟨field, found⟩ := known
    exact ⟨some field, found⟩
  · exact ⟨none, fun field same => known ⟨field, same⟩⟩

theorem deployKeyMeaning_unique (key : GoString) (a b : Option DeployField)
    (first : DeployKeyMeaning key a) (second : DeployKeyMeaning key b) : a = b := by
  cases a with
  | none => cases b with
    | none => rfl
    | some field => exact False.elim (first field second)
  | some field => cases b with
    | none => exact False.elim (second field first)
    | some other => exact congrArg some (deployField_fold_injective field other (first.symm.trans second))

abbrev DeployKeyValid := SchemaKeyValid DeployKeyMeaning

theorem deployKey_classify (wire : List w8) :
    (∃ item rest choice, wire = item ++ rest ∧ DeployKeyValid item choice) ∨
    (∃ reason, KeyRejected wire reason) :=
  schemaKey_classify DeployKeyMeaning deployKeyMeaning_total wire

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.deployField_fold_injective
#print axioms Perennial.github_com.NethermindEth.juno.core.deployKeyMeaning_total
#print axioms Perennial.github_com.NethermindEth.juno.core.deployKeyMeaning_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.deployKey_classify
