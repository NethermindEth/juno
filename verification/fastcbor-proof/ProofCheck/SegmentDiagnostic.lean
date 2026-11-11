import ProofCheck.SegmentBounds
import ProofCheck.SegmentOwnership
import ProofCheck.InitializationErrors
import ProofCheck.StructOpenSound
import ProofCheck.KeyFailureSound

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

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

/- Recursive error ownership follows the finite diagnostic path. Enclosing
   readers propagate the same allocated error without wrapping or relocating it. -/
mutual
def SegmentFieldError (result : GoInterface) (pos : w64) (reason : SegmentFieldFailure) : IProp GF :=
  match reason with
  | .children reason => iprop(∃ err : Loc, ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      SegmentArrayFailureOwned err pos reason)
  | .scalar reason => iprop(∃ err : Loc, ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
      ExpectedFailureError err pos (W8 0) reason (expectedFailureKind reason kinds))
termination_by sizeOf reason

def SegmentFailureError (result : GoInterface) (pos : w64) (reason : SegmentFailureReason) (kinds : FeltErrorKinds) : IProp GF :=
  match reason with
  | .opening reason => iprop(∃ err : Loc, ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      FeltFailureError err pos reason (feltFailureKind reason kinds))
  | .key reason => iprop(∃ err : Loc, ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      KeyFailureError err pos reason (keyFailureKind reason kinds))
  | .duplicate field => iprop(∃ (err : Loc) (args : GoSlice),
      ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      err ↦ cursor.DecodeError.mk kinds.unsupportedKind pos (go!"field %s named twice") args ∗
      args ↦* [interface.mkOk go.string #(segmentFieldName field)])
  | .value (.field reason) => SegmentFieldError result pos reason
  | .value (.ignored reason) => iprop(∃ err : Loc, ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      SkipFailureError err pos reason (skipFailureKind reason kinds.toSkipErrorKinds))
termination_by sizeOf reason

def SegmentArrayFailureOwned (err : Loc) (pos : w64) (reason : SegmentArrayFailureReason) : IProp GF :=
  match reason with
  | .expected reason => iprop(∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
      ExpectedFailureError err pos (W8 128) reason (expectedFailureKind reason kinds))
  | .count => iprop(∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
      err ↦ cursor.DecodeError.mk kinds.truncatedKind pos (go!"count exceeds the input") slice.nil)
  | .depth => iprop(∃ (kinds : FeltErrorKinds) (args : GoSlice), FeltErrorGlobals kinds ∗
      err ↦ cursor.DecodeError.mk kinds.depthKind pos (go!"nested more than %d levels") args ∗
      args ↦* [interface.mkOk go.int #(W64 32)])
  | .element reason => iprop(∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
      SegmentFailureError (interface.mkOk (go.PointerType cursor.DecodeError.ty) #err) pos reason kinds)
termination_by sizeOf reason
end

def SegmentFailureOwned (result : GoInterface) (pos : w64) (reason : SegmentFailureReason) : IProp GF := iprop(
  ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗ SegmentFailureError result pos reason kinds)

def SegmentDiagnosticOwned (err : Loc) (pos : w64) (reason : SegmentFailureReason) : IProp GF :=
  SegmentFailureOwned (interface.mkOk (go.PointerType cursor.DecodeError.ty) #err) pos reason

/-- Array/record propagation preserves the very same error object and position. -/
theorem segmentArrayFailureOwned_element (err : Loc) (pos : w64) (reason : SegmentFailureReason) :
    SegmentArrayFailureOwned (GF := GF) err pos (.element reason) = SegmentDiagnosticOwned err pos reason := by
  rw [SegmentArrayFailureOwned]
  rfl

theorem segmentFieldError_allocated (result : GoInterface) (pos : w64) (reason : SegmentFieldFailure) :
    SegmentFieldError (GF := GF) result pos reason ⊢
      ⌜∃ err : Loc, result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ := by
  cases reason <;> rw [SegmentFieldError]
  all_goals
    iintro ⟨%err, %Heq, _⟩
    ipureintro; exact ⟨err, Heq⟩

theorem segmentFieldError_nonNil (result : GoInterface) (pos : w64) (reason : SegmentFieldFailure) :
    SegmentFieldError (GF := GF) result pos reason ⊢ ⌜result ≠ interface.nil⌝ := by
  iintro Herr
  ihave %Hallocated := segmentFieldError_allocated result pos reason $$ Herr
  obtain ⟨err, rfl⟩ := Hallocated
  ipureintro; simp

theorem segmentFailureOwned_allocated (result : GoInterface) (pos : w64) (reason : SegmentFailureReason) :
    SegmentFailureOwned (GF := GF) result pos reason ⊢
      ⌜∃ err : Loc, result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ := by
  isimp only [SegmentFailureOwned]
  iintro ⟨%kinds, _, Herr⟩
  cases reason with
  | opening | key =>
    isimp only [SegmentFailureError] at Herr
    icases Herr with ⟨%err, %Heq, _⟩
    ipureintro; exact ⟨err, Heq⟩
  | duplicate =>
    isimp only [SegmentFailureError] at Herr
    icases Herr with ⟨%err, %args, %Heq, _⟩
    ipureintro; exact ⟨err, Heq⟩
  | value reason => cases reason with
    | field reason =>
      isimp only [SegmentFailureError] at Herr
      iapply segmentFieldError_allocated result pos reason
      iexact Herr
    | ignored =>
      isimp only [SegmentFailureError] at Herr
      icases Herr with ⟨%err, %Heq, _⟩
      ipureintro; exact ⟨err, Heq⟩

theorem segmentFailureOwned_nonNil (result : GoInterface) (pos : w64) (reason : SegmentFailureReason) :
    SegmentFailureOwned (GF := GF) result pos reason ⊢ ⌜result ≠ interface.nil⌝ := by
  iintro Herr
  ihave %Hallocated := segmentFailureOwned_allocated result pos reason $$ Herr
  obtain ⟨err, rfl⟩ := Hallocated
  ipureintro; simp

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentArrayFailureOwned_element
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentFieldError_allocated
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentFieldError_nonNil
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentFailureOwned_allocated
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentFailureOwned_nonNil
