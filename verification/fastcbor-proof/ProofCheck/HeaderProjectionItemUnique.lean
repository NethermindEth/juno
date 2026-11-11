import ProofCheck.HeaderProjectionPairs
import ProofCheck.FeltFailureUnique
import ProofCheck.WantedFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

theorem headerProjectionField_not_rejected (d offset : Nat) (field : HeaderProjectionField)
    (old final : HeaderProjectionValue) (item rest : List w8) (reason : HeaderProjectionFieldFailure)
    (valid : HeaderProjectionFieldValid d field old item final)
    (failure : HeaderProjectionFieldRejected d field (item ++ rest) offset reason) : False := by
  cases valid with
  | hash valid => cases failure with
    | hash failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure
  | root valid => cases failure with
    | root failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure
  | count valid => cases failure with
    | count failure => exact scalarValid_not_failure _ _ _ _ _ valid failure
  | timestamp valid => cases failure with
    | timestamp failure => exact uint64PointerValid_not_failure _ _ _ _ valid failure

theorem headerProjectionFieldRejected_unique (d offsetA offsetB : Nat) (field : HeaderProjectionField)
    (wire : List w8) (a b : HeaderProjectionFieldFailure)
    (first : HeaderProjectionFieldRejected d field wire offsetA a)
    (second : HeaderProjectionFieldRejected d field wire offsetB b) : offsetA = offsetB ∧ a = b := by
  cases first with
  | hash first => cases second with
    | hash second =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ first second
      exact ⟨rfl, rfl⟩
  | root first => cases second with
    | root second =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ first second
      exact ⟨rfl, rfl⟩
  | count first => cases second with
    | count second =>
      have eq := expectedFailure_reason_unique _ _ _ _ first.2.2 second.2.2
      exact ⟨rfl, congrArg HeaderProjectionFieldFailure.scalar eq⟩
  | timestamp first => cases second with
    | timestamp second =>
      have eq := expectedFailure_reason_unique _ _ _ _ first.2.2 second.2.2
      exact ⟨rfl, congrArg HeaderProjectionFieldFailure.scalar eq⟩

/-- The fixed selection policy gives one value and boundary, including ignored
    unknown keys and repeated fields. No implementation contract is assumed. -/
theorem headerProjectionItem_prefix_unique (d : Nat) (kind : HeaderProjectionKind)
    (seen : List Nat) (choice : Option Nat) (old a b : HeaderProjectionValue)
    (left right restA restB : List w8)
    (first : HeaderProjectionItemValid d kind seen choice old left a)
    (second : HeaderProjectionItemValid d kind seen choice old right b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | ignored ignored valid =>
    cases second with
    | ignored ignored2 valid2 =>
      obtain ⟨rfl, rfl⟩ := skipValid_prefix_unique _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
    | selected fresh lookup valid2 => rw [ignored] at fresh; cases fresh
  | @selected index field wire final fresh lookup valid =>
    cases second with
    | ignored ignored valid2 => rw [ignored] at fresh; cases fresh
    | @selected index2 field2 wire2 final2 fresh2 lookup2 valid2 =>
      have indices := Option.some.inj (fresh.symm.trans fresh2)
      subst index2
      have fields := Option.some.inj (lookup.symm.trans lookup2)
      subst field2
      exact headerProjectionField_prefix_unique _ _ _ _ _ _ _ _ _ valid valid2 same

theorem headerProjectionItem_not_rejected (d offset : Nat) (kind : HeaderProjectionKind)
    (seen : List Nat) (choice : Option Nat) (old final : HeaderProjectionValue)
    (item rest : List w8) (reason : HeaderProjectionFailure)
    (valid : HeaderProjectionItemValid d kind seen choice old item final)
    (failure : HeaderProjectionItemRejected d kind seen choice (item ++ rest) offset reason) : False := by
  cases valid with
  | ignored ignored valid =>
    cases failure with
    | ignored ignored2 failure => exact skipValid_not_failure _ _ _ _ _ valid failure
    | selected fresh lookup failure => rw [ignored] at fresh; cases fresh
  | @selected index field wire final fresh lookup valid =>
    cases failure with
    | ignored ignored failure => rw [ignored] at fresh; cases fresh
    | @selected index2 field2 wire2 offset2 reason2 fresh2 lookup2 failure =>
      have indices := Option.some.inj (fresh.symm.trans fresh2)
      subst index2
      have fields := Option.some.inj (lookup.symm.trans lookup2)
      subst field2
      exact headerProjectionField_not_rejected _ _ _ _ _ _ _ _ valid failure

theorem headerProjectionItemRejected_unique (d offsetA offsetB : Nat) (kind : HeaderProjectionKind)
    (seen : List Nat) (choice : Option Nat) (wire : List w8) (a b : HeaderProjectionFailure)
    (first : HeaderProjectionItemRejected d kind seen choice wire offsetA a)
    (second : HeaderProjectionItemRejected d kind seen choice wire offsetB b) : offsetA = offsetB ∧ a = b := by
  cases first with
  | ignored ignored failure =>
    cases second with
    | ignored ignored2 failure2 =>
      obtain ⟨rfl, rfl⟩ := skipFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl⟩
    | selected fresh lookup failure2 => rw [ignored] at fresh; cases fresh
  | @selected index field wire offset reason fresh lookup failure =>
    cases second with
    | ignored ignored failure2 => rw [ignored] at fresh; cases fresh
    | @selected index2 field2 wire2 offset2 reason2 fresh2 lookup2 failure2 =>
      have indices := Option.some.inj (fresh.symm.trans fresh2)
      subst index2
      have fields := Option.some.inj (lookup.symm.trans lookup2)
      subst field2
      obtain ⟨rfl, rfl⟩ := headerProjectionFieldRejected_unique _ _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionField_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionFieldRejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionItem_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionItem_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionItemRejected_unique
