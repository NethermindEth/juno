import ProofCheck.EventValues
import ProofCheck.SchemaKeySpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def EventKeyMeaning (key : GoString) : Option EventField → Prop
  | some field => key.map fieldFoldByte = (eventFieldName field).map fieldFoldByte
  | none => ∀ field, key.map fieldFoldByte ≠ (eventFieldName field).map fieldFoldByte

/-- The three Event names remain distinct under the decoder's ASCII folding. -/
theorem eventField_fold_injective (a b : EventField)
    (same : (eventFieldName a).map fieldFoldByte = (eventFieldName b).map fieldFoldByte) : a = b := by
  revert same
  cases a <;> cases b <;> decide

theorem eventKeyMeaning_total (key : GoString) : ∃ choice, EventKeyMeaning key choice := by
  classical
  by_cases known : ∃ field, key.map fieldFoldByte = (eventFieldName field).map fieldFoldByte
  · obtain ⟨field, found⟩ := known
    exact ⟨some field, found⟩
  · exact ⟨none, fun field same => known ⟨field, same⟩⟩

theorem eventKeyMeaning_unique (key : GoString) (a b : Option EventField)
    (first : EventKeyMeaning key a) (second : EventKeyMeaning key b) : a = b := by
  cases a with
  | none => cases b with
    | none => rfl
    | some field => exact False.elim (first field second)
  | some field => cases b with
    | none => exact False.elim (second field first)
    | some other => exact congrArg some (eventField_fold_injective field other (first.symm.trans second))

abbrev EventKeyValid := SchemaKeyValid EventKeyMeaning

theorem eventKey_classify (wire : List w8) :
    (∃ item rest choice, wire = item ++ rest ∧ EventKeyValid item choice) ∨
    (∃ reason, KeyRejected wire reason) :=
  schemaKey_classify EventKeyMeaning eventKeyMeaning_total wire

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.eventField_fold_injective
#print axioms Perennial.github_com.NethermindEth.juno.core.eventKeyMeaning_total
#print axioms Perennial.github_com.NethermindEth.juno.core.eventKeyMeaning_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.eventKey_classify
