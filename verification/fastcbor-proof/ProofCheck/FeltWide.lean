import ProofCheck.Reject

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

/-- Four unsigned 64-bit CBOR limbs, in their encoded order. No field-value restriction is imposed. -/
def wideFeltBytes (a b c d : List w8) : List w8 :=
  [W8 132, W8 27] ++ a ++ [W8 27] ++ b ++ [W8 27] ++ c ++ [W8 27] ++ d

def wideFeltValue (a b c d : List w8) : GoArray w64 4 :=
  array.mk 4 [leToU64 a.reverse, leToU64 b.reverse, leToU64 c.reverse, leToU64 d.reverse]

/-- The standard-library word decoder can borrow a suffix and restore ownership of the whole input. -/
theorem wp_bigEndian_offset (data : GoSlice) (xs bs rem : List w8) (off high : w64) (dq : DFrac) (hhigh : high = data.len)
    (hbound : 0 ≤ sint.Z off ∧ sint.Z off ≤ sint.Z data.len)
    (hbytes : subslice (sint.nat off) (sint.nat data.len) xs = bs ++ rem)
    (hlen : bs.length = 8) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.encoding.binary ∗ data ↦*{dq} xs }}
      (App (Val ((globalAddr encoding.binary.BigEndian) @!!
        go.PointerType encoding.binary.bigEndian.ty @!! go!"Uint64"))
        (Val #(slice.slice data w8 off high)))
    {{ RET #(leToU64 bs.reverse); data ↦*{dq} xs }} := by
  subst high
  iintro %Φ ⟨#Hpkg, Hs⟩ HΦ
  icases (ownSlice_slice off data.len data dq xs
      ⟨hbound.1, hbound.2, Int.le_refl _⟩).1 $$ Hs with ⟨Hpre, Hbytes, Hpost⟩
  isimp only [hbytes] at Hbytes
  wp_apply encoding.binary.wp_BigEndian_Uint64 (slice.slice data w8 off data.len)
    bs dq rem hlen $$ [Hbytes] with Hbytes
  · iframe Hbytes
  iapply HΦ
  iapply (ownSlice_slice off data.len data dq xs
      ⟨hbound.1, hbound.2, Int.le_refl _⟩).2
  rw [hbytes]
  iframe Hpre Hbytes Hpost

/-- The 37-byte fast path decodes arbitrary limb bits at arbitrary input offsets. -/
theorem wp_limbs_wide (c dst : Loc) (old : GoArray w64 4) (v : cursor.Cursor)
    (xs a b cc d rem : List w8) (dq : DFrac)
    (ha : a.length = 8) (hb : b.length = 8) (hc : cc.length = 8) (hd : d.length = 8)
    (hp : 0 ≤ sint.Z v.pos') (hdepth : sint.Z v.depth' < 32)
    (hbound : sint.Z v.pos' + 37 ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wideFeltBytes a b cc d ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Limbs")) (Val #dst))
    {{ RET #interface.nil; dst ↦ wideFeltValue a b cc d ∗
       c ↦ {v with pos' := v.pos' + W64 37} ∗ v.buf' ↦*{dq} xs }} := by
  rcases old with ⟨old⟩
  wp_start as ⟨Hdst, Hc, Hs⟩
  ihave %Holdlen := array_len dst (DFrac.own 1) 4 old $$ Hdst
  have holdlen : old.length = 4 := by omega
  obtain ⟨o0, o1, o2, o3, rfl⟩ : ∃ o0 o1 o2 o3 : w64, old = [o0, o1, o2, o3] :=
    ⟨old[0]'(by omega), old[1]'(by omega), old[2]'(by omega), old[3]'(by omega),
      List.eq_getElem_of_length_eq_four old holdlen⟩
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have hnd : ¬ sint.Z (W64 32) ≤ sint.Z v.depth' := by change ¬ 32 ≤ sint.Z v.depth'; omega
  wp_auto
  simp only [hnd, _root_.decide_false]
  wp_auto
  have hslice : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len ∧ sint.Z v.buf'.len ≤ sint.Z v.buf'.cap := by omega
  rw [ite_eq_left_of_eq_true _ _ (eq_true hslice)]
  icases (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs
      ⟨hp, hslice.2.1, Int.le_refl _⟩).1 $$ Hs with ⟨Hpre, Hbuf, Hpost⟩
  isimp only [hbytes] at Hbuf
  wp_auto
  have hlen : sint.Z (W64 37) ≤ sint.Z (v.buf'.len - v.pos') := by word
  simp only [hlen, _root_.decide_true]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 0) < sint.Z (v.buf'.len - v.pos'); word⟩)]
  wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _
      (wideFeltBytes a b cc d ++ rem) dq (W8 132) (by decide) $$ [Hbuf] with Hbuf
  · iframe Hbuf
    ipureintro
    change (wideFeltBytes a b cc d ++ rem)[0]? = some (W8 132)
    simp [wideFeltBytes, List.getElem?_append, ha, hb, hc, hd]
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 1) < sint.Z (v.buf'.len - v.pos'); word⟩)]
  wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _
      (wideFeltBytes a b cc d ++ rem) dq (W8 27) (by decide) $$ [Hbuf] with Hbuf
  · iframe Hbuf
    ipureintro
    change (wideFeltBytes a b cc d ++ rem)[1]? = some (W8 27)
    simp [wideFeltBytes, List.getElem?_append, ha, hb, hc, hd]
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 10) < sint.Z (v.buf'.len - v.pos'); word⟩)]
  wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _
      (wideFeltBytes a b cc d ++ rem) dq (W8 27) (by decide) $$ [Hbuf] with Hbuf
  · iframe Hbuf
    ipureintro
    change (wideFeltBytes a b cc d ++ rem)[10]? = some (W8 27)
    simp [wideFeltBytes, List.getElem?_append, ha, hb, hc, hd]
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 19) < sint.Z (v.buf'.len - v.pos'); word⟩)]
  wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _
      (wideFeltBytes a b cc d ++ rem) dq (W8 27) (by decide) $$ [Hbuf] with Hbuf
  · iframe Hbuf
    ipureintro
    change (wideFeltBytes a b cc d ++ rem)[19]? = some (W8 27)
    simp [wideFeltBytes, List.getElem?_append, ha, hb, hc, hd]
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 28) < sint.Z (v.buf'.len - v.pos'); word⟩)]
  wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _
      (wideFeltBytes a b cc d ++ rem) dq (W8 27) (by decide) $$ [Hbuf] with Hbuf
  · iframe Hbuf
    ipureintro
    change (wideFeltBytes a b cc d ++ rem)[28]? = some (W8 27)
    simp [wideFeltBytes, List.getElem?_append, ha, hb, hc, hd]
  ihave %Hbuflen := ownSlice_len _ _ _ $$ Hbuf
  ihave %Hbufwf := ownSlice_wf _ _ _ $$ Hbuf
  have hfull : sint.nat (v.buf'.len - v.pos') = (wideFeltBytes a b cc d ++ rem).length := Hbuflen.1.symm
  have hbnd2 : 0 ≤ sint.Z (W64 2) ∧ sint.Z (W64 2) ≤ sint.Z (v.buf'.len - v.pos') ∧
      sint.Z (v.buf'.len - v.pos') ≤ sint.Z (v.buf'.cap - v.pos') := by
    exact ⟨by decide, by word, Hbufwf.2⟩
  rw [ite_eq_left_of_eq_true _ _ (eq_true hbnd2)]
  have hreadbytes : subslice 2 (sint.nat (v.buf'.len - v.pos')) (wideFeltBytes a b cc d ++ rem) =
      a ++ ([W8 27] ++ b ++ [W8 27] ++ cc ++ [W8 27] ++ d ++ rem) := by
    rw [hfull]
    simp only [subslice, List.take_length]
    simp [wideFeltBytes, List.append_assoc]
  wp_apply wp_bigEndian_offset (slice.slice v.buf' w8 v.pos' v.buf'.len)
      (wideFeltBytes a b cc d ++ rem) a ([W8 27] ++ b ++ [W8 27] ++ cc ++ [W8 27] ++ d ++ rem)
      (W64 2) (v.buf'.len - v.pos') dq rfl ⟨hbnd2.1, hbnd2.2.1⟩ hreadbytes ha $$ [Hbuf] with Hbuf
  · iframe Hbuf
  icases array_acc dst 0 (DFrac.own 1) 4 (array.mk 4 [o0, o1, o2, o3]) o0
      (by decide) rfl $$ Hdst with ⟨Hword, Hrestore⟩
  simp only [show sint.Z (W64 0) = 0 from rfl]
  wp_store
  wp_auto
  ihave Hdst := Hrestore $$ %(leToU64 a.reverse) Hword
  ieval (change (dst ↦ array.mk 4 [leToU64 a.reverse, o1, o2, o3] : IProp GF) ⊢ _) at Hdst
  have hbnd11 : 0 ≤ sint.Z (W64 11) ∧ sint.Z (W64 11) ≤ sint.Z (v.buf'.len - v.pos') ∧
      sint.Z (v.buf'.len - v.pos') ≤ sint.Z (v.buf'.cap - v.pos') := ⟨by decide, by word, Hbufwf.2⟩
  rw [ite_eq_left_of_eq_true _ _ (eq_true hbnd11)]
  have hreadbytes11 : subslice 11 (sint.nat (v.buf'.len - v.pos')) (wideFeltBytes a b cc d ++ rem) =
      b ++ ([W8 27] ++ cc ++ [W8 27] ++ d ++ rem) := by
    rw [hfull]
    simp only [subslice, List.take_length]
    simp [wideFeltBytes, List.append_assoc, List.drop_append, List.drop_eq_nil_of_le, ha, hb, hc, hd]
  wp_apply wp_bigEndian_offset (slice.slice v.buf' w8 v.pos' v.buf'.len)
      (wideFeltBytes a b cc d ++ rem) b ([W8 27] ++ cc ++ [W8 27] ++ d ++ rem)
      (W64 11) (v.buf'.len - v.pos') dq rfl ⟨hbnd11.1, hbnd11.2.1⟩ hreadbytes11 hb
      $$ [Hbuf] with Hbuf
  · iframe Hbuf
  icases array_acc dst 1 (DFrac.own 1) 4 (array.mk 4 [leToU64 a.reverse, o1, o2, o3]) o1
      (by decide) rfl $$ Hdst with ⟨Hword, Hrestore⟩
  simp only [show sint.Z (W64 1) = 1 from rfl]
  wp_store
  wp_auto
  ihave Hdst := Hrestore $$ %(leToU64 b.reverse) Hword
  ieval (change (dst ↦ array.mk 4 [leToU64 a.reverse, leToU64 b.reverse, o2, o3] : IProp GF) ⊢ _) at Hdst
  have hbnd20 : 0 ≤ sint.Z (W64 20) ∧ sint.Z (W64 20) ≤ sint.Z (v.buf'.len - v.pos') ∧
      sint.Z (v.buf'.len - v.pos') ≤ sint.Z (v.buf'.cap - v.pos') := ⟨by decide, by word, Hbufwf.2⟩
  rw [ite_eq_left_of_eq_true _ _ (eq_true hbnd20)]
  have hreadbytes20 : subslice 20 (sint.nat (v.buf'.len - v.pos')) (wideFeltBytes a b cc d ++ rem) =
      cc ++ ([W8 27] ++ d ++ rem) := by
    rw [hfull]
    simp only [subslice, List.take_length]
    simp [wideFeltBytes, List.append_assoc, List.drop_append, List.drop_eq_nil_of_le, ha, hb, hc, hd]
  wp_apply wp_bigEndian_offset (slice.slice v.buf' w8 v.pos' v.buf'.len)
      (wideFeltBytes a b cc d ++ rem) cc ([W8 27] ++ d ++ rem)
      (W64 20) (v.buf'.len - v.pos') dq rfl ⟨hbnd20.1, hbnd20.2.1⟩ hreadbytes20 hc
      $$ [Hbuf] with Hbuf
  · iframe Hbuf
  icases array_acc dst 2 (DFrac.own 1) 4 (array.mk 4 [leToU64 a.reverse, leToU64 b.reverse, o2, o3]) o2
      (by decide) rfl $$ Hdst with ⟨Hword, Hrestore⟩
  simp only [show sint.Z (W64 2) = 2 from rfl]
  wp_store
  wp_auto
  ihave Hdst := Hrestore $$ %(leToU64 cc.reverse) Hword
  ieval (change (dst ↦ array.mk 4 [leToU64 a.reverse, leToU64 b.reverse, leToU64 cc.reverse, o3] : IProp GF) ⊢ _) at Hdst
  have hbnd29 : 0 ≤ sint.Z (W64 29) ∧ sint.Z (W64 29) ≤ sint.Z (v.buf'.len - v.pos') ∧
      sint.Z (v.buf'.len - v.pos') ≤ sint.Z (v.buf'.cap - v.pos') := ⟨by decide, by word, Hbufwf.2⟩
  rw [ite_eq_left_of_eq_true _ _ (eq_true hbnd29)]
  have hreadbytes29 : subslice 29 (sint.nat (v.buf'.len - v.pos')) (wideFeltBytes a b cc d ++ rem) =
      d ++ (rem) := by
    rw [hfull]
    simp only [subslice, List.take_length]
    simp [wideFeltBytes, List.append_assoc, List.drop_append, List.drop_eq_nil_of_le, ha, hb, hc, hd]
  wp_apply wp_bigEndian_offset (slice.slice v.buf' w8 v.pos' v.buf'.len)
      (wideFeltBytes a b cc d ++ rem) d (rem)
      (W64 29) (v.buf'.len - v.pos') dq rfl ⟨hbnd29.1, hbnd29.2.1⟩ hreadbytes29 hd
      $$ [Hbuf] with Hbuf
  · iframe Hbuf
  icases array_acc dst 3 (DFrac.own 1) 4 (array.mk 4 [leToU64 a.reverse, leToU64 b.reverse, leToU64 cc.reverse, o3]) o3
      (by decide) rfl $$ Hdst with ⟨Hword, Hrestore⟩
  simp only [show sint.Z (W64 3) = 3 from rfl]
  wp_store
  wp_auto
  ihave Hdst := Hrestore $$ %(leToU64 d.reverse) Hword
  ieval (change (dst ↦ array.mk 4 [leToU64 a.reverse, leToU64 b.reverse, leToU64 cc.reverse, leToU64 d.reverse] : IProp GF) ⊢ _) at Hdst
  ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
  · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs
      ⟨hp, hslice.2.1, Int.le_refl _⟩).2
    rw [hbytes]
    iframe Hpre Hbuf Hpost
  wp_end

/-- The typed reader publishes a fresh, owned felt whose four words match the encoded bytes. -/
theorem wp_readFelt_wide (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs a b cc d rem : List w8) (dq : DFrac)
    (ha : a.length = 8) (hb : b.length = 8) (hc : cc.length = 8) (hd : d.length = 8)
    (hp : 0 ≤ sint.Z v.pos') (hdepth : sint.Z v.depth' < 32)
    (hbound : sint.Z v.pos' + 37 ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wideFeltBytes a b cc d ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ (p : Loc), RET #interface.nil; dst ↦ p ∗ p ↦ wideFeltValue a b cc d ∗
       c ↦ {v with pos' := v.pos' + W64 37} ∗ v.buf' ↦*{dq} xs ∗ ⌜p ≠ null⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  have hi : xs[(sint.Z v.pos').toNat]? = some (W8 132) := by
    have hfirst : (subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs)[0]? = some (W8 132) := by
      rw [hbytes]
      simp [wideFeltBytes]
    rw [subslice_lookup _ _ 0 xs (by word), Nat.add_zero] at hfirst
    exact hfirst
  wp_auto
  wp_apply wp_null_other c v xs (W8 132) dq hp hi (by decide) (by decide)
    $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_limbs_wide c «$r0_ptr» (zero_val (GoArray w64 4)) v xs a b cc d rem dq
    ha hb hc hd hp hdepth hbound hbytes $$ [«$r0» Hc Hs] with ⟨Hv, Hc, Hs⟩
  · iframe «$r0» Hc Hs
  ihave %Hnn := typedPointsto_not_null _ _ _ $$ Hv
  iapply HΦ
  iframe Hdst Hv Hc Hs
  ipureintro; exact Hnn

/-- An array head at the nesting limit fails before any limb is read or output is changed. -/
theorem wp_limbsSlow_depth (c dst : Loc) (old : GoArray w64 4) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (count : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload count) (hm : head &&& W8 224 = W8 128)
    (hp : 0 ≤ sint.Z v.pos') (hdepth : 32 ≤ sint.Z v.depth')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrDepth ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"limbsSlow")) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"nested more than %d levels") args ∗
       args ↦* [interface.mkOk go.int #(W64 32)] }} := by
  wp_start as ⟨#Hkind, Hdst, Hc, Hs⟩
  wp_auto
  rw [← hm]
  wp_apply wp_expect_encoded c v xs payload rem head count dq enc hp hbound hi hbytes
    $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_enter_depth c {v with pos' := v.pos' + W64 (1 + payload.length)} v.pos' kind hdepth
    $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hkind Hc
  wp_end

/-- Limbs checks depth before either shortcut, including the 37-byte form. -/
theorem wp_limbs_depth (c dst : Loc) (old : GoArray w64 4) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (count : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload count) (hm : head &&& W8 224 = W8 128)
    (hp : 0 ≤ sint.Z v.pos') (hdepth : 32 ≤ sint.Z v.depth')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrDepth ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Limbs")) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"nested more than %d levels") args ∗
       args ↦* [interface.mkOk go.int #(W64 32)] }} := by
  wp_start as ⟨#Hkind, Hdst, Hc, Hs⟩
  wp_auto
  have hdep : sint.Z (W64 32) ≤ sint.Z v.depth' := hdepth
  simp only [hdep, _root_.decide_true]
  wp_auto
  wp_apply wp_limbsSlow_depth c dst old v xs payload rem head count dq kind enc hm hp hdepth hbound hi hbytes
    $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
  · iframe Hkind Hdst Hc Hs
  wp_end

/-- A typed felt field keeps its prior pointer on depth rejection; no partial result is published. -/
theorem wp_readFelt_depth (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (count : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload count) (hm : head &&& W8 224 = W8 128)
    (hp : 0 ≤ sint.Z v.pos') (hdepth : 32 ≤ sint.Z v.depth')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr cursor.ErrDepth ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"nested more than %d levels") args ∗
       args ↦* [interface.mkOk go.int #(W64 32)] }} := by
  wp_start as ⟨#Hkind, Hdst, Hc, Hs⟩
  have hn : head ≠ W8 246 := by intro h; subst head; contradiction
  have hu : head ≠ W8 247 := by intro h; subst head; contradiction
  wp_auto
  wp_apply wp_null_other c v xs head dq hp hi hn hu $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_limbs_depth c «$r0_ptr» (zero_val (GoArray w64 4)) v xs payload rem head count dq kind
    enc hm hp hdepth hbound hi hbytes $$ [«$r0» Hc Hs] as %err ⟨Hv, Hc, Hs, Herr⟩
  · iframe Hkind «$r0» Hc Hs
  wp_end

/-- Equivalently, all four uint64 words round-trip through their Juno wide CBOR encoding. -/
theorem wp_readFelt_wide_words (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs rem : List w8) (w0 w1 w2 w3 : w64) (dq : DFrac)
    (hp : 0 ≤ sint.Z v.pos') (hdepth : sint.Z v.depth' < 32)
    (hbound : sint.Z v.pos' + 37 ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wideFeltBytes (u64Le w0).reverse (u64Le w1).reverse (u64Le w2).reverse (u64Le w3).reverse ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ (p : Loc), RET #interface.nil; dst ↦ p ∗ p ↦ array.mk 4 [w0, w1, w2, w3] ∗
       c ↦ {v with pos' := v.pos' + W64 37} ∗ v.buf' ↦*{dq} xs ∗ ⌜p ≠ null⌝ }} := by
  simpa only [wideFeltValue, List.reverse_reverse, u64Le_to_word] using
    (wp_readFelt_wide (GF := GF) (hG := hG) c dst old v xs
      (u64Le w0).reverse (u64Le w1).reverse (u64Le w2).reverse (u64Le w3).reverse rem dq
      (by simp [u64Le_length]) (by simp [u64Le_length]) (by simp [u64Le_length]) (by simp [u64Le_length])
      hp hdepth hbound hbytes)

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bigEndian_offset
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbs_wide

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_wide

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbsSlow_depth

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbs_depth

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_depth

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_wide_words
