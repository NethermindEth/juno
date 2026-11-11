import ProofCheck.SierraEntriesValues
import ProofCheck.SchemaKeySpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def SierraEntriesKeyMeaning (key : GoString) : Option SierraEntriesField → Prop
  | some field => key.map fieldFoldByte = (sierraEntriesFieldName field).map fieldFoldByte
  | none => ∀ field, key.map fieldFoldByte ≠ (sierraEntriesFieldName field).map fieldFoldByte

/-- The three SierraEntries names remain distinct under the decoder's ASCII folding. -/
theorem sierraEntriesField_fold_injective (a b : SierraEntriesField)
    (same : (sierraEntriesFieldName a).map fieldFoldByte = (sierraEntriesFieldName b).map fieldFoldByte) : a = b := by
  revert same
  cases a <;> cases b <;> decide

theorem sierraEntriesKeyMeaning_total (key : GoString) : ∃ choice, SierraEntriesKeyMeaning key choice := by
  classical
  by_cases known : ∃ field, key.map fieldFoldByte = (sierraEntriesFieldName field).map fieldFoldByte
  · obtain ⟨field, found⟩ := known
    exact ⟨some field, found⟩
  · exact ⟨none, fun field same => known ⟨field, same⟩⟩

theorem sierraEntriesKeyMeaning_unique (key : GoString) (a b : Option SierraEntriesField)
    (first : SierraEntriesKeyMeaning key a) (second : SierraEntriesKeyMeaning key b) : a = b := by
  cases a with
  | none => cases b with
    | none => rfl
    | some field => exact False.elim (first field second)
  | some field => cases b with
    | none => exact False.elim (second field first)
    | some other => exact congrArg some (sierraEntriesField_fold_injective field other (first.symm.trans second))

abbrev SierraEntriesKeyValid := SchemaKeyValid SierraEntriesKeyMeaning

theorem sierraEntriesKey_classify (wire : List w8) :
    (∃ item rest choice, wire = item ++ rest ∧ SierraEntriesKeyValid item choice) ∨
    (∃ reason, KeyRejected wire reason) :=
  schemaKey_classify SierraEntriesKeyMeaning sierraEntriesKeyMeaning_total wire

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntriesField_fold_injective
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntriesKeyMeaning_total
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntriesKeyMeaning_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntriesKey_classify
