import ProofCheck.L1MessageValues
import ProofCheck.SchemaKeySpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def L1MessageKeyMeaning (key : GoString) : Option L1MessageField → Prop
  | some field => key.map fieldFoldByte = (l1MessageFieldName field).map fieldFoldByte
  | none => ∀ field, key.map fieldFoldByte ≠ (l1MessageFieldName field).map fieldFoldByte

/-- The five L1ToL2Message names remain distinct under the decoder's ASCII folding. -/
theorem l1MessageField_fold_injective (a b : L1MessageField)
    (same : (l1MessageFieldName a).map fieldFoldByte = (l1MessageFieldName b).map fieldFoldByte) : a = b := by
  revert same
  cases a <;> cases b <;> decide

theorem l1MessageKeyMeaning_total (key : GoString) : ∃ choice, L1MessageKeyMeaning key choice := by
  classical
  by_cases known : ∃ field, key.map fieldFoldByte = (l1MessageFieldName field).map fieldFoldByte
  · obtain ⟨field, found⟩ := known
    exact ⟨some field, found⟩
  · exact ⟨none, fun field same => known ⟨field, same⟩⟩

theorem l1MessageKeyMeaning_unique (key : GoString) (a b : Option L1MessageField)
    (first : L1MessageKeyMeaning key a) (second : L1MessageKeyMeaning key b) : a = b := by
  cases a with
  | none => cases b with
    | none => rfl
    | some field => exact False.elim (first field second)
  | some field => cases b with
    | none => exact False.elim (second field first)
    | some other => exact congrArg some (l1MessageField_fold_injective field other (first.symm.trans second))

abbrev L1MessageKeyValid := SchemaKeyValid L1MessageKeyMeaning

theorem l1MessageKey_classify (wire : List w8) :
    (∃ item rest choice, wire = item ++ rest ∧ L1MessageKeyValid item choice) ∨
    (∃ reason, KeyRejected wire reason) :=
  schemaKey_classify L1MessageKeyMeaning l1MessageKeyMeaning_total wire

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageField_fold_injective
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageKeyMeaning_total
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageKeyMeaning_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageKey_classify
