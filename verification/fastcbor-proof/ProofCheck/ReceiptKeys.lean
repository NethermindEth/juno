import ProofCheck.ReceiptValues
import ProofCheck.SchemaKeySpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def ReceiptKeyMeaning (key : GoString) : Option ReceiptField → Prop
  | some field => key.map fieldFoldByte = (receiptFieldName field).map fieldFoldByte
  | none => ∀ field, key.map fieldFoldByte ≠ (receiptFieldName field).map fieldFoldByte

/-- The five TransactionReceipt names remain distinct under the decoder's ASCII folding. -/
theorem receiptField_fold_injective (a b : ReceiptField)
    (same : (receiptFieldName a).map fieldFoldByte = (receiptFieldName b).map fieldFoldByte) : a = b := by
  revert same
  cases a <;> cases b <;> decide

theorem receiptKeyMeaning_total (key : GoString) : ∃ choice, ReceiptKeyMeaning key choice := by
  classical
  by_cases known : ∃ field, key.map fieldFoldByte = (receiptFieldName field).map fieldFoldByte
  · obtain ⟨field, found⟩ := known
    exact ⟨some field, found⟩
  · exact ⟨none, fun field same => known ⟨field, same⟩⟩

theorem receiptKeyMeaning_unique (key : GoString) (a b : Option ReceiptField)
    (first : ReceiptKeyMeaning key a) (second : ReceiptKeyMeaning key b) : a = b := by
  cases a with
  | none => cases b with
    | none => rfl
    | some field => exact False.elim (first field second)
  | some field => cases b with
    | none => exact False.elim (second field first)
    | some other => exact congrArg some (receiptField_fold_injective field other (first.symm.trans second))

abbrev ReceiptKeyValid := SchemaKeyValid ReceiptKeyMeaning

theorem receiptKey_classify (wire : List w8) :
    (∃ item rest choice, wire = item ++ rest ∧ ReceiptKeyValid item choice) ∨
    (∃ reason, KeyRejected wire reason) :=
  schemaKey_classify ReceiptKeyMeaning receiptKeyMeaning_total wire

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptField_fold_injective
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptKeyMeaning_total
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptKeyMeaning_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptKey_classify
