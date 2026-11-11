import ProofCheck.HeaderProjectionAPISpec
import ProofCheck.HeaderProjectionRecordComplete
import ProofCheck.HeaderProjectionGuard
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

def headerProjectionFirstName : HeaderProjectionKind → GoString
  | .hash => decodeHeaderHashProjectionFirst
  | .root => decodeHeaderGlobalStateRootProjectionFirst
  | .count => decodeHeaderTransactionCountProjectionFirst
  | .timestamp => decodeHeaderTimestampProjectionFirst
  | .hashAndRoot => decodeHeaderHashAndStateRootProjectionFirst

/-- Guard diagnostics refer to the initialized entry sentinel; decoder errors
    retain their initialized parser category and exact nonwrapping offset. -/
def HeaderProjectionAPIErrorOwned (result : GoInterface) : HeaderProjectionAPIError → IProp GF
  | .nonzero => iprop(∃ (category : GoInterface) (err : Loc), globalAddr cborerr.ErrUnsupported ↦□ category ∗
      err ↦ {(zero_val cborerr.DecodeError) with Kind' := category, Detail' := go!"the destination is not the zero value"} ∗
      ⌜result = interface.mkOk (go.PointerType cborerr.DecodeError.ty) #err⌝)
  | .decode offset reason => iprop(∃ kinds : FeltErrorKinds,
      FeltErrorGlobals kinds ∗ HeaderProjectionRecordError result (W64 offset) reason kinds ∗
      ⌜sint.Z (W64 offset) = offset⌝)

/-- Success returns an exact slice of the original input, including its backing
    pointer and capacity, rather than a copy with equal bytes. Errors return nil. -/
def HeaderProjectionFirstResultOwned (kind : HeaderProjectionKind) (dst : Loc)
    (input remaining : GoSlice) (wire : List w8) (result : GoInterface) : HeaderProjectionFirstResult → IProp GF
  | .success final rest => iprop(HeaderProjectionOwned kind dst final ∗ ∃ consumed : Nat,
      ⌜result = interface.nil ∧ remaining = slice.slice input w8 (W64 consumed) input.len ∧
        sint.Z (W64 consumed) = consumed ∧ consumed + rest.length = wire.length ∧
        sint.Z remaining.len = rest.length ∧ subslice (sint.nat (W64 consumed)) (sint.nat input.len) wire = rest⌝)
  | .failure final error => iprop(HeaderProjectionOwned kind dst final ∗
      HeaderProjectionAPIErrorOwned result error ∗ ⌜remaining = slice.nil⌝)

end github_com.NethermindEth.juno.core
end Perennial
