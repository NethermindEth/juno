import ProofCheck.ClassAPISpec
import ProofCheck.ClassComplete
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

/-- Ownership of a successful class interface and its concrete payload. -/
def ClassValueDestination (dst : Loc) (value : ClassValue) : IProp GF :=
  iprop(∃ out : GoInterface, dst ↦ out ∗ ClassValueOwned out value)

/-- Guard diagnostics refer to the initialized entry sentinel; decoder errors
    retain their initialized parser category and exact nonwrapping offset. -/
def ClassAPIErrorOwned (result : GoInterface) : ClassAPIError → IProp GF
  | .nonzero => iprop(∃ (category : GoInterface) (err : Loc), globalAddr cborerr.ErrUnsupported ↦□ category ∗
      err ↦ {(zero_val cborerr.DecodeError) with Kind' := category, Detail' := go!"the destination is not the zero value"} ∗
      ⌜result = interface.mkOk (go.PointerType cborerr.DecodeError.ty) #err⌝)
  | .decode offset detail => iprop(ClassErrorOwned result (W64 offset) detail ∗
      ⌜sint.Z (W64 offset) = offset⌝)
  | .trailing extra => iprop(∃ (category : GoInterface) (err : Loc), globalAddr cborerr.ErrExtraneousData ↦□ category ∗
      err ↦ {(zero_val cborerr.DecodeError) with Kind' := category, Extra' := W64 extra} ∗
      ⌜result = interface.mkOk (go.PointerType cborerr.DecodeError.ty) #err ∧ sint.Z (W64 extra) = extra⌝)

/-- Success returns an exact slice of the original input, including its backing
    pointer and capacity, rather than a copy with equal bytes. Errors return nil. -/
def ClassFirstResultOwned (dst : Loc)
    (input remaining : GoSlice) (wire : List w8) (result : GoInterface) : ClassFirstResult → IProp GF
  | .success final rest => iprop(ClassValueDestination dst final ∗ ∃ consumed : Nat,
      ⌜result = interface.nil ∧ remaining = slice.slice input w8 (W64 consumed) input.len ∧
        sint.Z (W64 consumed) = consumed ∧ consumed + rest.length = wire.length ∧
        sint.Z remaining.len = rest.length ∧ subslice (sint.nat (W64 consumed)) (sint.nat input.len) wire = rest⌝)
  | .failure final error => iprop(dst ↦ final ∗
      ClassAPIErrorOwned result error ∗ ⌜remaining = slice.nil⌝)

def ClassFullResultOwned (dst : Loc) (result : GoInterface) : ClassDecodeResult → IProp GF
  | .success final => iprop(ClassValueDestination dst final ∗ ⌜result = interface.nil⌝)
  | .failure final error => iprop(dst ↦ final ∗ ClassAPIErrorOwned result error)

theorem classAPIErrorOwned_nonNil (result : GoInterface) (error : ClassAPIError) :
    ClassAPIErrorOwned (GF := GF) result error ⊢ ⌜result ≠ interface.nil⌝ := by
  cases error with
  | nonzero =>
    isimp only [ClassAPIErrorOwned]
    iintro ⟨%category, %err, _, _, %heq⟩
    ipureintro; rw [heq]; simp
  | decode offset detail =>
    isimp only [ClassAPIErrorOwned]
    iintro ⟨Herr, _⟩
    iapply classErrorOwned_nonNil result (W64 offset) detail
    iexact Herr
  | trailing extra =>
    isimp only [ClassAPIErrorOwned]
    iintro ⟨%category, %err, _, _, %heq⟩
    ipureintro; rw [heq.1]; simp

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.classAPIErrorOwned_nonNil
