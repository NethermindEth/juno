import ProofCheck.ClassResultSpec
import ProofCheck.ClassDispatchBoundary
import ProofCheck.ClassDeprecatedDispatch
import ProofCheck.ClassSierraDispatch

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

def ClassValueOwned (out : GoInterface) : ClassValue → IProp GF
  | .nullish => iprop(⌜out = interface.nil⌝)
  | .deprecated value => iprop(∃ ptr : Loc,
      ⌜out = interface.mkOk (go.PointerType DeprecatedCairoClass.ty) #ptr⌝ ∗ DeprecatedClassOwned ptr value)
  | .sierra value => iprop(∃ ptr : Loc,
      ⌜out = interface.mkOk (go.PointerType SierraClass.ty) #ptr⌝ ∗ SierraClassOwned ptr value)

/-- Failed record allocations remain owned and private. Error positions refer
    to actual diagnostics; bare sentinel branches intentionally ignore them. -/
def ClassErrorOwned (result : GoInterface) (pos : w64) : ClassDispatchResult → IProp GF
  | .tagFailure reason => iprop(∃ (err : Loc) (kinds : FeltErrorKinds),
      ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗ FeltErrorGlobals kinds ∗
      ExpectedFailureError err pos (W8 192) reason (expectedFailureKind reason kinds))
  | .unknown _ _ _ => iprop(∃ kinds : FeltErrorKinds,
      FeltErrorGlobals kinds ∗ ⌜result = kinds.unsupportedKind ∧ result ≠ interface.nil⌝)
  | .payload _ (.deprecated (.failure retained _ _ _ reason)) => iprop(
      (∃ temporary : Loc, DeprecatedClassOwned temporary retained) ∗
      DeprecatedClassFailureOwned result pos reason)
  | .payload _ (.sierra (.failure retained _ _ _ reason)) => iprop(
      (∃ temporary : Loc, SierraClassOwned temporary retained) ∗
      ∃ err : Loc, ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗ SierraClassFailureOwned err pos reason)
  | _ => iprop(False)

theorem classErrorOwned_nonNil (result : GoInterface) (pos : w64) (detail : ClassDispatchResult) :
    ClassErrorOwned (GF := GF) result pos detail ⊢ ⌜result ≠ interface.nil⌝ := by
  cases detail with
  | nullish => isimp only [ClassErrorOwned]; iintro %H; exact False.elim H
  | tagFailure reason =>
    isimp only [ClassErrorOwned]
    iintro ⟨%err, %kinds, %Hresult, _⟩
    ipureintro; rw [Hresult]; simp
  | unknown =>
    isimp only [ClassErrorOwned]
    iintro ⟨%kinds, _, %Hresult⟩
    ipureintro; exact Hresult.2
  | payload tagConsumed value =>
    cases value with
    | deprecated value =>
      cases value with
      | success => isimp only [ClassErrorOwned]; iintro %H; exact False.elim H
      | failure retained consumed offset depth reason =>
        isimp only [ClassErrorOwned]
        iintro ⟨_, Herr⟩
        iapply deprecatedClassFailureOwned_nonNil result pos reason
        iexact Herr
    | sierra value =>
      cases value with
      | success => isimp only [ClassErrorOwned]; iintro %H; exact False.elim H
      | failure retained consumed offset depth reason =>
        isimp only [ClassErrorOwned]
        iintro ⟨_, %err, %Hresult, _⟩
        ipureintro; rw [Hresult]; simp

/-- Caller-facing state for the complete decoder: exact value and suffix on
    success, or original interface plus private partial state and exact error on
    failure. Fractional ownership of the input is returned separately. -/
def ClassReadOwned (c dst : Loc) (old : GoInterface) (v : cursor.Cursor) (xs wire : List w8)
    (result : GoInterface) : ClassResult → IProp GF
  | .success value rest => iprop(∃ (out : GoInterface) (consumed : Nat),
      dst ↦ out ∗ ClassValueOwned out value ∗ c ↦ {v with pos' := v.pos' + W64 consumed} ∗
      ⌜result = interface.nil ∧ consumed + rest.length = wire.length ∧
        sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len ∧
        subslice (sint.nat (v.pos' + W64 consumed)) (sint.nat v.buf'.len) xs = rest⌝)
  | .failure detail consumed offset extraDepth => iprop(dst ↦ old ∗
      c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
      ClassErrorOwned result (v.pos' + W64 offset) detail ∗
      ⌜result ≠ interface.nil ∧
        sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
        sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝)

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.classErrorOwned_nonNil
