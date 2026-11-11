import ProofCheck.DeclareValues
import ProofCheck.SchemaKeySpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def DeclareKeyMeaning (key : GoString) : Option DeclareField → Prop
  | some field => key.map fieldFoldByte = (declareFieldName field).map fieldFoldByte
  | none => ∀ field, key.map fieldFoldByte ≠ (declareFieldName field).map fieldFoldByte

/-- The fourteen Declare names remain distinct under the decoder's ASCII folding. -/
theorem declareField_fold_injective (a b : DeclareField)
    (same : (declareFieldName a).map fieldFoldByte = (declareFieldName b).map fieldFoldByte) : a = b := by
  revert same
  cases a <;> cases b <;> decide

theorem declareKeyMeaning_total (key : GoString) : ∃ choice, DeclareKeyMeaning key choice := by
  classical
  by_cases known : ∃ field, key.map fieldFoldByte = (declareFieldName field).map fieldFoldByte
  · obtain ⟨field, found⟩ := known
    exact ⟨some field, found⟩
  · exact ⟨none, fun field same => known ⟨field, same⟩⟩

theorem declareKeyMeaning_unique (key : GoString) (a b : Option DeclareField)
    (first : DeclareKeyMeaning key a) (second : DeclareKeyMeaning key b) : a = b := by
  cases a with
  | none => cases b with
    | none => rfl
    | some field => exact False.elim (first field second)
  | some field => cases b with
    | none => exact False.elim (second field first)
    | some other => exact congrArg some (declareField_fold_injective field other (first.symm.trans second))

abbrev DeclareKeyValid := SchemaKeyValid DeclareKeyMeaning

theorem declareKey_classify (wire : List w8) :
    (∃ item rest choice, wire = item ++ rest ∧ DeclareKeyValid item choice) ∨
    (∃ reason, KeyRejected wire reason) :=
  schemaKey_classify DeclareKeyMeaning declareKeyMeaning_total wire

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.declareField_fold_injective
#print axioms Perennial.github_com.NethermindEth.juno.core.declareKeyMeaning_total
#print axioms Perennial.github_com.NethermindEth.juno.core.declareKeyMeaning_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.declareKey_classify
