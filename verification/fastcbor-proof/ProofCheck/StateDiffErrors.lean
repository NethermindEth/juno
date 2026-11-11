import ProofCheck.StateMapDiagnostics
import ProofCheck.StateDiffPointerBoundary
import ProofCheck.FieldKeySpec

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

local instance (kinds : FeltErrorKinds) : Persistent (FeltErrorGlobals (GF := GF) kinds) := by
  unfold FeltErrorGlobals SkipErrorGlobals
  infer_instance

def StateDiffFieldFailureOwned (err : Loc) (pos : w64) : StateDiffFieldFailure → IProp GF
  | .storage reason => StorageDiffsFailureOwned err pos reason
  | .map reason => FeltMapFailureOwned err pos reason
  | .felts reason => iprop(∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
      FeltFailureError err pos reason (feltFailureKind reason kinds))
  | .migrated reason => MigratedClassesFailureOwned err pos reason

def StateDiffFailureError (err : Loc) (pos : w64) (reason : StateDiffFailureReason) (kinds : FeltErrorKinds) : IProp GF :=
  match reason with
  | .opening reason => FeltFailureError err pos reason (feltFailureKind reason kinds)
  | .key reason => KeyFailureError err pos reason (keyFailureKind reason kinds)
  | .duplicate field => iprop(∃ args : GoSlice,
      err ↦ cursor.DecodeError.mk kinds.unsupportedKind pos (go!"field %s named twice") args ∗
      args ↦* [interface.mkOk go.string #(stateDiffFieldName field)])
  | .value (.field reason) => StateDiffFieldFailureOwned err pos reason
  | .value (.ignored reason) => SkipFailureError err pos reason (skipFailureKind reason kinds.toSkipErrorKinds)

def StateDiffFailureOwned (err : Loc) (pos : w64) (reason : StateDiffFailureReason) : IProp GF := iprop(
  ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗ StateDiffFailureError err pos reason kinds)

/-- Package the actual opening diagnostic in the record's uniform error contract. -/
theorem wp_readStateDiff_open_rejected_owned (c dst : Loc) (old : StateDiffValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat) (reason : FeltFailureReason)
    (failure : StructOpenFailure d wire reason) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadStateDiff)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ StateDiffFailureOwned err v.pos' (.opening reason) }} := by
  simpa only [StateDiffFailureOwned, StateDiffFailureError] using
    wp_readStateDiff_open_rejected (GF := GF) c dst old v xs wire dq d reason failure hd hp hbytes

/-- Failed pointer allocation stays private under the same record error contract. -/
theorem wp_readStateDiffPointer_open_rejected_owned (c dst old : Loc) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat) (reason : FeltFailureReason)
    (failure : StructOpenFailure d wire reason) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadStateDiffPointer)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ (∃ temporary : Loc, StateDiffOwned temporary stateDiffZero) ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ StateDiffFailureOwned err v.pos' (.opening reason) }} := by
  simpa only [StateDiffFailureOwned, StateDiffFailureError] using
    wp_readStateDiffPointer_open_rejected (GF := GF) c dst old v xs wire dq d reason failure hd hp hbytes

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readStateDiff_open_rejected_owned
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readStateDiffPointer_open_rejected_owned
