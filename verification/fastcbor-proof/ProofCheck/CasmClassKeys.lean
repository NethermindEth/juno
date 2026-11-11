import ProofCheck.CasmClassValues
import ProofCheck.SchemaKeySpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def CasmClassKeyMeaning (key : GoString) : Option CasmClassField → Prop
  | some field => key.map fieldFoldByte = (casmClassFieldName field).map fieldFoldByte
  | none => ∀ field, key.map fieldFoldByte ≠ (casmClassFieldName field).map fieldFoldByte

/-- The nine CasmClass names remain distinct under the decoder's ASCII folding. -/
theorem casmClassField_fold_injective (a b : CasmClassField)
    (same : (casmClassFieldName a).map fieldFoldByte = (casmClassFieldName b).map fieldFoldByte) : a = b := by
  revert same
  cases a <;> cases b <;> decide

theorem casmClassKeyMeaning_total (key : GoString) : ∃ choice, CasmClassKeyMeaning key choice := by
  classical
  by_cases known : ∃ field, key.map fieldFoldByte = (casmClassFieldName field).map fieldFoldByte
  · obtain ⟨field, found⟩ := known
    exact ⟨some field, found⟩
  · exact ⟨none, fun field same => known ⟨field, same⟩⟩

theorem casmClassKeyMeaning_unique (key : GoString) (a b : Option CasmClassField)
    (first : CasmClassKeyMeaning key a) (second : CasmClassKeyMeaning key b) : a = b := by
  cases a with
  | none => cases b with
    | none => rfl
    | some field => exact False.elim (first field second)
  | some field => cases b with
    | none => exact False.elim (second field first)
    | some other => exact congrArg some (casmClassField_fold_injective field other (first.symm.trans second))

abbrev CasmClassKeyValid := SchemaKeyValid CasmClassKeyMeaning

theorem casmClassKey_classify (wire : List w8) :
    (∃ item rest choice, wire = item ++ rest ∧ CasmClassKeyValid item choice) ∨
    (∃ reason, KeyRejected wire reason) :=
  schemaKey_classify CasmClassKeyMeaning casmClassKeyMeaning_total wire

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassField_fold_injective
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassKeyMeaning_total
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassKeyMeaning_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassKey_classify
