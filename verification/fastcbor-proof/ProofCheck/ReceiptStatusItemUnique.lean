import ProofCheck.ReceiptStatusPairs
import ProofCheck.FeltFailureUnique
import ProofCheck.WantedFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- The fixed selection policy gives one value and boundary, including ignored
    unknown keys and repeated fields. No implementation contract is assumed. -/
theorem receiptStatusItem_prefix_unique (d : Nat)
    (seen : List Nat) (choice : Option Nat) (old a b : ReceiptStatusValue)
    (left right restA restB : List w8)
    (first : ReceiptStatusItemValid d seen choice old left a)
    (second : ReceiptStatusItemValid d seen choice old right b)
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
      exact receiptStatusField_prefix_unique _ _ _ _ _ _ _ _ valid valid2 same

theorem receiptStatusItem_not_rejected (d offset : Nat)
    (seen : List Nat) (choice : Option Nat) (old final : ReceiptStatusValue)
    (item rest : List w8) (reason : ReceiptStatusFailure)
    (valid : ReceiptStatusItemValid d seen choice old item final)
    (failure : ReceiptStatusItemRejected d seen choice (item ++ rest) offset reason) : False := by
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
      exact receiptStatusField_not_rejected _ _ _ _ _ _ _ valid failure

theorem receiptStatusItemRejected_unique (d offsetA offsetB : Nat)
    (seen : List Nat) (choice : Option Nat) (wire : List w8) (a b : ReceiptStatusFailure)
    (first : ReceiptStatusItemRejected d seen choice wire offsetA a)
    (second : ReceiptStatusItemRejected d seen choice wire offsetB b) : offsetA = offsetB ∧ a = b := by
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
      obtain ⟨rfl, rfl⟩ := receiptStatusField_rejected_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusItem_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusItem_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusItemRejected_unique
