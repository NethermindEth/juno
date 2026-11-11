import ProofCheck.ReceiptStatusFieldRead
import ProofCheck.ReceiptStatusRejectDecompose
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

def receiptStatusPairFailureKind (reason : ReceiptStatusFailure) (kinds : FeltErrorKinds) : GoInterface :=
  match reason with
  | .key reason => keyFailureKind reason kinds
  | .ignored reason => skipFailureKind reason kinds.toSkipErrorKinds
  | .field reason => receiptStatusFieldFailureKind reason kinds

def ReceiptStatusPairError (err : Loc) (pos : w64) (reason : ReceiptStatusFailure) (category : GoInterface) : IProp GF :=
  match reason with
  | .key reason => KeyFailureError err pos reason category
  | .ignored reason => SkipFailureError err pos reason category
  | .field reason => ReceiptStatusFieldError err pos reason category

theorem receiptStatusPairError_scan (err : Loc) (pos : w64) (reason : WantedFailureReason) (kinds : FeltErrorKinds) :
    ReceiptStatusPairError (GF := GF) err pos (receiptStatusScanFailure reason)
      (receiptStatusPairFailureKind (receiptStatusScanFailure reason) kinds) =
    WantedFailureError err pos reason (wantedFailureKind reason kinds) := by
  cases reason <;> rfl

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusPairError_scan
