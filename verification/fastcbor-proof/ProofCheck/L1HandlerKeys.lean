import ProofCheck.L1HandlerValues
import ProofCheck.SchemaKeySpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def L1HandlerKeyMeaning (key : GoString) : Option L1HandlerField → Prop
  | some field => key.map fieldFoldByte = (l1HandlerFieldName field).map fieldFoldByte
  | none => ∀ field, key.map fieldFoldByte ≠ (l1HandlerFieldName field).map fieldFoldByte

/-- The six L1-handler names remain distinct under the decoder's ASCII folding. -/
theorem l1HandlerField_fold_injective (a b : L1HandlerField)
    (same : (l1HandlerFieldName a).map fieldFoldByte = (l1HandlerFieldName b).map fieldFoldByte) : a = b := by
  revert same
  cases a <;> cases b <;> decide

theorem l1HandlerKeyMeaning_total (key : GoString) : ∃ choice, L1HandlerKeyMeaning key choice := by
  classical
  by_cases known : ∃ field, key.map fieldFoldByte = (l1HandlerFieldName field).map fieldFoldByte
  · obtain ⟨field, found⟩ := known
    exact ⟨some field, found⟩
  · exact ⟨none, fun field same => known ⟨field, same⟩⟩

theorem l1HandlerKeyMeaning_unique (key : GoString) (a b : Option L1HandlerField)
    (first : L1HandlerKeyMeaning key a) (second : L1HandlerKeyMeaning key b) : a = b := by
  cases a with
  | none => cases b with
    | none => rfl
    | some field => exact False.elim (first field second)
  | some field => cases b with
    | none => exact False.elim (second field first)
    | some other => exact congrArg some (l1HandlerField_fold_injective field other (first.symm.trans second))

abbrev L1HandlerKeyValid := SchemaKeyValid L1HandlerKeyMeaning

theorem l1HandlerKey_classify (wire : List w8) :
    (∃ item rest choice, wire = item ++ rest ∧ L1HandlerKeyValid item choice) ∨
    (∃ reason, KeyRejected wire reason) :=
  schemaKey_classify L1HandlerKeyMeaning l1HandlerKeyMeaning_total wire

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerField_fold_injective
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerKeyMeaning_total
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerKeyMeaning_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerKey_classify
