import ProofCheck.TransactionHashScanSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- One batched-scan step of a rejected projection: the scan fails, its selected
    field fails, or that field succeeds and rejection occurs in the remainder.
    Consumption and diagnostic position are deliberately separate. -/
inductive TransactionHashRejectStep (d : Nat) :
    Nat → List Nat → TransactionHashValue → List w8 → TransactionHashValue → Nat → Nat → TransactionHashFailure → Prop where
  | scan {n seen old wire consumed offset left reason}
      (failure : WantedRejected d (transactionHashNames ) seen n wire consumed offset left reason) :
      TransactionHashRejectStep d n seen old wire old consumed offset (transactionHashScanFailure reason)
  | field {n seen old scanned tail index left field offset reason}
      (scan : WantedValid d (transactionHashNames ) seen n scanned (.field index left))
      (lookup : (transactionHashFields )[index]? = some field)
      (failure : TransactionHashFieldRejected d field tail offset reason) :
      TransactionHashRejectStep d n seen old (scanned ++ tail) old
        scanned.length (scanned.length + offset) (.field reason)
  | next {n seen old middle final scanned value tail index left field consumed offset reason}
      (scan : WantedValid d (transactionHashNames ) seen n scanned (.field index left))
      (lookup : (transactionHashFields )[index]? = some field)
      (valid : TransactionHashFieldValid d field old value middle)
      (failure : TransactionHashPairsRejected d left (index :: seen) middle tail final consumed offset reason) :
      TransactionHashRejectStep d n seen old (scanned ++ value ++ tail) final
        (scanned.length + value.length + consumed) (scanned.length + value.length + offset) reason

/-- Prepending an ignored pair preserves published fields and selection while
    shifting both positions by the pair's full encoded length. -/
theorem transactionHashRejectStep_ignored (d n consumed offset : Nat) 
    (seen : List Nat) (choice : Option Nat) (old final : TransactionHashValue)
    (key value wire : List w8) (reason : TransactionHashFailure)
    (keyValid : WantedKeyValid (transactionHashNames ) key choice)
    (ignored : wantedSelect seen choice = none) (valueValid : SkipValid d value)
    (step : TransactionHashRejectStep d n seen old wire final consumed offset reason) :
    TransactionHashRejectStep d (n + 1) seen old (key ++ value ++ wire) final
      (key.length + value.length + consumed) (key.length + value.length + offset) reason := by
  cases step with
  | scan failure => exact .scan (.next keyValid ignored valueValid failure)
  | field scan lookup failure =>
    simpa only [List.append_assoc, List.length_append, Nat.add_assoc] using
      TransactionHashRejectStep.field (WantedValid.skipped keyValid ignored valueValid scan) lookup failure
  | next scan lookup valid failure =>
    simpa only [List.append_assoc, List.length_append, Nat.add_assoc] using
      TransactionHashRejectStep.next (WantedValid.skipped keyValid ignored valueValid scan) lookup valid failure

/-- Every independent per-pair rejection exposes the next batched scanner step,
    including arbitrary ignored and duplicate prefixes. -/
theorem transactionHashPairs_rejected_scan (d n consumed offset : Nat) 
    (seen : List Nat) (old final : TransactionHashValue) (wire : List w8) (reason : TransactionHashFailure)
    (failure : TransactionHashPairsRejected d n seen old wire final consumed offset reason) :
    TransactionHashRejectStep d n seen old wire final consumed offset reason := by
  induction failure with
  | key failure => exact .scan (.key failure)
  | value keyValid failure =>
    cases failure with
    | ignored ignored failure => exact .scan (.skipped keyValid ignored failure)
    | selected fresh lookup failure => exact .field (.selected keyValid fresh) lookup failure
  | @next n seen choice old middle final key value tail consumed offset reason keyValid valueValid failure ih =>
    cases valueValid with
    | ignored ignored valueValid =>
      have next : transactionHashNextSeen seen choice = seen := by simp [transactionHashNextSeen, ignored]
      rw [next] at ih
      exact transactionHashRejectStep_ignored d n consumed offset seen choice old final key value tail reason
        keyValid ignored valueValid ih
    | selected fresh lookup valid =>
      apply TransactionHashRejectStep.next (.selected keyValid fresh) lookup valid
      simpa only [transactionHashNextSeen, fresh] using failure

/-- The decomposition preserves the independent record relation in both
    directions; it does not weaken rejection behavior for implementation proofs. -/
theorem transactionHashRejectStep_pairs (d n consumed offset : Nat) 
    (seen : List Nat) (old final : TransactionHashValue) (wire : List w8) (reason : TransactionHashFailure)
    (step : TransactionHashRejectStep d n seen old wire final consumed offset reason) :
    TransactionHashPairsRejected d n seen old wire final consumed offset reason := by
  cases step with
  | scan failure => exact transactionHashScan_rejected _ _ _ _ _ _ _ _ _ failure
  | field scan lookup failure => exact transactionHashScan_field_rejected _ _ _ _ _ _ _ _ _ _ _ scan lookup failure
  | next scan lookup valid failure => exact transactionHashScan_tail_rejected _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ scan lookup valid failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashRejectStep_ignored
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashPairs_rejected_scan
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashRejectStep_pairs
