import ProofCheck.L2MessageValues
import ProofCheck.SchemaKeySpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def L2MessageKeyMeaning (key : GoString) : Option L2MessageField → Prop
  | some field => key.map fieldFoldByte = (l2MessageFieldName field).map fieldFoldByte
  | none => ∀ field, key.map fieldFoldByte ≠ (l2MessageFieldName field).map fieldFoldByte

/-- The three L2ToL1Message names remain distinct under the decoder's ASCII folding. -/
theorem l2MessageField_fold_injective (a b : L2MessageField)
    (same : (l2MessageFieldName a).map fieldFoldByte = (l2MessageFieldName b).map fieldFoldByte) : a = b := by
  revert same
  cases a <;> cases b <;> decide

theorem l2MessageKeyMeaning_total (key : GoString) : ∃ choice, L2MessageKeyMeaning key choice := by
  classical
  by_cases known : ∃ field, key.map fieldFoldByte = (l2MessageFieldName field).map fieldFoldByte
  · obtain ⟨field, found⟩ := known
    exact ⟨some field, found⟩
  · exact ⟨none, fun field same => known ⟨field, same⟩⟩

theorem l2MessageKeyMeaning_unique (key : GoString) (a b : Option L2MessageField)
    (first : L2MessageKeyMeaning key a) (second : L2MessageKeyMeaning key b) : a = b := by
  cases a with
  | none => cases b with
    | none => rfl
    | some field => exact False.elim (first field second)
  | some field => cases b with
    | none => exact False.elim (second field first)
    | some other => exact congrArg some (l2MessageField_fold_injective field other (first.symm.trans second))

abbrev L2MessageKeyValid := SchemaKeyValid L2MessageKeyMeaning

theorem l2MessageKey_classify (wire : List w8) :
    (∃ item rest choice, wire = item ++ rest ∧ L2MessageKeyValid item choice) ∨
    (∃ reason, KeyRejected wire reason) :=
  schemaKey_classify L2MessageKeyMeaning l2MessageKeyMeaning_total wire

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessageField_fold_injective
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessageKeyMeaning_total
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessageKeyMeaning_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessageKey_classify
