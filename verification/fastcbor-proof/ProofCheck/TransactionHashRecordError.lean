import ProofCheck.TransactionHashValues
import ProofCheck.TransactionHashRecord
import ProofCheck.TransactionHashPairError

set_option linter.unusedSectionVars false

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

def transactionHashRecordFailureKind (reason : TransactionHashRecordFailure) (kinds : FeltErrorKinds) : GoInterface :=
  match reason with
  | .tagMalformed reason => expectedFailureKind reason kinds
  | .tagUnsupported => kinds.unsupportedKind
  | .opening reason => feltFailureKind reason kinds
  | .pairs reason => transactionHashPairFailureKind reason kinds

/-- Allocated record diagnostics; unsupported tags use a bare sentinel instead. -/
def TransactionHashRecordDiagnostic (err : Loc) (pos : w64) (reason : TransactionHashRecordFailure)
    (category : GoInterface) : IProp GF :=
  match reason with
  | .tagMalformed reason => ExpectedFailureError err pos (W8 192) reason category
  | .tagUnsupported => iprop(False)
  | .opening reason => FeltFailureError err pos reason category
  | .pairs reason => TransactionHashPairError err pos reason category

/-- The actual error representation distinguishes the non-nil bare sentinel
    from owned allocated diagnostics with their exact absolute position. -/
def TransactionHashRecordError (result : GoInterface) (pos : w64) (reason : TransactionHashRecordFailure)
    (kinds : FeltErrorKinds) : IProp GF :=
  match reason with
  | .tagUnsupported => iprop(⌜result = kinds.unsupportedKind ∧ result ≠ interface.nil⌝)
  | _ => iprop(∃ err : Loc,
      ⌜result = interface.mkOk (go.PointerType utils.cbor.cursor.DecodeError.ty) #err⌝ ∗
      TransactionHashRecordDiagnostic err pos reason (transactionHashRecordFailureKind reason kinds))

/-- Allocated diagnostics establish the corresponding public error value;
    the impossible allocated-unsupported case is discharged from False. -/
theorem transactionHashRecordError_diagnostic (err : Loc) (pos : w64)
    (reason : TransactionHashRecordFailure) (kinds : FeltErrorKinds) :
    TransactionHashRecordDiagnostic (GF := GF) err pos reason (transactionHashRecordFailureKind reason kinds) ⊢
      TransactionHashRecordError (interface.mkOk (go.PointerType utils.cbor.cursor.DecodeError.ty) #err) pos reason kinds := by
  iintro Herr
  cases reason with
  | tagUnsupported =>
    isimp only [TransactionHashRecordDiagnostic] at Herr
    icases Herr with %impossible
    exact False.elim impossible
  | tagMalformed reason | opening reason | pairs reason =>
    isimp only [TransactionHashRecordError]
    iexists err
    iframe Herr
    ipureintro
    rfl

/-- Every specified record error is non-nil, which drives the outer API's
    cleanup branch independently of the particular diagnostic category. -/
theorem transactionHashRecordError_nonNil (result : GoInterface) (pos : w64)
    (reason : TransactionHashRecordFailure) (kinds : FeltErrorKinds) :
    TransactionHashRecordError (GF := GF) result pos reason kinds ⊢ ⌜result ≠ interface.nil⌝ := by
  iintro Herr
  cases reason with
  | tagUnsupported =>
    isimp only [TransactionHashRecordError] at Herr
    icases Herr with %Herror
    ipureintro
    exact Herror.2
  | tagMalformed reason | opening reason | pairs reason =>
    isimp only [TransactionHashRecordError] at Herr
    icases Herr with ⟨%err, %Hresult, _⟩
    ipureintro
    rw [Hresult]
    simp

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashRecordError_diagnostic
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashRecordError_nonNil
