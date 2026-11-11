import ProofCheck.ReceiptEventsPairs

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def receiptEventsScanFailure : WantedFailureReason → ReceiptEventsFailure
  | .key reason => .key reason
  | .skipped reason => .ignored reason

/-- An exhausted batched scan is exactly a sequence of ignored pairs and
    therefore leaves the projection value unchanged. -/
theorem receiptEventsScan_done (d n : Nat)
    (seen : List Nat) (old : ReceiptEventsValue) (wire suffix : List w8)
    (scan : WantedValid d receiptEventsNames seen n wire .done) :
    ReceiptEventsPairsValid d n seen old wire suffix old := by
  induction n generalizing wire with
  | zero => cases scan; exact .nil seen old suffix
  | succ n ih =>
    cases scan with
    | skipped keyValid ignored valueValid rest =>
      apply ReceiptEventsPairsValid.step keyValid (.ignored ignored valueValid)
      simpa only [receiptEventsNextSeen, ignored] using ih _ rest

/-- A selected key, its successfully decoded value and the remaining pair
    interpretation compose through any ignored prefix consumed by NextWanted. -/
theorem receiptEventsScan_selected (d n index left : Nat)
    (seen : List Nat) (field : ReceiptEventsField) (old middle final : ReceiptEventsValue)
    (scanned value tail suffix : List w8)
    (scan : WantedValid d receiptEventsNames seen n scanned (.field index left))
    (lookup : receiptEventsFields[index]? = some field)
    (valid : ReceiptEventsFieldValid d field old value (tail ++ suffix) middle)
    (rest : ReceiptEventsPairsValid d left (index :: seen) middle tail suffix final) :
    ReceiptEventsPairsValid d n seen old (scanned ++ value ++ tail) suffix final := by
  induction n generalizing scanned with
  | zero => cases scan
  | succ n ih =>
    cases scan with
    | selected keyValid fresh =>
      apply ReceiptEventsPairsValid.step keyValid (.selected fresh lookup valid)
      simpa only [receiptEventsNextSeen, fresh] using rest
    | @skipped n2 key ignoredValue items choice result keyValid ignored valueValid scanRest =>
      have remaining := ih _ scanRest
      have pairs := ReceiptEventsPairsValid.step keyValid (.ignored ignored valueValid)
        (by simpa only [receiptEventsNextSeen, ignored] using remaining)
      simpa only [List.append_assoc] using pairs

/-- Key and ignored-value scan failures retain all existing projection fields;
    their consumption and diagnostic offset agree with the per-pair policy. -/
theorem receiptEventsScan_rejected (d n consumed offset left : Nat)
    (seen : List Nat) (old : ReceiptEventsValue) (wire : List w8) (reason : WantedFailureReason)
    (scan : WantedRejected d receiptEventsNames seen n wire consumed offset left reason) :
    ReceiptEventsPairsRejected d n seen old wire old consumed offset 0 (receiptEventsScanFailure reason) := by
  induction scan with
  | key failure => exact .key failure
  | skipped keyValid ignored failure => exact .value keyValid (.ignored ignored failure)
  | next keyValid ignored valueValid failure ih =>
    apply ReceiptEventsPairsRejected.next keyValid (.ignored ignored valueValid)
    simpa only [receiptEventsNextSeen, ignored] using ih

/-- A failed selected-field read retains its specified consumption and depth;
    prior ignored pairs do not change the retained projection value. -/
theorem receiptEventsScan_field_rejected (d n index left consumed offset extraDepth : Nat)
    (seen : List Nat) (field : ReceiptEventsField) (old : ReceiptEventsValue)
    (scanned tail : List w8) (reason : ReceiptEventsFieldFailure)
    (scan : WantedValid d receiptEventsNames seen n scanned (.field index left))
    (lookup : receiptEventsFields[index]? = some field)
    (failure : ReceiptEventsFieldRejected d field tail consumed offset extraDepth reason) :
    ReceiptEventsPairsRejected d n seen old (scanned ++ tail) old
      (scanned.length + consumed) (scanned.length + offset) extraDepth (.field reason) := by
  induction n generalizing scanned with
  | zero => cases scan
  | succ n ih =>
    cases scan with
    | selected keyValid fresh => exact .value keyValid (.selected fresh lookup failure)
    | skipped keyValid ignored valueValid scanRest =>
      have remaining := ih _ scanRest
      have pairs := ReceiptEventsPairsRejected.next keyValid (.ignored ignored valueValid)
        (by simpa only [receiptEventsNextSeen, ignored] using remaining)
      simpa only [List.append_assoc, List.length_append, Nat.add_assoc] using pairs

/-- Later failures retain the selected field's published update. Their absolute
    positions include the scan, the successful value and the rejected tail. -/
theorem receiptEventsScan_tail_rejected (d n index left consumed offset extraDepth : Nat)
    (seen : List Nat) (field : ReceiptEventsField) (old middle final : ReceiptEventsValue)
    (scanned value tail : List w8) (reason : ReceiptEventsFailure)
    (scan : WantedValid d receiptEventsNames seen n scanned (.field index left))
    (lookup : receiptEventsFields[index]? = some field)
    (valid : ReceiptEventsFieldValid d field old value tail middle)
    (failure : ReceiptEventsPairsRejected d left (index :: seen) middle tail final consumed offset extraDepth reason) :
    ReceiptEventsPairsRejected d n seen old (scanned ++ value ++ tail) final
      (scanned.length + value.length + consumed) (scanned.length + value.length + offset) extraDepth reason := by
  induction n generalizing scanned with
  | zero => cases scan
  | succ n ih =>
    cases scan with
    | selected keyValid fresh =>
      apply ReceiptEventsPairsRejected.next keyValid (.selected fresh lookup valid)
      simpa only [receiptEventsNextSeen, fresh] using failure
    | skipped keyValid ignored valueValid scanRest =>
      have remaining := ih _ scanRest
      have pairs := ReceiptEventsPairsRejected.next keyValid (.ignored ignored valueValid)
        (by simpa only [receiptEventsNextSeen, ignored] using remaining)
      simpa only [List.append_assoc, List.length_append, Nat.add_assoc] using pairs

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsScan_done
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsScan_selected
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsScan_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsScan_field_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsScan_tail_rejected
