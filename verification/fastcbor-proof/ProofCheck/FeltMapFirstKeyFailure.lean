import ProofCheck.FeltMapFromFirstKeyFailure
import ProofCheck.StateMapFirstKeyFailureSpec

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

/-- A first key error returns from the actual allocated map loop before value
    decoding or insertion. The old destination and any shared reserve are untouched. -/
theorem wp_readFeltMap_first_key_failure (c dst old : Loc) (count : Nat)
    (v : cursor.Cursor) (xs payload wire : List w8) (head : w8) (dq : DFrac) (d offset : Nat) (reason : FeltFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (positive : 0 < count) (countBound : count < 2^63) (fit : 2 * count ≤ wire.length)
    (failure : LimbsFailure (d + 1) wire offset reason)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltMap)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ {v with pos' := v.pos' + W64 (1 + payload.length), depth' := v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs ∗ FeltMapFailureOwned err (v.pos' + W64 (1 + payload.length + offset)) (.element (.key reason)) ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) = sint.Z v.pos' + 1 + payload.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_readFeltMapFrom_first_key_failure c dst old spare_ptr slice.nil (W64 0) count v xs payload wire
    head dq d offset reason enc major positive countBound fit failure hd depth hp hbytes
    $$ [spare Hdst Hc Hs] as %err ⟨Hspare, Hdst, Hc, Hs, Herr, %Hnext⟩
  · iframe spare Hdst Hc Hs
  iapply HΦ
  iframe Hdst Hc Hs Herr
  ipureintro; exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFeltMap_first_key_failure
