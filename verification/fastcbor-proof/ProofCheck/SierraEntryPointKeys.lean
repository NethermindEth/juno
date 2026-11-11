import ProofCheck.SierraEntryPointValues
import ProofCheck.SchemaKeySpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def SierraEntryPointKeyMeaning (key : GoString) : Option SierraEntryPointField → Prop
  | some field => key.map fieldFoldByte = (sierraEntryPointFieldName field).map fieldFoldByte
  | none => ∀ field, key.map fieldFoldByte ≠ (sierraEntryPointFieldName field).map fieldFoldByte

/-- The two SierraEntryPoint names remain distinct under the decoder's ASCII folding. -/
theorem sierraEntryPointField_fold_injective (a b : SierraEntryPointField)
    (same : (sierraEntryPointFieldName a).map fieldFoldByte = (sierraEntryPointFieldName b).map fieldFoldByte) : a = b := by
  revert same
  cases a <;> cases b <;> decide

theorem sierraEntryPointKeyMeaning_total (key : GoString) : ∃ choice, SierraEntryPointKeyMeaning key choice := by
  classical
  by_cases known : ∃ field, key.map fieldFoldByte = (sierraEntryPointFieldName field).map fieldFoldByte
  · obtain ⟨field, found⟩ := known
    exact ⟨some field, found⟩
  · exact ⟨none, fun field same => known ⟨field, same⟩⟩

theorem sierraEntryPointKeyMeaning_unique (key : GoString) (a b : Option SierraEntryPointField)
    (first : SierraEntryPointKeyMeaning key a) (second : SierraEntryPointKeyMeaning key b) : a = b := by
  cases a with
  | none => cases b with
    | none => rfl
    | some field => exact False.elim (first field second)
  | some field => cases b with
    | none => exact False.elim (second field first)
    | some other => exact congrArg some (sierraEntryPointField_fold_injective field other (first.symm.trans second))

abbrev SierraEntryPointKeyValid := SchemaKeyValid SierraEntryPointKeyMeaning

theorem sierraEntryPointKey_classify (wire : List w8) :
    (∃ item rest choice, wire = item ++ rest ∧ SierraEntryPointKeyValid item choice) ∨
    (∃ reason, KeyRejected wire reason) :=
  schemaKey_classify SierraEntryPointKeyMeaning sierraEntryPointKeyMeaning_total wire

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryPointField_fold_injective
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryPointKeyMeaning_total
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryPointKeyMeaning_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryPointKey_classify
