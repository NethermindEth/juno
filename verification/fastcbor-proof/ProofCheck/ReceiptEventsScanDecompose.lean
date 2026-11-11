import ProofCheck.ReceiptEventsScanSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- A complete valid wire-order record exposes either scanner exhaustion or
    one selected field and a smaller valid remainder. Ignored pairs may precede
    either result; selection stops before the field's value. -/
theorem receiptEventsPairs_scan (d n : Nat) (seen : List Nat)
    (old final : ReceiptEventsValue) (wire suffix : List w8)
    (valid : ReceiptEventsPairsValid d n seen old wire suffix final) :
    (WantedValid d receiptEventsNames seen n wire .done ∧ final = old) ∨
    (∃ (scanned value tail : List w8) (index left : Nat) (field : ReceiptEventsField) (middle : ReceiptEventsValue),
      wire = scanned ++ value ++ tail ∧
      WantedValid d receiptEventsNames seen n scanned (.field index left) ∧
      receiptEventsFields[index]? = some field ∧
      ReceiptEventsFieldValid d field old value (tail ++ suffix) middle ∧
      ReceiptEventsPairsValid d left (index :: seen) middle tail suffix final) := by
  induction valid with
  | nil => exact Or.inl ⟨.done, rfl⟩
  | @step n seen choice old middle final key value tail suffix keyValid valueValid rest ih =>
    cases valueValid with
    | ignored ignored valueValid =>
      have next : receiptEventsNextSeen seen choice = seen := by simp [receiptEventsNextSeen, ignored]
      rw [next] at ih
      rcases ih with ⟨scan, same⟩ | ⟨scanned, selectedValue, remaining, index, left, field, afterValue, split, scan, lookup, selected, rest⟩
      · exact Or.inl ⟨.skipped keyValid ignored valueValid scan, same⟩
      · refine Or.inr ⟨key ++ value ++ scanned, selectedValue, remaining, index, left, field, afterValue, ?_,
          .skipped keyValid ignored valueValid scan, lookup, selected, rest⟩
        rw [split]
        simp only [List.append_assoc]
    | @selected index field value suffix2 final fresh lookup selected =>
      have next : receiptEventsNextSeen seen choice = index :: seen := by simp [receiptEventsNextSeen, fresh]
      exact Or.inr ⟨key, value, tail, index, n, field, _, rfl, .selected keyValid fresh, lookup, selected, by simpa only [next] using rest⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsPairs_scan
