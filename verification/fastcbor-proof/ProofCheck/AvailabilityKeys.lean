import ProofCheck.AvailabilityValues
import ProofCheck.SchemaKeySpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def AvailabilityKeyMeaning (key : GoString) : Option AvailabilityField → Prop
  | some field => key.map fieldFoldByte = (availabilityFieldName field).map fieldFoldByte
  | none => ∀ field, key.map fieldFoldByte ≠ (availabilityFieldName field).map fieldFoldByte

/-- The two DataAvailability names remain distinct under the decoder's ASCII folding. -/
theorem availabilityField_fold_injective (a b : AvailabilityField)
    (same : (availabilityFieldName a).map fieldFoldByte = (availabilityFieldName b).map fieldFoldByte) : a = b := by
  revert same
  cases a <;> cases b <;> decide

theorem availabilityKeyMeaning_total (key : GoString) : ∃ choice, AvailabilityKeyMeaning key choice := by
  classical
  by_cases known : ∃ field, key.map fieldFoldByte = (availabilityFieldName field).map fieldFoldByte
  · obtain ⟨field, found⟩ := known
    exact ⟨some field, found⟩
  · exact ⟨none, fun field same => known ⟨field, same⟩⟩

theorem availabilityKeyMeaning_unique (key : GoString) (a b : Option AvailabilityField)
    (first : AvailabilityKeyMeaning key a) (second : AvailabilityKeyMeaning key b) : a = b := by
  cases a with
  | none => cases b with
    | none => rfl
    | some field => exact False.elim (first field second)
  | some field => cases b with
    | none => exact False.elim (second field first)
    | some other => exact congrArg some (availabilityField_fold_injective field other (first.symm.trans second))

abbrev AvailabilityKeyValid := SchemaKeyValid AvailabilityKeyMeaning

theorem availabilityKey_classify (wire : List w8) :
    (∃ item rest choice, wire = item ++ rest ∧ AvailabilityKeyValid item choice) ∨
    (∃ reason, KeyRejected wire reason) :=
  schemaKey_classify AvailabilityKeyMeaning availabilityKeyMeaning_total wire

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilityField_fold_injective
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilityKeyMeaning_total
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilityKeyMeaning_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilityKey_classify
