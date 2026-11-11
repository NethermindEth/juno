import ProofCheck.GasUsedValues
import ProofCheck.SchemaKeySpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def GasUsedKeyMeaning (key : GoString) : Option GasUsedField → Prop
  | some field => key.map fieldFoldByte = (gasUsedFieldName field).map fieldFoldByte
  | none => ∀ field, key.map fieldFoldByte ≠ (gasUsedFieldName field).map fieldFoldByte

/-- The 3 GasConsumed names remain distinct under the decoder's ASCII folding. -/
theorem gasUsedField_fold_injective (a b : GasUsedField)
    (same : (gasUsedFieldName a).map fieldFoldByte = (gasUsedFieldName b).map fieldFoldByte) : a = b := by
  revert same
  cases a <;> cases b <;> decide

theorem gasUsedKeyMeaning_total (key : GoString) : ∃ choice, GasUsedKeyMeaning key choice := by
  classical
  by_cases known : ∃ field, key.map fieldFoldByte = (gasUsedFieldName field).map fieldFoldByte
  · obtain ⟨field, found⟩ := known
    exact ⟨some field, found⟩
  · exact ⟨none, fun field same => known ⟨field, same⟩⟩

theorem gasUsedKeyMeaning_unique (key : GoString) (a b : Option GasUsedField)
    (first : GasUsedKeyMeaning key a) (second : GasUsedKeyMeaning key b) : a = b := by
  cases a with
  | none => cases b with
    | none => rfl
    | some field => exact False.elim (first field second)
  | some field => cases b with
    | none => exact False.elim (second field first)
    | some other => exact congrArg some (gasUsedField_fold_injective field other (first.symm.trans second))

abbrev GasUsedKeyValid := SchemaKeyValid GasUsedKeyMeaning

theorem gasUsedKey_classify (wire : List w8) :
    (∃ item rest choice, wire = item ++ rest ∧ GasUsedKeyValid item choice) ∨
    (∃ reason, KeyRejected wire reason) :=
  schemaKey_classify GasUsedKeyMeaning gasUsedKeyMeaning_total wire

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedField_fold_injective
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedKeyMeaning_total
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedKeyMeaning_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedKey_classify
