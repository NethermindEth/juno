import ProofCheck.StateDiffValues
import ProofCheck.SchemaKeySpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def StateDiffKeyMeaning (key : GoString) : Option StateDiffField → Prop
  | some field => key.map fieldFoldByte = (stateDiffFieldName field).map fieldFoldByte
  | none => ∀ field, key.map fieldFoldByte ≠ (stateDiffFieldName field).map fieldFoldByte

/-- The seven StateDiff names remain distinct under the decoder's ASCII folding. -/
theorem stateDiffField_fold_injective (a b : StateDiffField)
    (same : (stateDiffFieldName a).map fieldFoldByte = (stateDiffFieldName b).map fieldFoldByte) : a = b := by
  revert same
  cases a <;> cases b <;> decide

theorem stateDiffKeyMeaning_total (key : GoString) : ∃ choice, StateDiffKeyMeaning key choice := by
  classical
  by_cases known : ∃ field, key.map fieldFoldByte = (stateDiffFieldName field).map fieldFoldByte
  · obtain ⟨field, found⟩ := known
    exact ⟨some field, found⟩
  · exact ⟨none, fun field same => known ⟨field, same⟩⟩

theorem stateDiffKeyMeaning_unique (key : GoString) (a b : Option StateDiffField)
    (first : StateDiffKeyMeaning key a) (second : StateDiffKeyMeaning key b) : a = b := by
  cases a with
  | none => cases b with
    | none => rfl
    | some field => exact False.elim (first field second)
  | some field => cases b with
    | none => exact False.elim (second field first)
    | some other => exact congrArg some (stateDiffField_fold_injective field other (first.symm.trans second))

abbrev StateDiffKeyValid := SchemaKeyValid StateDiffKeyMeaning

theorem stateDiffKey_classify (wire : List w8) :
    (∃ item rest choice, wire = item ++ rest ∧ StateDiffKeyValid item choice) ∨
    (∃ reason, KeyRejected wire reason) :=
  schemaKey_classify StateDiffKeyMeaning stateDiffKeyMeaning_total wire

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffField_fold_injective
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffKeyMeaning_total
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffKeyMeaning_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffKey_classify
