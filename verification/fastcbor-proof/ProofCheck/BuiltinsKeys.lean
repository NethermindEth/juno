import ProofCheck.BuiltinsValues
import ProofCheck.SchemaKeySpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def BuiltinsKeyMeaning (key : GoString) : Option BuiltinsField → Prop
  | some field => key.map fieldFoldByte = (builtinsFieldName field).map fieldFoldByte
  | none => ∀ field, key.map fieldFoldByte ≠ (builtinsFieldName field).map fieldFoldByte

/-- The 12 BuiltinInstanceCounter names remain distinct under the decoder's ASCII folding. -/
theorem builtinsField_fold_injective (a b : BuiltinsField)
    (same : (builtinsFieldName a).map fieldFoldByte = (builtinsFieldName b).map fieldFoldByte) : a = b := by
  revert same
  cases a <;> cases b <;> decide

theorem builtinsKeyMeaning_total (key : GoString) : ∃ choice, BuiltinsKeyMeaning key choice := by
  classical
  by_cases known : ∃ field, key.map fieldFoldByte = (builtinsFieldName field).map fieldFoldByte
  · obtain ⟨field, found⟩ := known
    exact ⟨some field, found⟩
  · exact ⟨none, fun field same => known ⟨field, same⟩⟩

theorem builtinsKeyMeaning_unique (key : GoString) (a b : Option BuiltinsField)
    (first : BuiltinsKeyMeaning key a) (second : BuiltinsKeyMeaning key b) : a = b := by
  cases a with
  | none => cases b with
    | none => rfl
    | some field => exact False.elim (first field second)
  | some field => cases b with
    | none => exact False.elim (second field first)
    | some other => exact congrArg some (builtinsField_fold_injective field other (first.symm.trans second))

abbrev BuiltinsKeyValid := SchemaKeyValid BuiltinsKeyMeaning

theorem builtinsKey_classify (wire : List w8) :
    (∃ item rest choice, wire = item ++ rest ∧ BuiltinsKeyValid item choice) ∨
    (∃ reason, KeyRejected wire reason) :=
  schemaKey_classify BuiltinsKeyMeaning builtinsKeyMeaning_total wire

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsField_fold_injective
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsKeyMeaning_total
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsKeyMeaning_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsKey_classify
