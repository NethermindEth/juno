import ProofCheck.ReceiptStatusScanSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- One batched-scan step of a rejected projection: the scan fails, its selected
    field fails, or that field succeeds and rejection occurs in the remainder.
    Consumption and diagnostic position are deliberately separate. -/
inductive ReceiptStatusRejectStep (d : Nat) :
    Nat → List Nat → ReceiptStatusValue → List w8 → ReceiptStatusValue → Nat → Nat → ReceiptStatusFailure → Prop where
  | scan {n seen old wire consumed offset left reason}
      (failure : WantedRejected d receiptStatusNames seen n wire consumed offset left reason) :
      ReceiptStatusRejectStep d n seen old wire old consumed offset (receiptStatusScanFailure reason)
  | field {n seen old scanned tail index left field offset reason}
      (scan : WantedValid d receiptStatusNames seen n scanned (.field index left))
      (lookup : receiptStatusFields[index]? = some field)
      (failure : ReceiptStatusFieldRejected field tail offset reason) :
      ReceiptStatusRejectStep d n seen old (scanned ++ tail) old
        scanned.length (scanned.length + offset) (.field reason)
  | next {n seen old middle final scanned value tail index left field consumed offset reason}
      (scan : WantedValid d receiptStatusNames seen n scanned (.field index left))
      (lookup : receiptStatusFields[index]? = some field)
      (valid : ReceiptStatusFieldValid field old value middle)
      (failure : ReceiptStatusPairsRejected d left (index :: seen) middle tail final consumed offset reason) :
      ReceiptStatusRejectStep d n seen old (scanned ++ value ++ tail) final
        (scanned.length + value.length + consumed) (scanned.length + value.length + offset) reason

/-- Prepending an ignored pair preserves published fields and selection while
    shifting both positions by the pair's full encoded length. -/
theorem receiptStatusRejectStep_ignored (d n consumed offset : Nat)
    (seen : List Nat) (choice : Option Nat) (old final : ReceiptStatusValue)
    (key value wire : List w8) (reason : ReceiptStatusFailure)
    (keyValid : WantedKeyValid receiptStatusNames key choice)
    (ignored : wantedSelect seen choice = none) (valueValid : SkipValid d value)
    (step : ReceiptStatusRejectStep d n seen old wire final consumed offset reason) :
    ReceiptStatusRejectStep d (n + 1) seen old (key ++ value ++ wire) final
      (key.length + value.length + consumed) (key.length + value.length + offset) reason := by
  cases step with
  | scan failure => exact .scan (.next keyValid ignored valueValid failure)
  | field scan lookup failure =>
    simpa only [List.append_assoc, List.length_append, Nat.add_assoc] using
      ReceiptStatusRejectStep.field (WantedValid.skipped keyValid ignored valueValid scan) lookup failure
  | next scan lookup valid failure =>
    simpa only [List.append_assoc, List.length_append, Nat.add_assoc] using
      ReceiptStatusRejectStep.next (WantedValid.skipped keyValid ignored valueValid scan) lookup valid failure

/-- Every independent per-pair rejection exposes the next batched scanner step,
    including arbitrary ignored and duplicate prefixes. -/
theorem receiptStatusPairs_rejected_scan (d n consumed offset : Nat)
    (seen : List Nat) (old final : ReceiptStatusValue) (wire : List w8) (reason : ReceiptStatusFailure)
    (failure : ReceiptStatusPairsRejected d n seen old wire final consumed offset reason) :
    ReceiptStatusRejectStep d n seen old wire final consumed offset reason := by
  induction failure with
  | key failure => exact .scan (.key failure)
  | value keyValid failure =>
    cases failure with
    | ignored ignored failure => exact .scan (.skipped keyValid ignored failure)
    | selected fresh lookup failure => exact .field (.selected keyValid fresh) lookup failure
  | @next n seen choice old middle final key value tail consumed offset reason keyValid valueValid failure ih =>
    cases valueValid with
    | ignored ignored valueValid =>
      have next : receiptStatusNextSeen seen choice = seen := by simp [receiptStatusNextSeen, ignored]
      rw [next] at ih
      exact receiptStatusRejectStep_ignored d n consumed offset seen choice old final key value tail reason
        keyValid ignored valueValid ih
    | selected fresh lookup valid =>
      apply ReceiptStatusRejectStep.next (.selected keyValid fresh) lookup valid
      simpa only [receiptStatusNextSeen, fresh] using failure

/-- The decomposition preserves the independent record relation in both
    directions; it does not weaken rejection behavior for implementation proofs. -/
theorem receiptStatusRejectStep_pairs (d n consumed offset : Nat)
    (seen : List Nat) (old final : ReceiptStatusValue) (wire : List w8) (reason : ReceiptStatusFailure)
    (step : ReceiptStatusRejectStep d n seen old wire final consumed offset reason) :
    ReceiptStatusPairsRejected d n seen old wire final consumed offset reason := by
  cases step with
  | scan failure => exact receiptStatusScan_rejected _ _ _ _ _ _ _ _ _ failure
  | field scan lookup failure => exact receiptStatusScan_field_rejected _ _ _ _ _ _ _ _ _ _ _ scan lookup failure
  | next scan lookup valid failure => exact receiptStatusScan_tail_rejected _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ scan lookup valid failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusRejectStep_ignored
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusPairs_rejected_scan
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusRejectStep_pairs
