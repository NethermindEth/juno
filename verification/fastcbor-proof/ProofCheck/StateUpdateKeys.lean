import ProofCheck.StateUpdateValues
import ProofCheck.SchemaKeySpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def StateUpdateKeyMeaning (key : GoString) : Option StateUpdateField → Prop
  | some field => key.map fieldFoldByte = (stateUpdateFieldName field).map fieldFoldByte
  | none => ∀ field, key.map fieldFoldByte ≠ (stateUpdateFieldName field).map fieldFoldByte

/-- The four StateUpdate names remain distinct under the decoder's ASCII folding. -/
theorem stateUpdateField_fold_injective (a b : StateUpdateField)
    (same : (stateUpdateFieldName a).map fieldFoldByte = (stateUpdateFieldName b).map fieldFoldByte) : a = b := by
  revert same
  cases a <;> cases b <;> decide

theorem stateUpdateKeyMeaning_total (key : GoString) : ∃ choice, StateUpdateKeyMeaning key choice := by
  classical
  by_cases known : ∃ field, key.map fieldFoldByte = (stateUpdateFieldName field).map fieldFoldByte
  · obtain ⟨field, found⟩ := known
    exact ⟨some field, found⟩
  · exact ⟨none, fun field same => known ⟨field, same⟩⟩

theorem stateUpdateKeyMeaning_unique (key : GoString) (a b : Option StateUpdateField)
    (first : StateUpdateKeyMeaning key a) (second : StateUpdateKeyMeaning key b) : a = b := by
  cases a with
  | none => cases b with
    | none => rfl
    | some field => exact False.elim (first field second)
  | some field => cases b with
    | none => exact False.elim (second field first)
    | some other => exact congrArg some (stateUpdateField_fold_injective field other (first.symm.trans second))

abbrev StateUpdateKeyValid := SchemaKeyValid StateUpdateKeyMeaning

theorem stateUpdateKey_classify (wire : List w8) :
    (∃ item rest choice, wire = item ++ rest ∧ StateUpdateKeyValid item choice) ∨
    (∃ reason, KeyRejected wire reason) :=
  schemaKey_classify StateUpdateKeyMeaning stateUpdateKeyMeaning_total wire

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateField_fold_injective
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateKeyMeaning_total
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateKeyMeaning_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateKey_classify
