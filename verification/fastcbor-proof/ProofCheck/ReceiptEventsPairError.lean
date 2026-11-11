import ProofCheck.ReceiptEventsFieldRead
import ProofCheck.ReceiptEventsRejectDecompose
import ProofCheck.ProjectionScanRejected

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Owned diagnostics from either the batched scanner or selected-field reader.
    Category identities come from initialized globals in each helper contract. -/
def ReceiptEventsPairErrorOwned (err : Loc) (pos : w64) : ReceiptEventsFailure → IProp GF
  | .key reason => iprop(∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
      KeyFailureError err pos reason (keyFailureKind reason kinds))
  | .ignored reason => iprop(∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
      SkipFailureError err pos reason (skipFailureKind reason kinds.toSkipErrorKinds))
  | .field reason => ReceiptEventsFieldErrorOwned err pos reason

theorem receiptEventsPairError_scan (err : Loc) (pos : w64)
    (reason : WantedFailureReason) (kinds : FeltErrorKinds) :
    FeltErrorGlobals kinds ∗ WantedFailureError err pos reason (wantedFailureKind reason kinds) ⊢
      (ReceiptEventsPairErrorOwned err pos (receiptEventsScanFailure reason) : IProp GF) := by
  cases reason <;>
    isimp only [ReceiptEventsPairErrorOwned, receiptEventsScanFailure, WantedFailureError, wantedFailureKind] <;>
    iintro ⟨Hglobals, Herr⟩ <;> iexists kinds <;> iframe Hglobals Herr

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsPairError_scan
