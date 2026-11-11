import ProofCheck.CasmEntryPointValues
import ProofCheck.SchemaKeySpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def CasmEntryPointKeyMeaning (key : GoString) : Option CasmEntryPointField → Prop
  | some field => key.map fieldFoldByte = (casmEntryPointFieldName field).map fieldFoldByte
  | none => ∀ field, key.map fieldFoldByte ≠ (casmEntryPointFieldName field).map fieldFoldByte

/-- The three CasmEntryPoint names remain distinct under the decoder's ASCII folding. -/
theorem casmEntryPointField_fold_injective (a b : CasmEntryPointField)
    (same : (casmEntryPointFieldName a).map fieldFoldByte = (casmEntryPointFieldName b).map fieldFoldByte) : a = b := by
  revert same
  cases a <;> cases b <;> decide

theorem casmEntryPointKeyMeaning_total (key : GoString) : ∃ choice, CasmEntryPointKeyMeaning key choice := by
  classical
  by_cases known : ∃ field, key.map fieldFoldByte = (casmEntryPointFieldName field).map fieldFoldByte
  · obtain ⟨field, found⟩ := known
    exact ⟨some field, found⟩
  · exact ⟨none, fun field same => known ⟨field, same⟩⟩

theorem casmEntryPointKeyMeaning_unique (key : GoString) (a b : Option CasmEntryPointField)
    (first : CasmEntryPointKeyMeaning key a) (second : CasmEntryPointKeyMeaning key b) : a = b := by
  cases a with
  | none => cases b with
    | none => rfl
    | some field => exact False.elim (first field second)
  | some field => cases b with
    | none => exact False.elim (second field first)
    | some other => exact congrArg some (casmEntryPointField_fold_injective field other (first.symm.trans second))

abbrev CasmEntryPointKeyValid := SchemaKeyValid CasmEntryPointKeyMeaning

theorem casmEntryPointKey_classify (wire : List w8) :
    (∃ item rest choice, wire = item ++ rest ∧ CasmEntryPointKeyValid item choice) ∨
    (∃ reason, KeyRejected wire reason) :=
  schemaKey_classify CasmEntryPointKeyMeaning casmEntryPointKeyMeaning_total wire

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointField_fold_injective
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointKeyMeaning_total
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointKeyMeaning_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointKey_classify
