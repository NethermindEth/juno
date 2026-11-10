import ProofCheck.ExpectedFailure

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

/-- Public in-place Limbs propagates a proved child failure through the actual
    shortcut/slow dispatch, preserving the caller's existing four-word array. -/
theorem wp_limbs_prefix_reject (c dst : Loc) (old : GoArray w64 4) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (count : w64) (items : List UnsignedWire)
    (dq : DFrac) (R : IProp GF) (E : Loc → IProp GF)
    (enc : HeadArgument head payload count) (hm : head &&& W8 224 = W8 128)
    (reject : UnsignedReject rem) (hprefix : items.length < 4) (hcount : items.length < uint.Z count)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hdepth : sint.Z v.depth' < 32)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ unsignedBytes items ++ rem)
    (hbad :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         R ∗ c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (unsignedBytes items).length), depth' := v.depth' + W64 1} ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Uint64")) (Val #()))
      {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
         c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (unsignedBytes items).length), depth' := v.depth' + W64 1} ∗ v.buf' ↦*{dq} xs ∗ E err }}) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       R ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Limbs")) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ E err }} := by
  wp_start_folded as ⟨HR, Hdst, Hc, Hs⟩
  by_cases heq : head = W8 132
  · subst head
    obtain ⟨hpay, hc⟩ := headArgument_inline_inv (W8 132) payload count enc (by decide)
    subst payload
    have hv : count = W64 4 := hc
    subst count
    wp_apply wp_limbs_four_reject c dst old v xs rem items dq R E
      hprefix reject hp hdepth hbytes
      (wp_limbsSlow_prefix_reject c dst old v xs [] rem (W8 132) (W64 4) items
        dq R E enc hm hprefix hcount hp hdepth hbytes hbad)
      $$ [HR Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe HR Hdst Hc Hs
    wp_end
  · wp_apply wp_limbs_nonfast_reject c dst old v xs
      (payload ++ unsignedBytes items ++ rem) head dq R E heq hp hdepth hbytes
      (wp_limbsSlow_prefix_reject c dst old v xs payload rem head count items
        dq R E enc hm hprefix hcount hp hdepth hbytes hbad)
      $$ [HR Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe HR Hdst Hc Hs
    wp_end

/-- Public in-place Limbs propagates a proved child failure through the actual
    shortcut/slow dispatch, preserving the caller's existing four-word array. -/
theorem wp_limbs_extras_reject (c dst : Loc) (old : GoArray w64 4) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (count : w64)
    (items : List UnsignedWire) (extras : List (List w8))
    (dq : DFrac) (R : IProp GF) (E : Loc → IProp GF)
    (d : Nat) (hd : sint.Z v.depth' = d) (hdepth : d < 32)
    (enc : HeadArgument head payload count) (hm : head &&& W8 224 = W8 128)
    (hcount : items.length = 4) (hmore : 4 + extras.length < uint.Z count)
    (valid : ∀ item ∈ extras, SkipValid (d + 1) item)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ unsignedBytes items ++ extras.flatten ++ rem)
    (hbad :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         R ∗ c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (unsignedBytes items).length + extras.flatten.length), depth' := v.depth' + W64 1} ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
      {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
         c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (unsignedBytes items).length + extras.flatten.length), depth' := v.depth' + W64 1} ∗ v.buf' ↦*{dq} xs ∗ E err }}) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       R ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Limbs")) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ E err }} := by
  wp_start_folded as ⟨HR, Hdst, Hc, Hs⟩
  have hne : head ≠ W8 132 := by
    intro heq
    obtain ⟨_, hc⟩ := headArgument_inline_inv head payload count enc (by rw [heq]; decide)
    rw [heq] at hc
    have hv : count = W64 4 := hc
    rw [hv] at hmore
    change (4 : Int) + extras.length < 4 at hmore
    omega
  wp_apply wp_limbs_nonfast_reject c dst old v xs
    (payload ++ unsignedBytes items ++ extras.flatten ++ rem) head dq R E hne hp (by omega) hbytes
    (wp_limbsSlow_extras_reject c dst old v xs payload rem head count items extras
      dq R E d hd hdepth enc hm hcount hmore valid hp hbytes hbad)
    $$ [HR Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
  · iframe HR Hdst Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbs_prefix_reject
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbs_extras_reject
