import ProofCheck.DeployAccountValues
import ProofCheck.SchemaKeySpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def DeployAccountKeyMeaning (key : GoString) : Option DeployAccountField → Prop
  | some field => key.map fieldFoldByte = (deployAccountFieldName field).map fieldFoldByte
  | none => ∀ field, key.map fieldFoldByte ≠ (deployAccountFieldName field).map fieldFoldByte

/-- The fourteen DeployAccount names remain distinct under the decoder's ASCII folding. -/
theorem deployAccountField_fold_injective (a b : DeployAccountField)
    (same : (deployAccountFieldName a).map fieldFoldByte = (deployAccountFieldName b).map fieldFoldByte) : a = b := by
  revert same
  cases a <;> cases b <;> decide

theorem deployAccountKeyMeaning_total (key : GoString) : ∃ choice, DeployAccountKeyMeaning key choice := by
  classical
  by_cases known : ∃ field, key.map fieldFoldByte = (deployAccountFieldName field).map fieldFoldByte
  · obtain ⟨field, found⟩ := known
    exact ⟨some field, found⟩
  · exact ⟨none, fun field same => known ⟨field, same⟩⟩

theorem deployAccountKeyMeaning_unique (key : GoString) (a b : Option DeployAccountField)
    (first : DeployAccountKeyMeaning key a) (second : DeployAccountKeyMeaning key b) : a = b := by
  cases a with
  | none => cases b with
    | none => rfl
    | some field => exact False.elim (first field second)
  | some field => cases b with
    | none => exact False.elim (second field first)
    | some other => exact congrArg some (deployAccountField_fold_injective field other (first.symm.trans second))

abbrev DeployAccountKeyValid := SchemaKeyValid DeployAccountKeyMeaning

theorem deployAccountKey_classify (wire : List w8) :
    (∃ item rest choice, wire = item ++ rest ∧ DeployAccountKeyValid item choice) ∨
    (∃ reason, KeyRejected wire reason) :=
  schemaKey_classify DeployAccountKeyMeaning deployAccountKeyMeaning_total wire

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountField_fold_injective
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountKeyMeaning_total
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountKeyMeaning_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountKey_classify
