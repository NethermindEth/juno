import ProofCheck.HeaderProjectionScanSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- One batched-scan step of a rejected projection: the scan fails, its selected
    field fails, or that field succeeds and rejection occurs in the remainder.
    Consumption and diagnostic position are deliberately separate. -/
inductive HeaderProjectionRejectStep (d : Nat) (kind : HeaderProjectionKind) :
    Nat → List Nat → HeaderProjectionValue → List w8 → HeaderProjectionValue → Nat → Nat → HeaderProjectionFailure → Prop where
  | scan {n seen old wire consumed offset left reason}
      (failure : WantedRejected d (headerProjectionNames kind) seen n wire consumed offset left reason) :
      HeaderProjectionRejectStep d kind n seen old wire old consumed offset (headerProjectionScanFailure reason)
  | field {n seen old scanned tail index left field offset reason}
      (scan : WantedValid d (headerProjectionNames kind) seen n scanned (.field index left))
      (lookup : (headerProjectionFields kind)[index]? = some field)
      (failure : HeaderProjectionFieldRejected d field tail offset reason) :
      HeaderProjectionRejectStep d kind n seen old (scanned ++ tail) old
        scanned.length (scanned.length + offset) (.field reason)
  | next {n seen old middle final scanned value tail index left field consumed offset reason}
      (scan : WantedValid d (headerProjectionNames kind) seen n scanned (.field index left))
      (lookup : (headerProjectionFields kind)[index]? = some field)
      (valid : HeaderProjectionFieldValid d field old value middle)
      (failure : HeaderProjectionPairsRejected d kind left (index :: seen) middle tail final consumed offset reason) :
      HeaderProjectionRejectStep d kind n seen old (scanned ++ value ++ tail) final
        (scanned.length + value.length + consumed) (scanned.length + value.length + offset) reason

/-- Prepending an ignored pair preserves published fields and selection while
    shifting both positions by the pair's full encoded length. -/
theorem headerProjectionRejectStep_ignored (d n consumed offset : Nat) (kind : HeaderProjectionKind)
    (seen : List Nat) (choice : Option Nat) (old final : HeaderProjectionValue)
    (key value wire : List w8) (reason : HeaderProjectionFailure)
    (keyValid : WantedKeyValid (headerProjectionNames kind) key choice)
    (ignored : wantedSelect seen choice = none) (valueValid : SkipValid d value)
    (step : HeaderProjectionRejectStep d kind n seen old wire final consumed offset reason) :
    HeaderProjectionRejectStep d kind (n + 1) seen old (key ++ value ++ wire) final
      (key.length + value.length + consumed) (key.length + value.length + offset) reason := by
  cases step with
  | scan failure => exact .scan (.next keyValid ignored valueValid failure)
  | field scan lookup failure =>
    simpa only [List.append_assoc, List.length_append, Nat.add_assoc] using
      HeaderProjectionRejectStep.field (WantedValid.skipped keyValid ignored valueValid scan) lookup failure
  | next scan lookup valid failure =>
    simpa only [List.append_assoc, List.length_append, Nat.add_assoc] using
      HeaderProjectionRejectStep.next (WantedValid.skipped keyValid ignored valueValid scan) lookup valid failure

/-- Every independent per-pair rejection exposes the next batched scanner step,
    including arbitrary ignored and duplicate prefixes. -/
theorem headerProjectionPairs_rejected_scan (d n consumed offset : Nat) (kind : HeaderProjectionKind)
    (seen : List Nat) (old final : HeaderProjectionValue) (wire : List w8) (reason : HeaderProjectionFailure)
    (failure : HeaderProjectionPairsRejected d kind n seen old wire final consumed offset reason) :
    HeaderProjectionRejectStep d kind n seen old wire final consumed offset reason := by
  induction failure with
  | key failure => exact .scan (.key failure)
  | value keyValid failure =>
    cases failure with
    | ignored ignored failure => exact .scan (.skipped keyValid ignored failure)
    | selected fresh lookup failure => exact .field (.selected keyValid fresh) lookup failure
  | @next n seen choice old middle final key value tail consumed offset reason keyValid valueValid failure ih =>
    cases valueValid with
    | ignored ignored valueValid =>
      have next : headerProjectionNextSeen seen choice = seen := by simp [headerProjectionNextSeen, ignored]
      rw [next] at ih
      exact headerProjectionRejectStep_ignored d n consumed offset kind seen choice old final key value tail reason
        keyValid ignored valueValid ih
    | selected fresh lookup valid =>
      apply HeaderProjectionRejectStep.next (.selected keyValid fresh) lookup valid
      simpa only [headerProjectionNextSeen, fresh] using failure

/-- The decomposition preserves the independent record relation in both
    directions; it does not weaken rejection behavior for implementation proofs. -/
theorem headerProjectionRejectStep_pairs (d n consumed offset : Nat) (kind : HeaderProjectionKind)
    (seen : List Nat) (old final : HeaderProjectionValue) (wire : List w8) (reason : HeaderProjectionFailure)
    (step : HeaderProjectionRejectStep d kind n seen old wire final consumed offset reason) :
    HeaderProjectionPairsRejected d kind n seen old wire final consumed offset reason := by
  cases step with
  | scan failure => exact headerProjectionScan_rejected _ _ _ _ _ _ _ _ _ _ failure
  | field scan lookup failure => exact headerProjectionScan_field_rejected _ _ _ _ _ _ _ _ _ _ _ _ scan lookup failure
  | next scan lookup valid failure => exact headerProjectionScan_tail_rejected _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ scan lookup valid failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionRejectStep_ignored
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionPairs_rejected_scan
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionRejectStep_pairs
