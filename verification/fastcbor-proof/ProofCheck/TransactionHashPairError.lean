import ProofCheck.TransactionHashValues
import ProofCheck.TransactionHashFieldSpec
import ProofCheck.TransactionHashRejectDecompose
import ProofCheck.TransactionHashScanRejected

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

def transactionHashPairFailureKind (reason : TransactionHashFailure) (kinds : FeltErrorKinds) : GoInterface :=
  match reason with
  | .key reason => keyFailureKind reason kinds
  | .ignored reason => skipFailureKind reason kinds.toSkipErrorKinds
  | .field reason => transactionHashFieldFailureKind reason kinds

def TransactionHashPairError (err : Loc) (pos : w64) (reason : TransactionHashFailure) (category : GoInterface) : IProp GF :=
  match reason with
  | .key reason => KeyFailureError err pos reason category
  | .ignored reason => SkipFailureError err pos reason category
  | .field reason => TransactionHashFieldError err pos reason category

theorem transactionHashPairError_scan (err : Loc) (pos : w64) (reason : WantedFailureReason) (kinds : FeltErrorKinds) :
    TransactionHashPairError (GF := GF) err pos (transactionHashScanFailure reason)
      (transactionHashPairFailureKind (transactionHashScanFailure reason) kinds) =
    WantedFailureError err pos reason (wantedFailureKind reason kinds) := by
  cases reason <;> rfl

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashPairError_scan
