import ProofCheck.SegmentValues
import ProofCheck.SchemaKeySpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def SegmentKeyMeaning (key : GoString) : Option SegmentField → Prop
  | some field => key.map fieldFoldByte = (segmentFieldName field).map fieldFoldByte
  | none => ∀ field, key.map fieldFoldByte ≠ (segmentFieldName field).map fieldFoldByte

/-- The two Segment names remain distinct under the decoder's ASCII folding. -/
theorem segmentField_fold_injective (a b : SegmentField)
    (same : (segmentFieldName a).map fieldFoldByte = (segmentFieldName b).map fieldFoldByte) : a = b := by
  revert same
  cases a <;> cases b <;> decide

theorem segmentKeyMeaning_total (key : GoString) : ∃ choice, SegmentKeyMeaning key choice := by
  classical
  by_cases known : ∃ field, key.map fieldFoldByte = (segmentFieldName field).map fieldFoldByte
  · obtain ⟨field, found⟩ := known
    exact ⟨some field, found⟩
  · exact ⟨none, fun field same => known ⟨field, same⟩⟩

theorem segmentKeyMeaning_unique (key : GoString) (a b : Option SegmentField)
    (first : SegmentKeyMeaning key a) (second : SegmentKeyMeaning key b) : a = b := by
  cases a with
  | none => cases b with
    | none => rfl
    | some field => exact False.elim (first field second)
  | some field => cases b with
    | none => exact False.elim (second field first)
    | some other => exact congrArg some (segmentField_fold_injective field other (first.symm.trans second))

abbrev SegmentKeyValid := SchemaKeyValid SegmentKeyMeaning

theorem segmentKey_classify (wire : List w8) :
    (∃ item rest choice, wire = item ++ rest ∧ SegmentKeyValid item choice) ∨
    (∃ reason, KeyRejected wire reason) :=
  schemaKey_classify SegmentKeyMeaning segmentKeyMeaning_total wire

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentField_fold_injective
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentKeyMeaning_total
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentKeyMeaning_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentKey_classify
