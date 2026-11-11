import ProofCheck.ExecutionValues
import ProofCheck.SchemaKeySpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def ExecutionKeyMeaning (key : GoString) : Option ExecutionField → Prop
  | some field => key.map fieldFoldByte = (executionFieldName field).map fieldFoldByte
  | none => ∀ field, key.map fieldFoldByte ≠ (executionFieldName field).map fieldFoldByte

/-- The five ExecutionResources names remain distinct under the decoder's ASCII folding. -/
theorem executionField_fold_injective (a b : ExecutionField)
    (same : (executionFieldName a).map fieldFoldByte = (executionFieldName b).map fieldFoldByte) : a = b := by
  revert same
  cases a <;> cases b <;> decide

theorem executionKeyMeaning_total (key : GoString) : ∃ choice, ExecutionKeyMeaning key choice := by
  classical
  by_cases known : ∃ field, key.map fieldFoldByte = (executionFieldName field).map fieldFoldByte
  · obtain ⟨field, found⟩ := known
    exact ⟨some field, found⟩
  · exact ⟨none, fun field same => known ⟨field, same⟩⟩

theorem executionKeyMeaning_unique (key : GoString) (a b : Option ExecutionField)
    (first : ExecutionKeyMeaning key a) (second : ExecutionKeyMeaning key b) : a = b := by
  cases a with
  | none => cases b with
    | none => rfl
    | some field => exact False.elim (first field second)
  | some field => cases b with
    | none => exact False.elim (second field first)
    | some other => exact congrArg some (executionField_fold_injective field other (first.symm.trans second))

abbrev ExecutionKeyValid := SchemaKeyValid ExecutionKeyMeaning

theorem executionKey_classify (wire : List w8) :
    (∃ item rest choice, wire = item ++ rest ∧ ExecutionKeyValid item choice) ∨
    (∃ reason, KeyRejected wire reason) :=
  schemaKey_classify ExecutionKeyMeaning executionKeyMeaning_total wire

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.executionField_fold_injective
#print axioms Perennial.github_com.NethermindEth.juno.core.executionKeyMeaning_total
#print axioms Perennial.github_com.NethermindEth.juno.core.executionKeyMeaning_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.executionKey_classify
