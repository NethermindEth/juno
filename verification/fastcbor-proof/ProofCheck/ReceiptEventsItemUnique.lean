import ProofCheck.ReceiptEventsPairs
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
theorem receiptEventsItem_prefix_unique (d : Nat)
    (seen : List Nat) (choice : Option Nat) (old a b : ReceiptEventsValue)
    (left right restA restB : List w8)
    (first : ReceiptEventsItemValid d seen choice old left restA a)
    (second : ReceiptEventsItemValid d seen choice old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | ignored ignored valid =>
    cases second with
    | ignored ignored2 valid2 =>
      obtain ⟨rfl, rfl⟩ := skipValid_prefix_unique _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
    | selected fresh lookup valid2 => rw [ignored] at fresh; cases fresh
  | @selected index field wire suffix final fresh lookup valid =>
    cases second with
    | ignored ignored valid2 => rw [ignored] at fresh; cases fresh
    | @selected index2 field2 wire2 suffix2 final2 fresh2 lookup2 valid2 =>
      have indices := Option.some.inj (fresh.symm.trans fresh2)
      subst index2
      have fields := Option.some.inj (lookup.symm.trans lookup2)
      subst field2
      exact receiptEventsField_prefix_unique _ _ _ _ _ _ _ _ _ valid valid2 same

theorem receiptEventsItem_not_rejected (d consumed offset extraDepth : Nat)
    (seen : List Nat) (choice : Option Nat) (old final : ReceiptEventsValue)
    (item rest : List w8) (reason : ReceiptEventsFailure)
    (valid : ReceiptEventsItemValid d seen choice old item rest final)
    (failure : ReceiptEventsItemRejected d seen choice (item ++ rest) consumed offset extraDepth reason) : False := by
  cases valid with
  | ignored ignored valid =>
    cases failure with
    | ignored ignored2 failure => exact skipValid_not_failure _ _ _ _ _ valid failure
    | selected fresh lookup failure => rw [ignored] at fresh; cases fresh
  | @selected index field wire suffix final fresh lookup valid =>
    cases failure with
    | ignored ignored failure => rw [ignored] at fresh; cases fresh
    | @selected index2 field2 wire2 consumed2 offset2 depth2 reason2 fresh2 lookup2 failure =>
      have indices := Option.some.inj (fresh.symm.trans fresh2)
      subst index2
      have fields := Option.some.inj (lookup.symm.trans lookup2)
      subst field2
      exact receiptEventsField_not_rejected _ _ _ _ _ _ _ _ _ _ valid failure

theorem receiptEventsItemRejected_unique (d consumedA consumedB offsetA offsetB depthA depthB : Nat)
    (seen : List Nat) (choice : Option Nat) (wire : List w8) (a b : ReceiptEventsFailure)
    (first : ReceiptEventsItemRejected d seen choice wire consumedA offsetA depthA a)
    (second : ReceiptEventsItemRejected d seen choice wire consumedB offsetB depthB b) : consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b := by
  cases first with
  | ignored ignored failure =>
    cases second with
    | ignored ignored2 failure2 =>
      obtain ⟨rfl, rfl⟩ := skipFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
    | selected fresh lookup failure2 => rw [ignored] at fresh; cases fresh
  | @selected index field wire consumed offset extraDepth reason fresh lookup failure =>
    cases second with
    | ignored ignored failure2 => rw [ignored] at fresh; cases fresh
    | @selected index2 field2 wire2 consumed2 offset2 depth2 reason2 fresh2 lookup2 failure2 =>
      have indices := Option.some.inj (fresh.symm.trans fresh2)
      subst index2
      have fields := Option.some.inj (lookup.symm.trans lookup2)
      subst field2
      obtain ⟨rfl, rfl, rfl, rfl⟩ := receiptEventsField_rejected_unique _ _ _ _ _ _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsItem_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsItem_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsItemRejected_unique
