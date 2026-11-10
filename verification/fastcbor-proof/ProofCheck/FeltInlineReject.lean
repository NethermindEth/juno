import ProofCheck.FeltWideReject

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 3200000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- A rejected unsigned prefix bypasses the wide fast path and is declined by
    the actual mixed-width shortcut, before propagating the proved slow error. -/
theorem wp_limbs_four_reject (c dst : Loc) (old : GoArray w64 4) (v : cursor.Cursor)
    (xs rem : List w8) (items : List UnsignedWire) (dq : DFrac) (R : IProp GF) (E : Loc → IProp GF)
    (hcount : items.length < 4) (reject : UnsignedReject rem)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hdepth : sint.Z v.depth' < 32)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = W8 132 :: (unsignedBytes items ++ rem))
    (hslow :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         R ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"limbsSlow")) (Val #dst))
      {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
         dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ E err }}) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       R ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Limbs")) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ E err }} := by
  wp_start as ⟨HR, Hdst, Hc, Hs⟩
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have hwidth := unsignedBytes_bound items
  have hnd : ¬ sint.Z (W64 32) ≤ sint.Z v.depth' := by change ¬ 32 ≤ sint.Z v.depth'; omega
  wp_auto
  simp only [hnd, _root_.decide_false]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp.1, hp.2, Hwf.2⟩)]
  icases (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs
      ⟨hp.1, hp.2, Int.le_refl _⟩).1 $$ Hs with ⟨Hpre, Hbuf, Hpost⟩
  isimp only [hbytes] at Hbuf
  ihave %Hbuflen := ownSlice_len _ _ _ $$ Hbuf
  wp_auto
  by_cases hlong : sint.Z (W64 37) ≤ sint.Z (v.buf'.len - v.pos')
  · simp only [hlong, _root_.decide_true]
    wp_auto
    rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 0) < sint.Z (v.buf'.len - v.pos'); word⟩)]
    wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _
        (W8 132 :: (unsignedBytes items ++ rem)) dq (W8 132) (by decide) $$ [Hbuf] with Hbuf
    · iframe Hbuf; ipureintro; rfl
    have hblen : 37 ≤ (W8 132 :: (unsignedBytes items ++ rem)).length := by
      have heq := Hbuflen.1
      change (W8 132 :: (unsignedBytes items ++ rem)).length = (sint.Z (v.buf'.len - v.pos')).toNat at heq
      change (37 : Int) ≤ sint.Z (v.buf'.len - v.pos') at hlong
      omega
    list_elem (W8 132 :: (unsignedBytes items ++ rem)) 1 as byte1
    rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 1) < sint.Z (v.buf'.len - v.pos'); word⟩)]
    wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _
        (W8 132 :: (unsignedBytes items ++ rem)) dq byte1 (by decide) $$ [Hbuf] with Hbuf
    · iframe Hbuf; ipureintro; exact Hbyte1_lookup
    by_cases heq1 : byte1 = W8 27
    · have hg1 : (W8 132 :: (unsignedBytes items ++ rem))[1]? = some (W8 27) := by simpa only [heq1] using Hbyte1_lookup
      simp only [heq1, _root_.decide_true]
      wp_auto
      list_elem (W8 132 :: (unsignedBytes items ++ rem)) 10 as byte10
      rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 10) < sint.Z (v.buf'.len - v.pos'); word⟩)]
      wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _
          (W8 132 :: (unsignedBytes items ++ rem)) dq byte10 (by decide) $$ [Hbuf] with Hbuf
      · iframe Hbuf; ipureintro; exact Hbyte10_lookup
      by_cases heq10 : byte10 = W8 27
      · have hg10 : (W8 132 :: (unsignedBytes items ++ rem))[10]? = some (W8 27) := by simpa only [heq10] using Hbyte10_lookup
        simp only [heq10, _root_.decide_true]
        wp_auto
        list_elem (W8 132 :: (unsignedBytes items ++ rem)) 19 as byte19
        rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 19) < sint.Z (v.buf'.len - v.pos'); word⟩)]
        wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _
            (W8 132 :: (unsignedBytes items ++ rem)) dq byte19 (by decide) $$ [Hbuf] with Hbuf
        · iframe Hbuf; ipureintro; exact Hbyte19_lookup
        by_cases heq19 : byte19 = W8 27
        · have hg19 : (W8 132 :: (unsignedBytes items ++ rem))[19]? = some (W8 27) := by simpa only [heq19] using Hbyte19_lookup
          simp only [heq19, _root_.decide_true]
          wp_auto
          list_elem (W8 132 :: (unsignedBytes items ++ rem)) 28 as byte28
          rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 28) < sint.Z (v.buf'.len - v.pos'); word⟩)]
          wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _
              (W8 132 :: (unsignedBytes items ++ rem)) dq byte28 (by decide) $$ [Hbuf] with Hbuf
          · iframe Hbuf; ipureintro; exact Hbyte28_lookup
          by_cases heq28 : byte28 = W8 27
          · have hg28 : (W8 132 :: (unsignedBytes items ++ rem))[28]? = some (W8 27) := by simpa only [heq28] using Hbyte28_lookup
            simp only [heq28, _root_.decide_true]
            wp_auto
            exfalso
            exact rejected_felt_not_wide items rem hcount reject hblen ⟨hg1, hg10, hg19, hg28⟩
          · simp only [heq28, _root_.decide_false]
            wp_auto
            wp_apply wp_shortLimbs_prefix_reject dst old (slice.slice v.buf' w8 v.pos' v.buf'.len) items rem dq hcount reject
                $$ [Hdst Hbuf] with ⟨Hdst, Hbuf⟩
            · iframe Hdst Hbuf
            ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
            · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
              rw [hbytes]
              iframe Hpre Hbuf Hpost
            wp_apply hslow $$ [HR Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
            · iframe HR Hdst Hc Hs
            wp_end
        · simp only [heq19, _root_.decide_false]
          wp_auto
          wp_apply wp_shortLimbs_prefix_reject dst old (slice.slice v.buf' w8 v.pos' v.buf'.len) items rem dq hcount reject
              $$ [Hdst Hbuf] with ⟨Hdst, Hbuf⟩
          · iframe Hdst Hbuf
          ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
          · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
            rw [hbytes]
            iframe Hpre Hbuf Hpost
          wp_apply hslow $$ [HR Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
          · iframe HR Hdst Hc Hs
          wp_end
      · simp only [heq10, _root_.decide_false]
        wp_auto
        wp_apply wp_shortLimbs_prefix_reject dst old (slice.slice v.buf' w8 v.pos' v.buf'.len) items rem dq hcount reject
            $$ [Hdst Hbuf] with ⟨Hdst, Hbuf⟩
        · iframe Hdst Hbuf
        ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
        · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
          rw [hbytes]
          iframe Hpre Hbuf Hpost
        wp_apply hslow $$ [HR Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
        · iframe HR Hdst Hc Hs
        wp_end
    · simp only [heq1, _root_.decide_false]
      wp_auto
      wp_apply wp_shortLimbs_prefix_reject dst old (slice.slice v.buf' w8 v.pos' v.buf'.len) items rem dq hcount reject
          $$ [Hdst Hbuf] with ⟨Hdst, Hbuf⟩
      · iframe Hdst Hbuf
      ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
      · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
        rw [hbytes]
        iframe Hpre Hbuf Hpost
      wp_apply hslow $$ [HR Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
      · iframe HR Hdst Hc Hs
      wp_end
  · simp only [hlong, _root_.decide_false]
    wp_auto
    wp_apply wp_shortLimbs_prefix_reject dst old (slice.slice v.buf' w8 v.pos' v.buf'.len) items rem dq hcount reject
        $$ [Hdst Hbuf] with ⟨Hdst, Hbuf⟩
    · iframe Hdst Hbuf
    ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
    · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
      rw [hbytes]
      iframe Hpre Hbuf Hpost
    wp_apply hslow $$ [HR Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe HR Hdst Hc Hs
    wp_end

/-- Every array-header width, including the inline four-element header, routes
    a failed unsigned prefix to the actual slow-loop rollback proof. -/
theorem wp_readFelt_prefix_reject (c dst : Loc) (old : Loc) (v : cursor.Cursor)
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
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       R ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ E err }} := by
  wp_start as ⟨HR, Hdst, Hc, Hs⟩
  have hi := input_suffix_head xs (payload ++ unsignedBytes items ++ rem)
    (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hn : head ≠ W8 246 := by intro h; subst head; contradiction
  have hu : head ≠ W8 247 := by intro h; subst head; contradiction
  wp_auto
  wp_apply wp_null_other c v xs head dq hp.1 hi hn hu $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  by_cases heq : head = W8 132
  · subst head
    obtain ⟨hpay, hc⟩ := headArgument_inline_inv (W8 132) payload count enc (by decide)
    subst payload
    have hv : count = W64 4 := hc
    subst count
    wp_apply wp_limbs_four_reject c «$r0_ptr» (zero_val (GoArray w64 4)) v xs rem items dq R E
      hprefix reject hp hdepth hbytes
      (wp_limbsSlow_prefix_reject c «$r0_ptr» (zero_val (GoArray w64 4)) v xs [] rem (W8 132) (W64 4) items
        dq R E enc hm hprefix hcount hp hdepth hbytes hbad)
      $$ [HR «$r0» Hc Hs] as %err ⟨Hv, Hc, Hs, Herr⟩
    · iframe HR «$r0» Hc Hs
    wp_end
  · wp_apply wp_limbs_nonfast_reject c «$r0_ptr» (zero_val (GoArray w64 4)) v xs
      (payload ++ unsignedBytes items ++ rem) head dq R E heq hp hdepth hbytes
      (wp_limbsSlow_prefix_reject c «$r0_ptr» (zero_val (GoArray w64 4)) v xs payload rem head count items
        dq R E enc hm hprefix hcount hp hdepth hbytes hbad)
      $$ [HR «$r0» Hc Hs] as %err ⟨Hv, Hc, Hs, Herr⟩
    · iframe HR «$r0» Hc Hs
    wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbs_four_reject
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_prefix_reject
