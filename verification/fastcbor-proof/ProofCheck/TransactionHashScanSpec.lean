import ProofCheck.TransactionHashPairsUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def transactionHashScanFailure : WantedFailureReason → TransactionHashFailure
  | .key reason => .key reason
  | .skipped reason => .ignored reason

/-- An exhausted batched scan is exactly a sequence of ignored pairs and
    therefore leaves the projection value unchanged. -/
theorem transactionHashScan_done (d n : Nat) 
    (seen : List Nat) (old : TransactionHashValue) (wire : List w8)
    (scan : WantedValid d (transactionHashNames ) seen n wire .done) :
    TransactionHashPairsValid d n seen old wire old := by
  induction n generalizing wire with
  | zero => cases scan; exact .nil seen old
  | succ n ih =>
    cases scan with
    | skipped keyValid ignored valueValid rest =>
      apply TransactionHashPairsValid.step keyValid (.ignored ignored valueValid)
      simpa only [transactionHashNextSeen, ignored] using ih _ rest

/-- A selected key, its successfully decoded value and the remaining pair
    interpretation compose through any ignored prefix consumed by NextWanted. -/
theorem transactionHashScan_selected (d n index left : Nat) 
    (seen : List Nat) (field : TransactionHashField) (old middle final : TransactionHashValue)
    (scanned value tail : List w8)
    (scan : WantedValid d (transactionHashNames ) seen n scanned (.field index left))
    (lookup : (transactionHashFields )[index]? = some field)
    (valid : TransactionHashFieldValid d field old value middle)
    (rest : TransactionHashPairsValid d left (index :: seen) middle tail final) :
    TransactionHashPairsValid d n seen old (scanned ++ value ++ tail) final := by
  induction n generalizing scanned with
  | zero => cases scan
  | succ n ih =>
    cases scan with
    | selected keyValid fresh =>
      apply TransactionHashPairsValid.step keyValid (.selected fresh lookup valid)
      simpa only [transactionHashNextSeen, fresh] using rest
    | @skipped n2 key ignoredValue items choice result keyValid ignored valueValid scanRest =>
      have remaining := ih _ scanRest
      have pairs := TransactionHashPairsValid.step keyValid (.ignored ignored valueValid)
        (by simpa only [transactionHashNextSeen, ignored] using remaining)
      simpa only [List.append_assoc] using pairs

/-- Key and ignored-value scan failures retain all existing projection fields;
    their consumption and diagnostic offset agree with the per-pair policy. -/
theorem transactionHashScan_rejected (d n consumed offset left : Nat) 
    (seen : List Nat) (old : TransactionHashValue) (wire : List w8) (reason : WantedFailureReason)
    (scan : WantedRejected d (transactionHashNames ) seen n wire consumed offset left reason) :
    TransactionHashPairsRejected d n seen old wire old consumed offset (transactionHashScanFailure reason) := by
  induction scan with
  | key failure => exact .key failure
  | skipped keyValid ignored failure => exact .value keyValid (.ignored ignored failure)
  | next keyValid ignored valueValid failure ih =>
    apply TransactionHashPairsRejected.next keyValid (.ignored ignored valueValid)
    simpa only [transactionHashNextSeen, ignored] using ih

/-- A failed selected-field read rolls back to its value start after the scan;
    prior ignored pairs do not change the retained projection value. -/
theorem transactionHashScan_field_rejected (d n index left offset : Nat) 
    (seen : List Nat) (field : TransactionHashField) (old : TransactionHashValue)
    (scanned tail : List w8) (reason : TransactionHashFieldFailure)
    (scan : WantedValid d (transactionHashNames ) seen n scanned (.field index left))
    (lookup : (transactionHashFields )[index]? = some field)
    (failure : TransactionHashFieldRejected d field tail offset reason) :
    TransactionHashPairsRejected d n seen old (scanned ++ tail) old
      scanned.length (scanned.length + offset) (.field reason) := by
  induction n generalizing scanned with
  | zero => cases scan
  | succ n ih =>
    cases scan with
    | selected keyValid fresh => exact .value keyValid (.selected fresh lookup failure)
    | skipped keyValid ignored valueValid scanRest =>
      have remaining := ih _ scanRest
      have pairs := TransactionHashPairsRejected.next keyValid (.ignored ignored valueValid)
        (by simpa only [transactionHashNextSeen, ignored] using remaining)
      simpa only [List.append_assoc, List.length_append, Nat.add_assoc] using pairs

/-- Later failures retain the selected field's published update. Their absolute
    positions include the scan, the successful value and the rejected tail. -/
theorem transactionHashScan_tail_rejected (d n index left consumed offset : Nat) 
    (seen : List Nat) (field : TransactionHashField) (old middle final : TransactionHashValue)
    (scanned value tail : List w8) (reason : TransactionHashFailure)
    (scan : WantedValid d (transactionHashNames ) seen n scanned (.field index left))
    (lookup : (transactionHashFields )[index]? = some field)
    (valid : TransactionHashFieldValid d field old value middle)
    (failure : TransactionHashPairsRejected d left (index :: seen) middle tail final consumed offset reason) :
    TransactionHashPairsRejected d n seen old (scanned ++ value ++ tail) final
      (scanned.length + value.length + consumed) (scanned.length + value.length + offset) reason := by
  induction n generalizing scanned with
  | zero => cases scan
  | succ n ih =>
    cases scan with
    | selected keyValid fresh =>
      apply TransactionHashPairsRejected.next keyValid (.selected fresh lookup valid)
      simpa only [transactionHashNextSeen, fresh] using failure
    | skipped keyValid ignored valueValid scanRest =>
      have remaining := ih _ scanRest
      have pairs := TransactionHashPairsRejected.next keyValid (.ignored ignored valueValid)
        (by simpa only [transactionHashNextSeen, ignored] using remaining)
      simpa only [List.append_assoc, List.length_append, Nat.add_assoc] using pairs

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashScan_done
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashScan_selected
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashScan_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashScan_field_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashScan_tail_rejected
