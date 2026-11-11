import ProofCheck.ReceiptEventsScanSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- One batched-scan step of a rejected projection: the scan fails, its selected
    field fails, or that field succeeds and rejection occurs in the remainder.
    Consumption and diagnostic position are deliberately separate. -/
inductive ReceiptEventsRejectStep (d : Nat) :
    Nat → List Nat → ReceiptEventsValue → List w8 → ReceiptEventsValue → Nat → Nat → Nat → ReceiptEventsFailure → Prop where
  | scan {n seen old wire consumed offset left reason}
      (failure : WantedRejected d receiptEventsNames seen n wire consumed offset left reason) :
      ReceiptEventsRejectStep d n seen old wire old consumed offset 0 (receiptEventsScanFailure reason)
  | field {n seen old scanned tail index left field consumed offset extraDepth reason}
      (scan : WantedValid d receiptEventsNames seen n scanned (.field index left))
      (lookup : receiptEventsFields[index]? = some field)
      (failure : ReceiptEventsFieldRejected d field tail consumed offset extraDepth reason) :
      ReceiptEventsRejectStep d n seen old (scanned ++ tail) old
        (scanned.length + consumed) (scanned.length + offset) extraDepth (.field reason)
  | next {n seen old middle final scanned value tail index left field consumed offset extraDepth reason}
      (scan : WantedValid d receiptEventsNames seen n scanned (.field index left))
      (lookup : receiptEventsFields[index]? = some field)
      (valid : ReceiptEventsFieldValid d field old value tail middle)
      (failure : ReceiptEventsPairsRejected d left (index :: seen) middle tail final consumed offset extraDepth reason) :
      ReceiptEventsRejectStep d n seen old (scanned ++ value ++ tail) final
        (scanned.length + value.length + consumed) (scanned.length + value.length + offset) extraDepth reason

/-- Prepending an ignored pair preserves published fields and selection while
    shifting both positions by the pair's full encoded length. -/
theorem receiptEventsRejectStep_ignored (d n consumed offset extraDepth : Nat)
    (seen : List Nat) (choice : Option Nat) (old final : ReceiptEventsValue)
    (key value wire : List w8) (reason : ReceiptEventsFailure)
    (keyValid : WantedKeyValid receiptEventsNames key choice)
    (ignored : wantedSelect seen choice = none) (valueValid : SkipValid d value)
    (step : ReceiptEventsRejectStep d n seen old wire final consumed offset extraDepth reason) :
    ReceiptEventsRejectStep d (n + 1) seen old (key ++ value ++ wire) final
      (key.length + value.length + consumed) (key.length + value.length + offset) extraDepth reason := by
  cases step with
  | scan failure => exact .scan (.next keyValid ignored valueValid failure)
  | field scan lookup failure =>
    simpa only [List.append_assoc, List.length_append, Nat.add_assoc] using
      ReceiptEventsRejectStep.field (WantedValid.skipped keyValid ignored valueValid scan) lookup failure
  | next scan lookup valid failure =>
    simpa only [List.append_assoc, List.length_append, Nat.add_assoc] using
      ReceiptEventsRejectStep.next (WantedValid.skipped keyValid ignored valueValid scan) lookup valid failure

/-- Every independent per-pair rejection exposes the next batched scanner step,
    including arbitrary ignored and duplicate prefixes. -/
theorem receiptEventsPairs_rejected_scan (d n consumed offset extraDepth : Nat)
    (seen : List Nat) (old final : ReceiptEventsValue) (wire : List w8) (reason : ReceiptEventsFailure)
    (failure : ReceiptEventsPairsRejected d n seen old wire final consumed offset extraDepth reason) :
    ReceiptEventsRejectStep d n seen old wire final consumed offset extraDepth reason := by
  induction failure with
  | key failure => exact .scan (.key failure)
  | value keyValid failure =>
    cases failure with
    | ignored ignored failure => exact .scan (.skipped keyValid ignored failure)
    | selected fresh lookup failure => exact .field (.selected keyValid fresh) lookup failure
  | @next n seen choice old middle final key value tail consumed offset extraDepth reason keyValid valueValid failure ih =>
    cases valueValid with
    | ignored ignored valueValid =>
      have next : receiptEventsNextSeen seen choice = seen := by simp [receiptEventsNextSeen, ignored]
      rw [next] at ih
      exact receiptEventsRejectStep_ignored d n consumed offset extraDepth seen choice old final key value tail reason
        keyValid ignored valueValid ih
    | selected fresh lookup valid =>
      apply ReceiptEventsRejectStep.next (.selected keyValid fresh) lookup valid
      simpa only [receiptEventsNextSeen, fresh] using failure

/-- The decomposition preserves the independent record relation in both
    directions; it does not weaken rejection behavior for implementation proofs. -/
theorem receiptEventsRejectStep_pairs (d n consumed offset extraDepth : Nat)
    (seen : List Nat) (old final : ReceiptEventsValue) (wire : List w8) (reason : ReceiptEventsFailure)
    (step : ReceiptEventsRejectStep d n seen old wire final consumed offset extraDepth reason) :
    ReceiptEventsPairsRejected d n seen old wire final consumed offset extraDepth reason := by
  cases step with
  | scan failure => exact receiptEventsScan_rejected _ _ _ _ _ _ _ _ _ failure
  | field scan lookup failure => exact receiptEventsScan_field_rejected _ _ _ _ _ _ _ _ _ _ _ _ _ scan lookup failure
  | next scan lookup valid failure => exact receiptEventsScan_tail_rejected _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ scan lookup valid failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsRejectStep_ignored
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsPairs_rejected_scan
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsRejectStep_pairs
