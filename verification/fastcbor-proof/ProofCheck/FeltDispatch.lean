import ProofCheck.FeltShortcut

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 1600000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]


/-- When a valid four-limb value misses a wide shape check, Limbs uses the mixed-width shortcut. -/
theorem wp_limbs_four_notwide (c dst : Loc) (old : GoArray w64 4) (v : cursor.Cursor)
    (xs rem : List w8) (items : List UnsignedWire) (dq : DFrac) (hcount : items.length = 4)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hdepth : sint.Z v.depth' < 32)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = W8 132 :: (unsignedBytes items ++ rem))
    (hguards : ¬ ((W8 132 :: (unsignedBytes items ++ rem))[1]? = some (W8 27) ∧
      (W8 132 :: (unsignedBytes items ++ rem))[10]? = some (W8 27) ∧
      (W8 132 :: (unsignedBytes items ++ rem))[19]? = some (W8 27) ∧
      (W8 132 :: (unsignedBytes items ++ rem))[28]? = some (W8 27))) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Limbs")) (Val #dst))
    {{ RET #interface.nil; dst ↦ paddedLimbs items ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + (unsignedBytes items).length)} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have hwidth := unsignedBytes_bound items
  have hsizepos : sint.Z (W64 0) < sint.Z (W64 (1 + (unsignedBytes items).length)) := by word
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
            exact hguards ⟨hg1, hg10, hg19, hg28⟩
          · simp only [heq28, _root_.decide_false]
            wp_auto
            wp_apply wp_shortLimbs_four dst old (slice.slice v.buf' w8 v.pos' v.buf'.len) items rem dq hcount
                $$ [Hdst Hbuf] with ⟨Hdst, Hbuf⟩
            · iframe Hdst Hbuf
            simp only [hsizepos, _root_.decide_true]
            wp_auto
            ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
            · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
              rw [hbytes]
              iframe Hpre Hbuf Hpost
            wp_end
        · simp only [heq19, _root_.decide_false]
          wp_auto
          wp_apply wp_shortLimbs_four dst old (slice.slice v.buf' w8 v.pos' v.buf'.len) items rem dq hcount
              $$ [Hdst Hbuf] with ⟨Hdst, Hbuf⟩
          · iframe Hdst Hbuf
          simp only [hsizepos, _root_.decide_true]
          wp_auto
          ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
          · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
            rw [hbytes]
            iframe Hpre Hbuf Hpost
          wp_end
      · simp only [heq10, _root_.decide_false]
        wp_auto
        wp_apply wp_shortLimbs_four dst old (slice.slice v.buf' w8 v.pos' v.buf'.len) items rem dq hcount
            $$ [Hdst Hbuf] with ⟨Hdst, Hbuf⟩
        · iframe Hdst Hbuf
        simp only [hsizepos, _root_.decide_true]
        wp_auto
        ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
        · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
          rw [hbytes]
          iframe Hpre Hbuf Hpost
        wp_end
    · simp only [heq1, _root_.decide_false]
      wp_auto
      wp_apply wp_shortLimbs_four dst old (slice.slice v.buf' w8 v.pos' v.buf'.len) items rem dq hcount
          $$ [Hdst Hbuf] with ⟨Hdst, Hbuf⟩
      · iframe Hdst Hbuf
      simp only [hsizepos, _root_.decide_true]
      wp_auto
      ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
      · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
        rw [hbytes]
        iframe Hpre Hbuf Hpost
      wp_end
  · simp only [hlong, _root_.decide_false]
    wp_auto
    wp_apply wp_shortLimbs_four dst old (slice.slice v.buf' w8 v.pos' v.buf'.len) items rem dq hcount
        $$ [Hdst Hbuf] with ⟨Hdst, Hbuf⟩
    · iframe Hdst Hbuf
    simp only [hsizepos, _root_.decide_true]
    wp_auto
    ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
    · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
      rw [hbytes]
      iframe Hpre Hbuf Hpost
    wp_end

/-- Every valid inline four-element array reaches a proved fast path, regardless of limb widths. -/
theorem wp_limbs_four (c dst : Loc) (old : GoArray w64 4) (v : cursor.Cursor)
    (xs rem : List w8) (items : List UnsignedWire) (dq : DFrac) (hcount : items.length = 4)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hdepth : sint.Z v.depth' < 32)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = W8 132 :: (unsignedBytes items ++ rem)) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Limbs")) (Val #dst))
    {{ RET #interface.nil; dst ↦ paddedLimbs items ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + (unsignedBytes items).length)} ∗ v.buf' ↦*{dq} xs }} := by
  obtain ⟨a, b, cc, d, rfl⟩ : ∃ a b cc d : UnsignedWire, items = [a, b, cc, d] :=
    ⟨items[0]'(by omega), items[1]'(by omega), items[2]'(by omega), items[3]'(by omega),
      List.eq_getElem_of_length_eq_four items hcount⟩
  by_cases hw : a.head = W8 27 ∧ b.head = W8 27 ∧ cc.head = W8 27 ∧ d.head = W8 27
  · obtain ⟨ha, hb, hc, hd⟩ := hw
    obtain ⟨hla, hva⟩ := unsignedWire_wide a ha
    obtain ⟨hlb, hvb⟩ := unsignedWire_wide b hb
    obtain ⟨hlc, hvc⟩ := unsignedWire_wide cc hc
    obtain ⟨hld, hvd⟩ := unsignedWire_wide d hd
    have hwire : W8 132 :: (unsignedBytes [a, b, cc, d] ++ rem) =
        wideFeltBytes a.payload b.payload cc.payload d.payload ++ rem := by
      simp [unsignedBytes, UnsignedWire.bytes, wideFeltBytes, ha, hb, hc, hd, List.append_assoc]
    have hsize : 1 + (unsignedBytes [a, b, cc, d]).length = 37 := by
      simp [unsignedBytes, UnsignedWire.bytes, hla, hlb, hlc, hld]
    have hvalue : paddedLimbs [a, b, cc, d] = wideFeltValue a.payload b.payload cc.payload d.payload := by
      simp [paddedLimbs, wideFeltValue, hva, hvb, hvc, hvd]
    iintro %Φ ⟨#Hpkg, Hdst, Hc, Hs⟩ HΦ
    ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    have hbound : sint.Z v.pos' + 37 ≤ sint.Z v.buf'.len := by
      have hlen := Hlen.1
      have hcount := congrArg List.length hbytes
      simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega),
        List.length_cons, List.length_append] at hcount
      simp only [sint.nat, sint.Z] at hlen hcount hp ⊢
      omega
    wp_apply wp_limbs_wide c dst old v xs a.payload b.payload cc.payload d.payload rem dq
      hla hlb hlc hld hp.1 hdepth hbound (hbytes.trans hwire) $$ [Hdst Hc Hs] with ⟨Hdst, Hc, Hs⟩
    · iframe Hdst Hc Hs
    have hsizeInt : (1 : Int) + (unsignedBytes [a, b, cc, d]).length = 37 := by omega
    isimp only [hvalue, hsizeInt] at HΦ
    iapply HΦ
    iframe Hdst Hc Hs
  · have hguards : ¬ ((W8 132 :: (unsignedBytes [a, b, cc, d] ++ rem))[1]? = some (W8 27) ∧
        (W8 132 :: (unsignedBytes [a, b, cc, d] ++ rem))[10]? = some (W8 27) ∧
        (W8 132 :: (unsignedBytes [a, b, cc, d] ++ rem))[19]? = some (W8 27) ∧
        (W8 132 :: (unsignedBytes [a, b, cc, d] ++ rem))[28]? = some (W8 27)) := by
      intro hg
      exact hw (four_wide_guards a b cc d rem hg.1 hg.2.1 hg.2.2.1 hg.2.2.2)
    exact wp_limbs_four_notwide c dst old v xs rem [a, b, cc, d] dq rfl hp hdepth hbytes hguards

/-- All successful arrays of at most four limbs, including non-preferred head and limb widths. -/
theorem wp_limbs_small (c dst : Loc) (old : GoArray w64 4) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (items : List UnsignedWire) (dq : DFrac)
    (enc : HeadArgument head payload (W64 items.length)) (hm : head &&& W8 224 = W8 128)
    (hcount : items.length ≤ 4)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hdepth : sint.Z v.depth' < 32)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ unsignedBytes items ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Limbs")) (Val #dst))
    {{ RET #interface.nil; dst ↦ paddedLimbs items ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (unsignedBytes items).length)} ∗
       v.buf' ↦*{dq} xs }} := by
  by_cases heq : head = W8 132
  · subst head
    obtain ⟨rfl, hv⟩ := headArgument_inline_inv (W8 132) payload (W64 items.length) enc (by decide)
    change W64 items.length = W64 4 at hv
    have hc : items.length = 4 := by word
    simpa only [List.length_nil, Int.natCast_zero, Int.add_zero] using
      wp_limbs_four c dst old v xs rem items dq hc hp hdepth hbytes
  · exact wp_limbs_nonfast_small c dst old v xs payload rem head items dq enc hm heq hcount hp hdepth hbytes

/-- Consuming a complete prefix advances without signed wraparound and leaves its exact suffix. -/
theorem input_consumption (xs front rest : List w8) (pos len : w64)
    (hp : 0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z len)
    (hlen : xs.length = sint.nat len)
    (hbytes : subslice (sint.nat pos) (sint.nat len) xs = front ++ rest) :
    sint.Z (pos + W64 front.length) = sint.Z pos + front.length ∧
    sint.Z (pos + W64 front.length) ≤ sint.Z len ∧
    subslice (sint.nat (pos + W64 front.length)) (sint.nat len) xs = rest := by
  have hbound : sint.Z pos + front.length ≤ sint.Z len := by
    have hcount := congrArg List.length hbytes
    simp only [subslice_length (sint.nat pos) (sint.nat len) xs (by omega), List.length_append] at hcount
    simp only [sint.nat, sint.Z] at hlen hcount hp ⊢
    omega
  have hnext : sint.Z (pos + W64 front.length) = sint.Z pos + front.length := by word
  refine ⟨hnext, by omega, ?_⟩
  have hnat : sint.nat (pos + W64 front.length) = sint.nat pos + front.length := by word
  rw [hnat]
  exact input_suffix_after xs front rest (sint.nat pos) (sint.nat len) hbytes

/-- Every array of zero through four valid limbs produces a fresh, owned typed felt. -/
theorem wp_readFelt_small (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (items : List UnsignedWire) (dq : DFrac)
    (enc : HeadArgument head payload (W64 items.length)) (hm : head &&& W8 224 = W8 128)
    (hcount : items.length ≤ 4)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hdepth : sint.Z v.depth' < 32)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ unsignedBytes items ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ (p : Loc), RET #interface.nil; dst ↦ p ∗ p ↦ paddedLimbs items ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (unsignedBytes items).length)} ∗
       v.buf' ↦*{dq} xs ∗ ⌜p ≠ null ∧
         sint.Z (v.pos' + W64 (1 + payload.length + (unsignedBytes items).length)) =
           sint.Z v.pos' + (1 + payload.length + (unsignedBytes items).length) ∧
         sint.Z (v.pos' + W64 (1 + payload.length + (unsignedBytes items).length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + (unsignedBytes items).length)))
           (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ unsignedBytes items) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
  have hi := input_suffix_head xs (payload ++ unsignedBytes items ++ rem)
    (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hn : head ≠ W8 246 := by intro h; subst head; contradiction
  have hu : head ≠ W8 247 := by intro h; subst head; contradiction
  wp_auto
  wp_apply wp_null_other c v xs head dq hp.1 hi hn hu $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_limbs_small c «$r0_ptr» (zero_val (GoArray w64 4)) v xs payload rem head items dq
    enc hm hcount hp hdepth hbytes $$ [«$r0» Hc Hs] with ⟨Hv, Hc, Hs⟩
  · iframe «$r0» Hc Hs
  ihave %Hnn := typedPointsto_not_null _ _ _ $$ Hv
  iapply HΦ
  iframe Hdst Hv Hc Hs
  ipureintro
  exact ⟨Hnn, by simpa only [Int.add_comm (payload.length : Int) 1] using Hnext⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbs_four_notwide

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbs_four

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbs_small
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_small

#print axioms Perennial.github_com.NethermindEth.juno.core.input_consumption
