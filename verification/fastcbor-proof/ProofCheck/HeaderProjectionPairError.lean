import ProofCheck.HeaderProjectionFieldRejected
import ProofCheck.HeaderProjectionRejectDecompose
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

def headerProjectionPairFailureKind (reason : HeaderProjectionFailure) (kinds : FeltErrorKinds) : GoInterface :=
  match reason with
  | .key reason => keyFailureKind reason kinds
  | .ignored reason => skipFailureKind reason kinds.toSkipErrorKinds
  | .field reason => headerProjectionFieldFailureKind reason kinds

def HeaderProjectionPairError (err : Loc) (pos : w64) (reason : HeaderProjectionFailure) (category : GoInterface) : IProp GF :=
  match reason with
  | .key reason => KeyFailureError err pos reason category
  | .ignored reason => SkipFailureError err pos reason category
  | .field reason => HeaderProjectionFieldError err pos reason category

theorem headerProjectionPairError_scan (err : Loc) (pos : w64) (reason : WantedFailureReason) (kinds : FeltErrorKinds) :
    HeaderProjectionPairError (GF := GF) err pos (headerProjectionScanFailure reason)
      (headerProjectionPairFailureKind (headerProjectionScanFailure reason) kinds) =
    WantedFailureError err pos reason (wantedFailureKind reason kinds) := by
  cases reason <;> rfl

/-- The selected-field error category comes from the existing initialized
    globals, rather than an assumed category for each generated reader. -/
theorem headerProjectionFieldErrorGlobals_lookup (kinds : FeltErrorKinds) (reason : HeaderProjectionFieldFailure) :
    FeltErrorGlobals (GF := GF) kinds ⊢
      headerProjectionFieldFailureAddr (GF := GF) reason ↦□ headerProjectionFieldFailureKind reason kinds := by
  iintro Hglobals
  cases reason with
  | felt reason =>
    isimp only [headerProjectionFieldFailureAddr, headerProjectionFieldFailureKind]
    iapply feltErrorGlobals_lookup kinds reason
    iexact Hglobals
  | scalar reason =>
    ihave Hcategory := feltErrorGlobals_lookup kinds (.expected (W8 0) reason) $$ Hglobals
    isimp only [feltFailureAddr, feltFailureKind] at Hcategory
    isimp only [headerProjectionFieldFailureAddr, headerProjectionFieldFailureKind]
    iexact Hcategory

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionPairError_scan
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionFieldErrorGlobals_lookup
