import ProofCheck.StateUpdateAPISpec
import ProofCheck.StateUpdateErrors
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

/-- Public diagnostics retain their initialized category, concrete error object
    and nonwrapping parser offset or trailing-byte count. Nested StateDiff
    failure ownership includes any private partially decoded child. -/
def StateUpdateAPIErrorOwned (result : GoInterface) : StateUpdateAPIError → IProp GF
  | .nonzero => iprop(∃ (category : GoInterface) (err : Loc), globalAddr cborerr.ErrUnsupported ↦□ category ∗
      err ↦ {(zero_val cborerr.DecodeError) with Kind' := category, Detail' := go!"the destination is not the zero value"} ∗
      ⌜result = interface.mkOk (go.PointerType cborerr.DecodeError.ty) #err⌝)
  | .decode offset reason => iprop(∃ err : Loc, StateUpdateFailureOwned err (W64 offset) reason ∗
      ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧ sint.Z (W64 offset) = offset⌝)
  | .trailing extra => iprop(∃ (category : GoInterface) (err : Loc), globalAddr cborerr.ErrExtraneousData ↦□ category ∗
      err ↦ {(zero_val cborerr.DecodeError) with Kind' := category, Extra' := W64 extra} ∗
      ⌜result = interface.mkOk (go.PointerType cborerr.DecodeError.ty) #err ∧ sint.Z (W64 extra) = extra⌝)

/-- Success returns an exact slice of the original input, including its backing
    pointer and capacity, rather than a copy with equal bytes. Errors return nil. -/
def StateUpdateFirstResultOwned (dst : Loc)
    (input remaining : GoSlice) (wire : List w8) (result : GoInterface) : StateUpdateFirstResult → IProp GF
  | .success final rest => iprop(StateUpdateOwned dst final ∗ ∃ consumed : Nat,
      ⌜result = interface.nil ∧ remaining = slice.slice input w8 (W64 consumed) input.len ∧
        sint.Z (W64 consumed) = consumed ∧ consumed + rest.length = wire.length ∧
        sint.Z remaining.len = rest.length ∧ subslice (sint.nat (W64 consumed)) (sint.nat input.len) wire = rest⌝)
  | .failure final error => iprop(StateUpdateOwned dst final ∗
      StateUpdateAPIErrorOwned result error ∗ ⌜remaining = slice.nil⌝)

def StateUpdateFullResultOwned (dst : Loc) (result : GoInterface) : StateUpdateDecodeResult → IProp GF
  | .success final => iprop(StateUpdateOwned dst final ∗ ⌜result = interface.nil⌝)
  | .failure final error => iprop(StateUpdateOwned dst final ∗ StateUpdateAPIErrorOwned result error)

theorem stateUpdateAPIErrorOwned_nonNil (result : GoInterface) (error : StateUpdateAPIError) :
    StateUpdateAPIErrorOwned (GF := GF) result error ⊢ ⌜result ≠ interface.nil⌝ := by
  cases error with
  | nonzero =>
    isimp only [StateUpdateAPIErrorOwned]
    iintro ⟨%category, %err, _, _, %heq⟩
    ipureintro; rw [heq]; simp
  | decode offset reason =>
    isimp only [StateUpdateAPIErrorOwned]
    iintro ⟨%err, _, %heq⟩
    ipureintro; rw [heq.1]; simp
  | trailing extra =>
    isimp only [StateUpdateAPIErrorOwned]
    iintro ⟨%category, %err, _, _, %heq⟩
    ipureintro; rw [heq.1]; simp

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateAPIErrorOwned_nonNil
