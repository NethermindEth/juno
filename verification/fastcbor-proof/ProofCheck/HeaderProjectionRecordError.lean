import ProofCheck.HeaderProjectionRecord
import ProofCheck.HeaderProjectionPairError

set_option linter.unusedSectionVars false

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

def headerProjectionRecordFailureKind (reason : HeaderProjectionRecordFailure) (kinds : FeltErrorKinds) : GoInterface :=
  match reason with
  | .tagMalformed reason => expectedFailureKind reason kinds
  | .tagUnsupported => kinds.unsupportedKind
  | .opening reason => feltFailureKind reason kinds
  | .pairs reason => headerProjectionPairFailureKind reason kinds

/-- Allocated record diagnostics; unsupported tags use a bare sentinel instead. -/
def HeaderProjectionRecordDiagnostic (err : Loc) (pos : w64) (reason : HeaderProjectionRecordFailure)
    (category : GoInterface) : IProp GF :=
  match reason with
  | .tagMalformed reason => ExpectedFailureError err pos (W8 192) reason category
  | .tagUnsupported => iprop(False)
  | .opening reason => FeltFailureError err pos reason category
  | .pairs reason => HeaderProjectionPairError err pos reason category

/-- The actual error representation distinguishes the non-nil bare sentinel
    from owned allocated diagnostics with their exact absolute position. -/
def HeaderProjectionRecordError (result : GoInterface) (pos : w64) (reason : HeaderProjectionRecordFailure)
    (kinds : FeltErrorKinds) : IProp GF :=
  match reason with
  | .tagUnsupported => iprop(⌜result = kinds.unsupportedKind ∧ result ≠ interface.nil⌝)
  | _ => iprop(∃ err : Loc,
      ⌜result = interface.mkOk (go.PointerType utils.cbor.cursor.DecodeError.ty) #err⌝ ∗
      HeaderProjectionRecordDiagnostic err pos reason (headerProjectionRecordFailureKind reason kinds))

/-- Allocated diagnostics establish the corresponding public error value;
    the impossible allocated-unsupported case is discharged from False. -/
theorem headerProjectionRecordError_diagnostic (err : Loc) (pos : w64)
    (reason : HeaderProjectionRecordFailure) (kinds : FeltErrorKinds) :
    HeaderProjectionRecordDiagnostic (GF := GF) err pos reason (headerProjectionRecordFailureKind reason kinds) ⊢
      HeaderProjectionRecordError (interface.mkOk (go.PointerType utils.cbor.cursor.DecodeError.ty) #err) pos reason kinds := by
  iintro Herr
  cases reason with
  | tagUnsupported =>
    isimp only [HeaderProjectionRecordDiagnostic] at Herr
    icases Herr with %impossible
    exact False.elim impossible
  | tagMalformed reason | opening reason | pairs reason =>
    isimp only [HeaderProjectionRecordError]
    iexists err
    iframe Herr
    ipureintro
    rfl

/-- Every specified record error is non-nil, which drives the outer API's
    cleanup branch independently of the particular diagnostic category. -/
theorem headerProjectionRecordError_nonNil (result : GoInterface) (pos : w64)
    (reason : HeaderProjectionRecordFailure) (kinds : FeltErrorKinds) :
    HeaderProjectionRecordError (GF := GF) result pos reason kinds ⊢ ⌜result ≠ interface.nil⌝ := by
  iintro Herr
  cases reason with
  | tagUnsupported =>
    isimp only [HeaderProjectionRecordError] at Herr
    icases Herr with %Herror
    ipureintro
    exact Herror.2
  | tagMalformed reason | opening reason | pairs reason =>
    isimp only [HeaderProjectionRecordError] at Herr
    icases Herr with ⟨%err, %Hresult, _⟩
    ipureintro
    rw [Hresult]
    simp

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionRecordError_diagnostic
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionRecordError_nonNil
