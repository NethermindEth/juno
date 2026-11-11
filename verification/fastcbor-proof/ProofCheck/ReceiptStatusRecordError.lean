import ProofCheck.ReceiptStatusRecord
import ProofCheck.ReceiptStatusPairError

set_option linter.unusedSectionVars false

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

def receiptStatusRecordFailureKind (reason : ReceiptStatusRecordFailure) (kinds : FeltErrorKinds) : GoInterface :=
  match reason with
  | .tagMalformed reason => expectedFailureKind reason kinds
  | .tagUnsupported => kinds.unsupportedKind
  | .opening reason => feltFailureKind reason kinds
  | .pairs reason => receiptStatusPairFailureKind reason kinds

/-- Allocated record diagnostics; unsupported tags use a bare sentinel instead. -/
def ReceiptStatusRecordDiagnostic (err : Loc) (pos : w64) (reason : ReceiptStatusRecordFailure)
    (category : GoInterface) : IProp GF :=
  match reason with
  | .tagMalformed reason => ExpectedFailureError err pos (W8 192) reason category
  | .tagUnsupported => iprop(False)
  | .opening reason => FeltFailureError err pos reason category
  | .pairs reason => ReceiptStatusPairError err pos reason category

/-- The actual error representation distinguishes the non-nil bare sentinel
    from owned allocated diagnostics with their exact absolute position. -/
def ReceiptStatusRecordError (result : GoInterface) (pos : w64) (reason : ReceiptStatusRecordFailure)
    (kinds : FeltErrorKinds) : IProp GF :=
  match reason with
  | .tagUnsupported => iprop(⌜result = kinds.unsupportedKind ∧ result ≠ interface.nil⌝)
  | _ => iprop(∃ err : Loc,
      ⌜result = interface.mkOk (go.PointerType utils.cbor.cursor.DecodeError.ty) #err⌝ ∗
      ReceiptStatusRecordDiagnostic err pos reason (receiptStatusRecordFailureKind reason kinds))

/-- Allocated diagnostics establish the corresponding public error value;
    the impossible allocated-unsupported case is discharged from False. -/
theorem receiptStatusRecordError_diagnostic (err : Loc) (pos : w64)
    (reason : ReceiptStatusRecordFailure) (kinds : FeltErrorKinds) :
    ReceiptStatusRecordDiagnostic (GF := GF) err pos reason (receiptStatusRecordFailureKind reason kinds) ⊢
      ReceiptStatusRecordError (interface.mkOk (go.PointerType utils.cbor.cursor.DecodeError.ty) #err) pos reason kinds := by
  iintro Herr
  cases reason with
  | tagUnsupported =>
    isimp only [ReceiptStatusRecordDiagnostic] at Herr
    icases Herr with %impossible
    exact False.elim impossible
  | tagMalformed reason | opening reason | pairs reason =>
    isimp only [ReceiptStatusRecordError]
    iexists err
    iframe Herr
    ipureintro
    rfl

/-- Every specified record error is non-nil, which drives the outer API's
    cleanup branch independently of the particular diagnostic category. -/
theorem receiptStatusRecordError_nonNil (result : GoInterface) (pos : w64)
    (reason : ReceiptStatusRecordFailure) (kinds : FeltErrorKinds) :
    ReceiptStatusRecordError (GF := GF) result pos reason kinds ⊢ ⌜result ≠ interface.nil⌝ := by
  iintro Herr
  cases reason with
  | tagUnsupported =>
    isimp only [ReceiptStatusRecordError] at Herr
    icases Herr with %Herror
    ipureintro
    exact Herror.2
  | tagMalformed reason | opening reason | pairs reason =>
    isimp only [ReceiptStatusRecordError] at Herr
    icases Herr with ⟨%err, %Hresult, _⟩
    ipureintro
    rw [Hresult]
    simp

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusRecordError_diagnostic
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusRecordError_nonNil
