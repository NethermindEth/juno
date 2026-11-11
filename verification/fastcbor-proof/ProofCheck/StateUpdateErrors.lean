import ProofCheck.StateDiffErrors
import ProofCheck.StateUpdateBoundary

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

/-- A failed StateDiff pointer includes its private owned partial record, while
    the original outer field remains untouched. Scalar felt errors need no child. -/
def StateUpdateFieldFailureOwned (err : Loc) (pos : w64) : StateUpdateFieldFailure → IProp GF
  | .felt reason => iprop(∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
      FeltFailureError err pos reason (feltFailureKind reason kinds))
  | .diff retained reason => iprop((∃ temporary : Loc, StateDiffOwned temporary retained) ∗
      StateDiffFailureOwned err pos reason)

def StateUpdateFailureError (err : Loc) (pos : w64) (reason : StateUpdateFailureReason) (kinds : FeltErrorKinds) : IProp GF :=
  match reason with
  | .opening reason => FeltFailureError err pos reason (feltFailureKind reason kinds)
  | .key reason => KeyFailureError err pos reason (keyFailureKind reason kinds)
  | .duplicate field => iprop(∃ args : GoSlice,
      err ↦ cursor.DecodeError.mk kinds.unsupportedKind pos (go!"field %s named twice") args ∗
      args ↦* [interface.mkOk go.string #(stateUpdateFieldName field)])
  | .value (.field reason) => StateUpdateFieldFailureOwned err pos reason
  | .value (.ignored reason) => SkipFailureError err pos reason (skipFailureKind reason kinds.toSkipErrorKinds)

def StateUpdateFailureOwned (err : Loc) (pos : w64) (reason : StateUpdateFailureReason) : IProp GF := iprop(
  ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗ StateUpdateFailureError err pos reason kinds)

theorem wp_readStateUpdate_open_rejected_owned (c dst : Loc) (old : StateUpdateValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat) (reason : FeltFailureReason)
    (failure : StructOpenFailure d wire reason) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗
       StateUpdateOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadStateUpdate)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       StateUpdateOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ StateUpdateFailureOwned err v.pos' (.opening reason) }} := by
  simpa only [StateUpdateFailureOwned, StateUpdateFailureError] using
    wp_readStateUpdate_open_rejected (GF := GF) c dst old v xs wire dq d reason failure hd hp hbytes

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readStateUpdate_open_rejected_owned
