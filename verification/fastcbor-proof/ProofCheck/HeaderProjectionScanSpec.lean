import ProofCheck.HeaderProjectionPairsUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def headerProjectionScanFailure : WantedFailureReason → HeaderProjectionFailure
  | .key reason => .key reason
  | .skipped reason => .ignored reason

/-- An exhausted batched scan is exactly a sequence of ignored pairs and
    therefore leaves the projection value unchanged. -/
theorem headerProjectionScan_done (d n : Nat) (kind : HeaderProjectionKind)
    (seen : List Nat) (old : HeaderProjectionValue) (wire : List w8)
    (scan : WantedValid d (headerProjectionNames kind) seen n wire .done) :
    HeaderProjectionPairsValid d kind n seen old wire old := by
  induction n generalizing wire with
  | zero => cases scan; exact .nil seen old
  | succ n ih =>
    cases scan with
    | skipped keyValid ignored valueValid rest =>
      apply HeaderProjectionPairsValid.step keyValid (.ignored ignored valueValid)
      simpa only [headerProjectionNextSeen, ignored] using ih _ rest

/-- A selected key, its successfully decoded value and the remaining pair
    interpretation compose through any ignored prefix consumed by NextWanted. -/
theorem headerProjectionScan_selected (d n index left : Nat) (kind : HeaderProjectionKind)
    (seen : List Nat) (field : HeaderProjectionField) (old middle final : HeaderProjectionValue)
    (scanned value tail : List w8)
    (scan : WantedValid d (headerProjectionNames kind) seen n scanned (.field index left))
    (lookup : (headerProjectionFields kind)[index]? = some field)
    (valid : HeaderProjectionFieldValid d field old value middle)
    (rest : HeaderProjectionPairsValid d kind left (index :: seen) middle tail final) :
    HeaderProjectionPairsValid d kind n seen old (scanned ++ value ++ tail) final := by
  induction n generalizing scanned with
  | zero => cases scan
  | succ n ih =>
    cases scan with
    | selected keyValid fresh =>
      apply HeaderProjectionPairsValid.step keyValid (.selected fresh lookup valid)
      simpa only [headerProjectionNextSeen, fresh] using rest
    | @skipped n2 key ignoredValue items choice result keyValid ignored valueValid scanRest =>
      have remaining := ih _ scanRest
      have pairs := HeaderProjectionPairsValid.step keyValid (.ignored ignored valueValid)
        (by simpa only [headerProjectionNextSeen, ignored] using remaining)
      simpa only [List.append_assoc] using pairs

/-- Key and ignored-value scan failures retain all existing projection fields;
    their consumption and diagnostic offset agree with the per-pair policy. -/
theorem headerProjectionScan_rejected (d n consumed offset left : Nat) (kind : HeaderProjectionKind)
    (seen : List Nat) (old : HeaderProjectionValue) (wire : List w8) (reason : WantedFailureReason)
    (scan : WantedRejected d (headerProjectionNames kind) seen n wire consumed offset left reason) :
    HeaderProjectionPairsRejected d kind n seen old wire old consumed offset (headerProjectionScanFailure reason) := by
  induction scan with
  | key failure => exact .key failure
  | skipped keyValid ignored failure => exact .value keyValid (.ignored ignored failure)
  | next keyValid ignored valueValid failure ih =>
    apply HeaderProjectionPairsRejected.next keyValid (.ignored ignored valueValid)
    simpa only [headerProjectionNextSeen, ignored] using ih

/-- A failed selected-field read rolls back to its value start after the scan;
    prior ignored pairs do not change the retained projection value. -/
theorem headerProjectionScan_field_rejected (d n index left offset : Nat) (kind : HeaderProjectionKind)
    (seen : List Nat) (field : HeaderProjectionField) (old : HeaderProjectionValue)
    (scanned tail : List w8) (reason : HeaderProjectionFieldFailure)
    (scan : WantedValid d (headerProjectionNames kind) seen n scanned (.field index left))
    (lookup : (headerProjectionFields kind)[index]? = some field)
    (failure : HeaderProjectionFieldRejected d field tail offset reason) :
    HeaderProjectionPairsRejected d kind n seen old (scanned ++ tail) old
      scanned.length (scanned.length + offset) (.field reason) := by
  induction n generalizing scanned with
  | zero => cases scan
  | succ n ih =>
    cases scan with
    | selected keyValid fresh => exact .value keyValid (.selected fresh lookup failure)
    | skipped keyValid ignored valueValid scanRest =>
      have remaining := ih _ scanRest
      have pairs := HeaderProjectionPairsRejected.next keyValid (.ignored ignored valueValid)
        (by simpa only [headerProjectionNextSeen, ignored] using remaining)
      simpa only [List.append_assoc, List.length_append, Nat.add_assoc] using pairs

/-- Later failures retain the selected field's published update. Their absolute
    positions include the scan, the successful value and the rejected tail. -/
theorem headerProjectionScan_tail_rejected (d n index left consumed offset : Nat) (kind : HeaderProjectionKind)
    (seen : List Nat) (field : HeaderProjectionField) (old middle final : HeaderProjectionValue)
    (scanned value tail : List w8) (reason : HeaderProjectionFailure)
    (scan : WantedValid d (headerProjectionNames kind) seen n scanned (.field index left))
    (lookup : (headerProjectionFields kind)[index]? = some field)
    (valid : HeaderProjectionFieldValid d field old value middle)
    (failure : HeaderProjectionPairsRejected d kind left (index :: seen) middle tail final consumed offset reason) :
    HeaderProjectionPairsRejected d kind n seen old (scanned ++ value ++ tail) final
      (scanned.length + value.length + consumed) (scanned.length + value.length + offset) reason := by
  induction n generalizing scanned with
  | zero => cases scan
  | succ n ih =>
    cases scan with
    | selected keyValid fresh =>
      apply HeaderProjectionPairsRejected.next keyValid (.selected fresh lookup valid)
      simpa only [headerProjectionNextSeen, fresh] using failure
    | skipped keyValid ignored valueValid scanRest =>
      have remaining := ih _ scanRest
      have pairs := HeaderProjectionPairsRejected.next keyValid (.ignored ignored valueValid)
        (by simpa only [headerProjectionNextSeen, ignored] using remaining)
      simpa only [List.append_assoc, List.length_append, Nat.add_assoc] using pairs

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionScan_done
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionScan_selected
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionScan_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionScan_field_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionScan_tail_rejected
