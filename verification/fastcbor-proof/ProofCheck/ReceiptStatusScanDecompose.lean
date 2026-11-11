import ProofCheck.ReceiptStatusScanSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- A complete valid wire-order record exposes either scanner exhaustion or
    one selected field and a smaller valid remainder. Ignored pairs may precede
    either result; selection stops before the field's value. -/
theorem receiptStatusPairs_scan (d n : Nat) (seen : List Nat)
    (old final : ReceiptStatusValue) (wire : List w8)
    (valid : ReceiptStatusPairsValid d n seen old wire final) :
    (WantedValid d receiptStatusNames seen n wire .done ∧ final = old) ∨
    (∃ (scanned value tail : List w8) (index left : Nat) (field : ReceiptStatusField) (middle : ReceiptStatusValue),
      wire = scanned ++ value ++ tail ∧
      WantedValid d receiptStatusNames seen n scanned (.field index left) ∧
      receiptStatusFields[index]? = some field ∧
      ReceiptStatusFieldValid field old value middle ∧
      ReceiptStatusPairsValid d left (index :: seen) middle tail final) := by
  induction valid with
  | nil => exact Or.inl ⟨.done, rfl⟩
  | @step n seen choice old middle final key value tail keyValid valueValid rest ih =>
    cases valueValid with
    | ignored ignored valueValid =>
      have next : receiptStatusNextSeen seen choice = seen := by simp [receiptStatusNextSeen, ignored]
      rw [next] at ih
      rcases ih with ⟨scan, same⟩ | ⟨scanned, selectedValue, remaining, index, left, field, afterValue, split, scan, lookup, selected, rest⟩
      · exact Or.inl ⟨.skipped keyValid ignored valueValid scan, same⟩
      · refine Or.inr ⟨key ++ value ++ scanned, selectedValue, remaining, index, left, field, afterValue, ?_,
          .skipped keyValid ignored valueValid scan, lookup, selected, rest⟩
        rw [split]
        simp only [List.append_assoc]
    | @selected index field value final fresh lookup selected =>
      have next : receiptStatusNextSeen seen choice = index :: seen := by simp [receiptStatusNextSeen, fresh]
      exact Or.inr ⟨key, value, tail, index, n, field, _, rfl, .selected keyValid fresh, lookup, selected, by simpa only [next] using rest⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusPairs_scan
