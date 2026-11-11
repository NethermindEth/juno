import ProofCheck.InvokeValues
import ProofCheck.SchemaKeySpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def InvokeKeyMeaning (key : GoString) : Option InvokeField → Prop
  | some field => key.map fieldFoldByte = (invokeFieldName field).map fieldFoldByte
  | none => ∀ field, key.map fieldFoldByte ≠ (invokeFieldName field).map fieldFoldByte

/-- The sixteen Invoke names remain distinct under the decoder's ASCII folding. -/
theorem invokeField_fold_injective (a b : InvokeField)
    (same : (invokeFieldName a).map fieldFoldByte = (invokeFieldName b).map fieldFoldByte) : a = b := by
  revert same
  cases a <;> cases b <;> decide

theorem invokeKeyMeaning_total (key : GoString) : ∃ choice, InvokeKeyMeaning key choice := by
  classical
  by_cases known : ∃ field, key.map fieldFoldByte = (invokeFieldName field).map fieldFoldByte
  · obtain ⟨field, found⟩ := known
    exact ⟨some field, found⟩
  · exact ⟨none, fun field same => known ⟨field, same⟩⟩

theorem invokeKeyMeaning_unique (key : GoString) (a b : Option InvokeField)
    (first : InvokeKeyMeaning key a) (second : InvokeKeyMeaning key b) : a = b := by
  cases a with
  | none => cases b with
    | none => rfl
    | some field => exact False.elim (first field second)
  | some field => cases b with
    | none => exact False.elim (second field first)
    | some other => exact congrArg some (invokeField_fold_injective field other (first.symm.trans second))

abbrev InvokeKeyValid := SchemaKeyValid InvokeKeyMeaning

theorem invokeKey_classify (wire : List w8) :
    (∃ item rest choice, wire = item ++ rest ∧ InvokeKeyValid item choice) ∨
    (∃ reason, KeyRejected wire reason) :=
  schemaKey_classify InvokeKeyMeaning invokeKeyMeaning_total wire

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeField_fold_injective
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeKeyMeaning_total
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeKeyMeaning_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeKey_classify
