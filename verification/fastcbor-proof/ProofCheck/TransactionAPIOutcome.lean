import ProofCheck.TransactionAPISpec
import ProofCheck.TransactionComplete
import ProofCheck.EntryErrorsInitialize

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Ownership of a successful transaction interface and its concrete payload. -/
def TransactionValueDestination (dst : Loc) (value : TransactionValue) : IProp GF :=
  iprop(∃ out : GoInterface, dst ↦ out ∗ TransactionValueOwned out value)

/-- Guard diagnostics refer to the initialized entry sentinel; decoder errors
    retain their initialized parser category and exact nonwrapping offset. -/
def TransactionAPIErrorOwned (result : GoInterface) : TransactionAPIError → IProp GF
  | .nonzero => iprop(∃ (category : GoInterface) (err : Loc), globalAddr cborerr.ErrUnsupported ↦□ category ∗
      err ↦ {(zero_val cborerr.DecodeError) with Kind' := category, Detail' := go!"the destination is not the zero value"} ∗
      ⌜result = interface.mkOk (go.PointerType cborerr.DecodeError.ty) #err⌝)
  | .decode offset detail => iprop(TransactionErrorOwned result (W64 offset) detail ∗
      ⌜sint.Z (W64 offset) = offset⌝)
  | .trailing extra => iprop(∃ (category : GoInterface) (err : Loc), globalAddr cborerr.ErrExtraneousData ↦□ category ∗
      err ↦ {(zero_val cborerr.DecodeError) with Kind' := category, Extra' := W64 extra} ∗
      ⌜result = interface.mkOk (go.PointerType cborerr.DecodeError.ty) #err ∧ sint.Z (W64 extra) = extra⌝)

/-- Success returns an exact slice of the original input, including its backing
    pointer and capacity, rather than a copy with equal bytes. Errors return nil. -/
def TransactionFirstResultOwned (dst : Loc)
    (input remaining : GoSlice) (wire : List w8) (result : GoInterface) : TransactionFirstResult → IProp GF
  | .success final rest => iprop(TransactionValueDestination dst final ∗ ∃ consumed : Nat,
      ⌜result = interface.nil ∧ remaining = slice.slice input w8 (W64 consumed) input.len ∧
        sint.Z (W64 consumed) = consumed ∧ consumed + rest.length = wire.length ∧
        sint.Z remaining.len = rest.length ∧ subslice (sint.nat (W64 consumed)) (sint.nat input.len) wire = rest⌝)
  | .failure final error => iprop(dst ↦ final ∗
      TransactionAPIErrorOwned result error ∗ ⌜remaining = slice.nil⌝)

def TransactionFullResultOwned (dst : Loc) (result : GoInterface) : TransactionDecodeResult → IProp GF
  | .success final => iprop(TransactionValueDestination dst final ∗ ⌜result = interface.nil⌝)
  | .failure final error => iprop(dst ↦ final ∗ TransactionAPIErrorOwned result error)

theorem transactionAPIErrorOwned_nonNil (result : GoInterface) (error : TransactionAPIError) :
    TransactionAPIErrorOwned (GF := GF) result error ⊢ ⌜result ≠ interface.nil⌝ := by
  cases error with
  | nonzero =>
    isimp only [TransactionAPIErrorOwned]
    iintro ⟨%category, %err, _, _, %heq⟩
    ipureintro; rw [heq]; simp
  | decode offset detail =>
    isimp only [TransactionAPIErrorOwned]
    iintro ⟨Herr, _⟩
    iapply transactionErrorOwned_nonNil result (W64 offset) detail
    iexact Herr
  | trailing extra =>
    isimp only [TransactionAPIErrorOwned]
    iintro ⟨%category, %err, _, _, %heq⟩
    ipureintro; rw [heq.1]; simp

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionAPIErrorOwned_nonNil
