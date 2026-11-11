import ProofCheck.KeyRejected
import ProofCheck.Containers

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

/-- An exhausted closed projection touches no cursor or field metadata. -/
theorem wp_nextWanted_done (c st fields : Loc) (next seen : w64) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       st ↦ cursor.Struct.mk fields (W64 0) next seen false }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextWanted")) (Val #st))
    {{ RET (PairV #(W64 (-1)) #interface.nil);
       st ↦ cursor.Struct.mk fields (W64 0) next seen false }} := by
  wp_start as Hst
  wp_auto
  wp_for
  wp_end

/-- Finishing an open projection closes precisely one container and preserves
    the cursor position, field mask and search hint. -/
theorem wp_nextWanted_close (c st fields : Loc) (v : cursor.Cursor) (next seen : w64) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       st ↦ cursor.Struct.mk fields (W64 0) next seen true ∗ c ↦ {v with depth' := v.depth' + W64 1} }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextWanted")) (Val #st))
    {{ RET (PairV #(W64 (-1)) #interface.nil);
       st ↦ cursor.Struct.mk fields (W64 0) next seen false ∗ c ↦ v }} := by
  wp_start as ⟨Hst, Hc⟩
  wp_auto
  wp_for
  wp_apply wp_leave c v $$ [Hc] with Hc
  · iframe Hc
  wp_end

/-- A rejected key returns immediately without decrementing the current pair
    or closing the container. No metadata is read on this branch. -/
theorem wp_nextWanted_key_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c st : Loc) (v : cursor.Cursor) (s : cursor.Struct) (xs wire : List w8) (dq : DFrac)
    (reason : KeyFailureReason) (kind : GoInterface) (failure : KeyRejected wire reason)
    (left : 0 < sint.Z s.left')
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       keyFailureAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ st ↦ s ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextWanted")) (Val #st))
    {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ st ↦ s ∗ v.buf' ↦*{dq} xs ∗ KeyFailureError err v.pos' reason kind }} := by
  wp_start as ⟨#Hkind, Hc, Hst, Hs⟩
  wp_auto
  wp_for
  have hz : sint.Z (W64 0) = 0 := rfl
  simp only [hz, left, _root_.decide_true, ite_true]
  wp_auto
  wp_apply wp_Key_rejected validator c v xs wire dq reason kind failure hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_for_post
  iapply HΦ
  iframe Hc Hst Hs Herr

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextWanted_done
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextWanted_close
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextWanted_key_rejected
