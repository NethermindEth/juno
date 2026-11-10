import ProofCheck.HeaderRecordSpec
import ProofCheck.HeaderLookupFailure

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- The independent record reason selects the actual shared error sentinel. -/
def headerRecordFailureKind (reason : HeaderRecordFailureReason) (kinds : FeltErrorKinds) : GoInterface :=
  match reason with
  | .opening reason => feltFailureKind reason kinds
  | .value reason => headerValueFailureKind reason kinds
  | .duplicate _ => kinds.unsupportedKind
  | .key reason => keyFailureKind reason kinds

/-- Heap ownership of the exact concrete diagnostic for the independent
    Header record reason, including duplicate-field formatting arguments. -/
def HeaderRecordFailureError (err : Loc) (pos : w64) (reason : HeaderRecordFailureReason) (kind : GoInterface) : IProp GF :=
  match reason with
  | .opening reason => FeltFailureError err pos reason kind
  | .value reason => HeaderValueFailureError err pos reason kind
  | .duplicate field => HeaderLookupFailureError err pos (.duplicate field) kind
  | .key reason => KeyFailureError err pos reason kind

end github_com.NethermindEth.juno.core
end Perennial
