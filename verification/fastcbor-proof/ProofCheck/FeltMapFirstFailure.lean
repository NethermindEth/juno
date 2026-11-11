import ProofCheck.FeltMapReserveFailure
import ProofCheck.FeltMapFirstFailureSpec

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

/-- The ordinary map wrapper starts from an empty reserve. A first nonnull
    value failure therefore includes real reserve growth, with no destination
    publication and the exact independent map diagnostic. -/
theorem wp_readFeltMap_first_value_failure (c dst old : Loc) (count : Nat)
    (v : cursor.Cursor) (xs payload keyItem wire : List w8)
    (head : w8) (key : FeltMapKey) (dq : DFrac) (d offset : Nat) (reason : FeltFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (positive : 0 < count) (fit : 2 * count ≤ (keyItem ++ wire).length) (countBound : count < 2^63)
    (valid : FeltMapKeyValid (d + 1) keyItem key)
    (failure : FeltFailure (d + 1) wire offset reason)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ keyItem ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltMap)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + keyItem.length), depth' := v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs ∗ FeltMapFailureOwned err (v.pos' + W64 (1 + payload.length + keyItem.length + offset)) (.element (.value reason)) ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length)) = sint.Z v.pos' + 1 + payload.length + keyItem.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length + offset)) = sint.Z v.pos' + 1 + payload.length + keyItem.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length + offset)) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  wp_auto
  ihave Hreserve : (FeltReserve slice.nil 0 : IProp GF) $$ []
  · iapply feltReserve_nil
  wp_apply wp_readFeltMapFrom_first_value_failure c dst old spare_ptr slice.nil 0 count 0 v xs payload keyItem wire
    head key dq d offset reason enc major positive fit (by simpa using countBound) valid failure hd depth hp hbytes
    $$ [spare Hreserve Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr, Hprivate, %Hnext⟩
  · iframe spare Hreserve Hdst Hc Hs
  iapply HΦ
  iframe Hdst Hc Hs Herr
  ipureintro; exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFeltMap_first_value_failure
