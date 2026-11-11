import ProofCheck.ReceiptAPISpec
import ProofCheck.ReceiptRecordComplete
import ProofCheck.ReceiptGuard
import ProofCheck.EntryErrorsInitialize

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

/-- Guard diagnostics refer to the initialized entry sentinel; decoder errors
    retain their initialized parser category and exact nonwrapping offset. -/
def ReceiptAPIErrorOwned (result : GoInterface) : ReceiptAPIError → IProp GF
  | .nonzero => iprop(∃ (category : GoInterface) (err : Loc), globalAddr cborerr.ErrUnsupported ↦□ category ∗
      err ↦ {(zero_val cborerr.DecodeError) with Kind' := category, Detail' := go!"the destination is not the zero value"} ∗
      ⌜result = interface.mkOk (go.PointerType cborerr.DecodeError.ty) #err⌝)
  | .decode offset reason => iprop(ReceiptFailureOwned result (W64 offset) reason ∗
      ⌜sint.Z (W64 offset) = offset⌝)
  | .trailing extra => iprop(∃ (category : GoInterface) (err : Loc), globalAddr cborerr.ErrExtraneousData ↦□ category ∗
      err ↦ {(zero_val cborerr.DecodeError) with Kind' := category, Extra' := W64 extra} ∗
      ⌜result = interface.mkOk (go.PointerType cborerr.DecodeError.ty) #err ∧ sint.Z (W64 extra) = extra⌝)

/-- Success returns an exact slice of the original input, including its backing
    pointer and capacity, rather than a copy with equal bytes. Errors return nil. -/
def ReceiptFirstResultOwned (dst : Loc)
    (input remaining : GoSlice) (wire : List w8) (result : GoInterface) : ReceiptFirstResult → IProp GF
  | .success final rest => iprop(ReceiptOwned dst final ∗ ∃ consumed : Nat,
      ⌜result = interface.nil ∧ remaining = slice.slice input w8 (W64 consumed) input.len ∧
        sint.Z (W64 consumed) = consumed ∧ consumed + rest.length = wire.length ∧
        sint.Z remaining.len = rest.length ∧ subslice (sint.nat (W64 consumed)) (sint.nat input.len) wire = rest⌝)
  | .failure final error => iprop(ReceiptOwned dst final ∗
      ReceiptAPIErrorOwned result error ∗ ⌜remaining = slice.nil⌝)

def ReceiptFullResultOwned (dst : Loc) (result : GoInterface) : ReceiptDecodeResult → IProp GF
  | .success final => iprop(ReceiptOwned dst final ∗ ⌜result = interface.nil⌝)
  | .failure final error => iprop(ReceiptOwned dst final ∗ ReceiptAPIErrorOwned result error)

theorem receiptAPIErrorOwned_nonNil (result : GoInterface) (error : ReceiptAPIError) :
    ReceiptAPIErrorOwned (GF := GF) result error ⊢ ⌜result ≠ interface.nil⌝ := by
  cases error with
  | nonzero =>
    isimp only [ReceiptAPIErrorOwned]
    iintro ⟨%category, %err, _, _, %heq⟩
    ipureintro; rw [heq]; simp
  | decode offset reason =>
    isimp only [ReceiptAPIErrorOwned]
    iintro ⟨Herr, _⟩
    iapply receiptFailureOwned_nonNil result (W64 offset) reason
    iexact Herr
  | trailing extra =>
    isimp only [ReceiptAPIErrorOwned]
    iintro ⟨%category, %err, _, _, %heq⟩
    ipureintro; rw [heq.1]; simp

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptAPIErrorOwned_nonNil
