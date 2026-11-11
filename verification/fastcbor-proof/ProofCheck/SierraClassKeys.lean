import ProofCheck.SierraClassValues
import ProofCheck.SchemaKeySpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def SierraClassKeyMeaning (key : GoString) : Option SierraClassField → Prop
  | some field => key.map fieldFoldByte = (sierraClassFieldName field).map fieldFoldByte
  | none => ∀ field, key.map fieldFoldByte ≠ (sierraClassFieldName field).map fieldFoldByte

/-- The seven SierraClass names remain distinct under the decoder's ASCII folding. -/
theorem sierraClassField_fold_injective (a b : SierraClassField)
    (same : (sierraClassFieldName a).map fieldFoldByte = (sierraClassFieldName b).map fieldFoldByte) : a = b := by
  revert same
  cases a <;> cases b <;> decide

theorem sierraClassKeyMeaning_total (key : GoString) : ∃ choice, SierraClassKeyMeaning key choice := by
  classical
  by_cases known : ∃ field, key.map fieldFoldByte = (sierraClassFieldName field).map fieldFoldByte
  · obtain ⟨field, found⟩ := known
    exact ⟨some field, found⟩
  · exact ⟨none, fun field same => known ⟨field, same⟩⟩

theorem sierraClassKeyMeaning_unique (key : GoString) (a b : Option SierraClassField)
    (first : SierraClassKeyMeaning key a) (second : SierraClassKeyMeaning key b) : a = b := by
  cases a with
  | none => cases b with
    | none => rfl
    | some field => exact False.elim (first field second)
  | some field => cases b with
    | none => exact False.elim (second field first)
    | some other => exact congrArg some (sierraClassField_fold_injective field other (first.symm.trans second))

abbrev SierraClassKeyValid := SchemaKeyValid SierraClassKeyMeaning

theorem sierraClassKey_classify (wire : List w8) :
    (∃ item rest choice, wire = item ++ rest ∧ SierraClassKeyValid item choice) ∨
    (∃ reason, KeyRejected wire reason) :=
  schemaKey_classify SierraClassKeyMeaning sierraClassKeyMeaning_total wire

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassField_fold_injective
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassKeyMeaning_total
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassKeyMeaning_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassKey_classify
