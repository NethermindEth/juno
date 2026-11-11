import ProofCheck.TransactionHashScanSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- A complete valid wire-order record exposes either scanner exhaustion or
    one selected field and a smaller valid remainder. Ignored pairs may precede
    either result; selection stops before the field's value. -/
theorem transactionHashPairs_scan (d n : Nat) (seen : List Nat)
    (old final : TransactionHashValue) (wire : List w8)
    (valid : TransactionHashPairsValid d n seen old wire final) :
    (WantedValid d (transactionHashNames ) seen n wire .done ∧ final = old) ∨
    (∃ (scanned value tail : List w8) (index left : Nat) (field : TransactionHashField) (middle : TransactionHashValue),
      wire = scanned ++ value ++ tail ∧
      WantedValid d (transactionHashNames ) seen n scanned (.field index left) ∧
      (transactionHashFields )[index]? = some field ∧
      TransactionHashFieldValid d field old value middle ∧
      TransactionHashPairsValid d left (index :: seen) middle tail final) := by
  induction valid with
  | nil => exact Or.inl ⟨.done, rfl⟩
  | @step n seen choice old middle final key value tail keyValid valueValid rest ih =>
    cases valueValid with
    | ignored ignored valueValid =>
      have next : transactionHashNextSeen seen choice = seen := by simp [transactionHashNextSeen, ignored]
      rw [next] at ih
      rcases ih with ⟨scan, same⟩ | ⟨scanned, selectedValue, remaining, index, left, field, afterValue, split, scan, lookup, selected, rest⟩
      · exact Or.inl ⟨.skipped keyValid ignored valueValid scan, same⟩
      · refine Or.inr ⟨key ++ value ++ scanned, selectedValue, remaining, index, left, field, afterValue, ?_,
          .skipped keyValid ignored valueValid scan, lookup, selected, rest⟩
        rw [split]
        simp only [List.append_assoc]
    | @selected index field value final fresh lookup selected =>
      have next : transactionHashNextSeen seen choice = index :: seen := by simp [transactionHashNextSeen, fresh]
      exact Or.inr ⟨key, value, tail, index, n, field, _, rfl, .selected keyValid fresh, lookup, selected, by simpa only [next] using rest⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashPairs_scan
