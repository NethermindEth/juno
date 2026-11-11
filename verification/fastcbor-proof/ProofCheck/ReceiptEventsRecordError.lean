import ProofCheck.ReceiptEventsRecord
import ProofCheck.ReceiptEventsPairError

set_option linter.unusedSectionVars false

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Allocated record diagnostics; unsupported tags use a bare sentinel instead. -/
def ReceiptEventsRecordDiagnostic (err : Loc) (pos : w64) (reason : ReceiptEventsRecordFailure)
    (kinds : FeltErrorKinds) : IProp GF :=
  match reason with
  | .tagMalformed reason => ExpectedFailureError err pos (W8 192) reason (expectedFailureKind reason kinds)
  | .tagUnsupported => iprop(False)
  | .opening reason => FeltFailureError err pos reason (feltFailureKind reason kinds)
  | .pairs reason => ReceiptEventsPairErrorOwned err pos reason

/-- The actual error representation distinguishes the non-nil bare sentinel
    from owned allocated diagnostics with their exact absolute position. -/
def ReceiptEventsRecordError (result : GoInterface) (pos : w64) (reason : ReceiptEventsRecordFailure)
    (kinds : FeltErrorKinds) : IProp GF :=
  match reason with
  | .tagUnsupported => iprop(⌜result = kinds.unsupportedKind ∧ result ≠ interface.nil⌝)
  | _ => iprop(∃ err : Loc,
      ⌜result = interface.mkOk (go.PointerType utils.cbor.cursor.DecodeError.ty) #err⌝ ∗
      ReceiptEventsRecordDiagnostic err pos reason kinds)

/-- Allocated diagnostics establish the corresponding public error value;
    the impossible allocated-unsupported case is discharged from False. -/
theorem receiptEventsRecordError_diagnostic (err : Loc) (pos : w64)
    (reason : ReceiptEventsRecordFailure) (kinds : FeltErrorKinds) :
    ReceiptEventsRecordDiagnostic (GF := GF) err pos reason kinds ⊢
      ReceiptEventsRecordError (interface.mkOk (go.PointerType utils.cbor.cursor.DecodeError.ty) #err) pos reason kinds := by
  iintro Herr
  cases reason with
  | tagUnsupported =>
    isimp only [ReceiptEventsRecordDiagnostic] at Herr
    icases Herr with %impossible
    exact False.elim impossible
  | tagMalformed reason | opening reason | pairs reason =>
    isimp only [ReceiptEventsRecordError]
    iexists err
    iframe Herr
    ipureintro
    rfl

/-- Every specified record error is non-nil, which drives the outer API's
    cleanup branch independently of the particular diagnostic category. -/
theorem receiptEventsRecordError_nonNil (result : GoInterface) (pos : w64)
    (reason : ReceiptEventsRecordFailure) (kinds : FeltErrorKinds) :
    ReceiptEventsRecordError (GF := GF) result pos reason kinds ⊢ ⌜result ≠ interface.nil⌝ := by
  iintro Herr
  cases reason with
  | tagUnsupported =>
    isimp only [ReceiptEventsRecordError] at Herr
    icases Herr with %Herror
    ipureintro
    exact Herror.2
  | tagMalformed reason | opening reason | pairs reason =>
    isimp only [ReceiptEventsRecordError] at Herr
    icases Herr with ⟨%err, %Hresult, _⟩
    ipureintro
    rw [Hresult]
    simp

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsRecordError_diagnostic
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsRecordError_nonNil
