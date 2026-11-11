import ProofCheck.ResourceMapBoundary
import ProofCheck.ResourceMapSpec
import ProofCheck.DAModeComplete
import ProofCheck.ProjectionTagSuccess
import ProofCheck.ResourceBoundsRecordComplete
import ProofCheck.ResourceMapEntriesSpec

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 4096

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

local instance (kinds : FeltErrorKinds) : Persistent (FeltErrorGlobals (GF := GF) kinds) := by
  unfold FeltErrorGlobals SkipErrorGlobals
  infer_instance

/-- Key nullish/overflow errors are actual initialized bare sentinels. The
    position parameter is used only when the returned error has a diagnostic. -/
def ResourceMapEntryError (result : GoInterface) (pos : w64) : ResourceMapEntryFailureReason → IProp GF
  | .key .nullish => iprop(∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
      ⌜result = kinds.unsupportedKind ∧ result ≠ interface.nil⌝)
  | .key (.numeric reason) => iprop(∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗ DAModeError result pos kinds reason)
  | .value reason => iprop(∃ err : Loc,
      ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗ ResourceBoundsFailureOwned err pos reason)

/-- Completed entries and the failed value remain private. A key rejection
    occurs before allocation of the temporary value, represented by zero only
    in the independent failure relation. -/
def ResourceMapPartial (done : List (w32 × ResourceBoundsValue)) (retained : ResourceBoundsValue)
    (reason : ResourceMapEntryFailureReason) : IProp GF := iprop(
  (∃ out : Loc, ResourceMapOwned out (resourceMapOfEntries done)) ∗
  match reason with
  | .key _ => ⌜retained = resourceBoundsZero⌝
  | .value _ => ∃ failed : Loc, ResourceBoundsOwned failed retained)

def ResourceMapFailureOwned (result : GoInterface) (pos : w64) : ResourceMapFailureReason → IProp GF
  | .expected reason => iprop(∃ (err : Loc) (kinds : FeltErrorKinds),
      ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗ FeltErrorGlobals kinds ∗
      ExpectedFailureError err pos (W8 160) reason (expectedFailureKind reason kinds))
  | .count => iprop(∃ (err : Loc) (kinds : FeltErrorKinds),
      ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗ FeltErrorGlobals kinds ∗
      err ↦ cursor.DecodeError.mk kinds.truncatedKind pos (go!"count exceeds the input") slice.nil)
  | .depth => iprop(∃ (err : Loc) (kinds : FeltErrorKinds) (args : GoSlice),
      ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗ FeltErrorGlobals kinds ∗
      err ↦ cursor.DecodeError.mk kinds.depthKind pos (go!"nested more than %d levels") args ∗
      args ↦* [interface.mkOk go.int #(W64 32)])
  | .element reason => ResourceMapEntryError result pos reason

theorem resourceMapEntryError_nonNil (result : GoInterface) (pos : w64) (reason : ResourceMapEntryFailureReason) :
    ResourceMapEntryError (GF := GF) result pos reason ⊢ ⌜result ≠ interface.nil⌝ := by
  iintro Herr
  cases reason with
  | key reason =>
    cases reason with
    | nullish =>
      isimp only [ResourceMapEntryError] at Herr
      icases Herr with ⟨%kinds, _, %Hresult⟩
      ipureintro; exact Hresult.2
    | numeric reason =>
      isimp only [ResourceMapEntryError] at Herr
      icases Herr with ⟨%kinds, _, Herr⟩
      iapply daModeError_nonNil $$ Herr
  | value reason =>
    isimp only [ResourceMapEntryError] at Herr
    icases Herr with ⟨%err, %Hresult, _⟩
    ipureintro; rw [Hresult]; simp

theorem resourceMapFailureOwned_nonNil (result : GoInterface) (pos : w64) (reason : ResourceMapFailureReason) :
    ResourceMapFailureOwned (GF := GF) result pos reason ⊢ ⌜result ≠ interface.nil⌝ := by
  iintro Herr
  cases reason with
  | expected reason =>
    isimp only [ResourceMapFailureOwned] at Herr
    icases Herr with ⟨%err, %kinds, %Hresult, _⟩
    ipureintro; rw [Hresult]; simp
  | count =>
    isimp only [ResourceMapFailureOwned] at Herr
    icases Herr with ⟨%err, %kinds, %Hresult, _⟩
    ipureintro; rw [Hresult]; simp
  | depth =>
    isimp only [ResourceMapFailureOwned] at Herr
    icases Herr with ⟨%err, %kinds, %args, %Hresult, _⟩
    ipureintro; rw [Hresult]; simp
  | element reason =>
    isimp only [ResourceMapFailureOwned] at Herr
    iapply resourceMapEntryError_nonNil $$ Herr

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMapEntryError_nonNil
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMapFailureOwned_nonNil
