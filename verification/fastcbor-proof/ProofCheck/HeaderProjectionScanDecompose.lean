import ProofCheck.HeaderProjectionScanSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- A complete valid wire-order record exposes either scanner exhaustion or
    one selected field and a smaller valid remainder. Ignored pairs may precede
    either result; selection stops before the field's value. -/
theorem headerProjectionPairs_scan (d n : Nat) (kind : HeaderProjectionKind) (seen : List Nat)
    (old final : HeaderProjectionValue) (wire : List w8)
    (valid : HeaderProjectionPairsValid d kind n seen old wire final) :
    (WantedValid d (headerProjectionNames kind) seen n wire .done ∧ final = old) ∨
    (∃ (scanned value tail : List w8) (index left : Nat) (field : HeaderProjectionField) (middle : HeaderProjectionValue),
      wire = scanned ++ value ++ tail ∧
      WantedValid d (headerProjectionNames kind) seen n scanned (.field index left) ∧
      (headerProjectionFields kind)[index]? = some field ∧
      HeaderProjectionFieldValid d field old value middle ∧
      HeaderProjectionPairsValid d kind left (index :: seen) middle tail final) := by
  induction valid with
  | nil => exact Or.inl ⟨.done, rfl⟩
  | @step n seen choice old middle final key value tail keyValid valueValid rest ih =>
    cases valueValid with
    | ignored ignored valueValid =>
      have next : headerProjectionNextSeen seen choice = seen := by simp [headerProjectionNextSeen, ignored]
      rw [next] at ih
      rcases ih with ⟨scan, same⟩ | ⟨scanned, selectedValue, remaining, index, left, field, afterValue, split, scan, lookup, selected, rest⟩
      · exact Or.inl ⟨.skipped keyValid ignored valueValid scan, same⟩
      · refine Or.inr ⟨key ++ value ++ scanned, selectedValue, remaining, index, left, field, afterValue, ?_,
          .skipped keyValid ignored valueValid scan, lookup, selected, rest⟩
        rw [split]
        simp only [List.append_assoc]
    | @selected index field value final fresh lookup selected =>
      have next : headerProjectionNextSeen seen choice = index :: seen := by simp [headerProjectionNextSeen, fresh]
      exact Or.inr ⟨key, value, tail, index, n, field, _, rfl, .selected keyValid fresh, lookup, selected, by simpa only [next] using rest⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionPairs_scan
