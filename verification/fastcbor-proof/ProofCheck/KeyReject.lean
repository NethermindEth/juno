import ProofCheck.KeyInteger

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

/-- A complete non-key head is rolled back before reporting the key-type mismatch. -/
theorem wp_keySlow_tag (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument)
    (htext : head &&& W8 224 ≠ W8 96)
    (huint : head &&& W8 224 ≠ W8 0) (hneg : head &&& W8 224 ≠ W8 32)
    (htag : head &&& W8 224 = W8 192)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"keySlow")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #slice.nil #false) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"tag where %s is expected") args ∗
       args ↦* [interface.mkOk go.string #(go!"a text or integer key")] }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hw := headArgument_width head payload argument enc
  have Hnext := input_consumption xs (head :: payload) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    omega
  have hi := input_suffix_head xs (payload ++ rem) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem := by
    have hn : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
    rw [hn]
    exact input_suffix_after xs [head] (payload ++ rem) _ _ hbytes
  wp_auto
  wp_apply wp_head_encoded c v xs payload rem head argument dq enc hp.1 hbound hi hafter
    $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  simp only [htext, _root_.decide_false]
  wp_auto
  simp only [hneg, _root_.decide_false]
  wp_auto
  simp only [huint, _root_.decide_false]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp.1, by omega⟩)]
  wp_apply wp_load_slice_index v.buf' _ xs dq head hp.1 $$ [Hs] with Hs
  · iframe Hs; ipureintro; exact hi
  wp_apply wp_mismatch_tag c v (go!"a text or integer key") head kind htag $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc Hkind
  wp_end

/-- A complete non-key head is rolled back before reporting the key-type mismatch. -/
theorem wp_keySlow_type (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument)
    (htext : head &&& W8 224 ≠ W8 96)
    (huint : head &&& W8 224 ≠ W8 0) (hneg : head &&& W8 224 ≠ W8 32)
    (htag : head &&& W8 224 ≠ W8 192)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrType ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"keySlow")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #slice.nil #false) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"expected %s, found %s") args ∗
       args ↦* [interface.mkOk go.string #(go!"a text or integer key"), interface.mkOk go.string #(headDescription head)] }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hw := headArgument_width head payload argument enc
  have Hnext := input_consumption xs (head :: payload) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    omega
  have hi := input_suffix_head xs (payload ++ rem) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem := by
    have hn : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
    rw [hn]
    exact input_suffix_after xs [head] (payload ++ rem) _ _ hbytes
  wp_auto
  wp_apply wp_head_encoded c v xs payload rem head argument dq enc hp.1 hbound hi hafter
    $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  simp only [htext, _root_.decide_false]
  wp_auto
  simp only [hneg, _root_.decide_false]
  wp_auto
  simp only [huint, _root_.decide_false]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp.1, by omega⟩)]
  wp_apply wp_load_slice_index v.buf' _ xs dq head hp.1 $$ [Hs] with Hs
  · iframe Hs; ipureintro; exact hi
  wp_apply wp_mismatch_type c v (go!"a text or integer key") head kind htag $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc Hkind
  wp_end

/-- Missing text-key payloads fail before validation and preserve the original cursor. -/
theorem wp_keySlow_text_truncated (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument)
    (hm : head &&& W8 224 = W8 96)
    (hshort : rem.length < uint.Z argument)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"keySlow")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #slice.nil #false) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"key longer than the input") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hw := headArgument_width head payload argument enc
  have Hnext := input_consumption xs (head :: payload) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    omega
  have hi := input_suffix_head xs (payload ++ rem) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem := by
    have hn : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
    rw [hn]
    exact input_suffix_after xs [head] (payload ++ rem) _ _ hbytes
  wp_auto
  wp_apply wp_head_encoded c v xs payload rem head argument dq enc hp.1 hbound hi hafter
    $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  simp only [hm]
  wp_auto
  wp_method_call
  wp_call
  wp_auto
  have hremaining : (rem.length : Int) = sint.Z v.buf'.len - (sint.Z v.pos' + 1 + payload.length) := by
    have hcount := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega),
      List.length_append, List.length_cons] at hcount
    simp only [sint.nat, sint.Z] at Hlen hcount hp ⊢
    omega
  have hbad : uint.Z (v.buf'.len - (v.pos' + W64 (1 + payload.length))) < uint.Z argument := by word
  simp only [hbad, _root_.decide_true]
  wp_auto
  wp_apply wp_fail c v kind (go!"key longer than the input") slice.nil $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc
  wp_end
/-- Public Key preserves the typed mismatch diagnostic and rollback. -/
theorem wp_Key_tag (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument)
    (htext : head &&& W8 224 ≠ W8 96)
    (huint : head &&& W8 224 ≠ W8 0) (hneg : head &&& W8 224 ≠ W8 32)
    (htag : head &&& W8 224 = W8 192)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Key")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #slice.nil #false) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"tag where %s is expected") args ∗
       args ↦* [interface.mkOk go.string #(go!"a text or integer key")] }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have Hnext := input_consumption xs (head :: payload) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hheadbound : sint.Z v.pos' < sint.Z v.buf'.len := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    omega
  have hslow := byte_nontext_key_slow head htext
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp.1, hp.2, Hwf.2⟩)]
  wp_auto
  have hlenpos : sint.Z (W64 0) < sint.Z (v.buf'.len - v.pos') := by word
  simp only [hlenpos, _root_.decide_true]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, hlenpos⟩)]
  icases (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).1
    $$ Hs with ⟨Hpre, Hbuf, Hpost⟩
  isimp only [hbytes] at Hbuf
  wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _
    ((head :: payload) ++ rem) dq head (by decide) $$ [Hbuf] with Hbuf
  · iframe Hbuf; ipureintro; rfl
  have hnot24 : W64 (uint.Z (head - W8 96)) ≠ W64 24 := by word
  simp only [hnot24, _root_.decide_false]
  wp_auto
  have hnotshort : ¬ sint.Z (W64 (uint.Z (head - W8 96))) < sint.Z (W64 24) := by word
  simp only [hnotshort, _root_.decide_false]
  wp_auto
  ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
  · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
    rw [hbytes]
    iframe Hpre Hbuf Hpost
  wp_apply wp_keySlow_tag c v xs payload rem head argument dq kind enc htext huint hneg htag hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hc Hs Hkind
  wp_end

/-- Public Key preserves the typed mismatch diagnostic and rollback. -/
theorem wp_Key_type (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument)
    (htext : head &&& W8 224 ≠ W8 96)
    (huint : head &&& W8 224 ≠ W8 0) (hneg : head &&& W8 224 ≠ W8 32)
    (htag : head &&& W8 224 ≠ W8 192)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrType ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Key")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #slice.nil #false) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"expected %s, found %s") args ∗
       args ↦* [interface.mkOk go.string #(go!"a text or integer key"), interface.mkOk go.string #(headDescription head)] }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have Hnext := input_consumption xs (head :: payload) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hheadbound : sint.Z v.pos' < sint.Z v.buf'.len := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    omega
  have hslow := byte_nontext_key_slow head htext
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp.1, hp.2, Hwf.2⟩)]
  wp_auto
  have hlenpos : sint.Z (W64 0) < sint.Z (v.buf'.len - v.pos') := by word
  simp only [hlenpos, _root_.decide_true]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, hlenpos⟩)]
  icases (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).1
    $$ Hs with ⟨Hpre, Hbuf, Hpost⟩
  isimp only [hbytes] at Hbuf
  wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _
    ((head :: payload) ++ rem) dq head (by decide) $$ [Hbuf] with Hbuf
  · iframe Hbuf; ipureintro; rfl
  have hnot24 : W64 (uint.Z (head - W8 96)) ≠ W64 24 := by word
  simp only [hnot24, _root_.decide_false]
  wp_auto
  have hnotshort : ¬ sint.Z (W64 (uint.Z (head - W8 96))) < sint.Z (W64 24) := by word
  simp only [hnotshort, _root_.decide_false]
  wp_auto
  ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
  · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
    rw [hbytes]
    iframe Hpre Hbuf Hpost
  wp_apply wp_keySlow_type c v xs payload rem head argument dq kind enc htext huint hneg htag hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hc Hs Hkind
  wp_end


/-- Recover the argument byte of a complete one-byte CBOR length header. -/
theorem headArgument_byte_inv (head : w8) (payload : List w8) (argument : w64)
    (enc : HeadArgument head payload argument) (hbyte : head &&& W8 31 = W8 24) :
    ∃ b : w8, payload = [b] ∧ argument = W64 (uint.Z b) := by
  cases enc with
  | inline h => rw [hbyte] at h; contradiction
  | byte b h => exact ⟨b, rfl, rfl⟩
  | u16 a b h => rw [h] at hbyte; contradiction
  | u32 bs hlen h => rw [h] at hbyte; contradiction
  | u64 bs hlen h => rw [h] at hbyte; contradiction

/-- Public Key rejects a truncated text body through every short-key dispatch branch. -/
theorem wp_Key_text_truncated (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument)
    (hm : head &&& W8 224 = W8 96)
    (hshort : rem.length < uint.Z argument)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Key")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #slice.nil #false) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"key longer than the input") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have Hnext := input_consumption xs (head :: payload) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hheadbound : sint.Z v.pos' < sint.Z v.buf'.len := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    omega
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp.1, hp.2, Hwf.2⟩)]
  wp_auto
  have hlenpos : sint.Z (W64 0) < sint.Z (v.buf'.len - v.pos') := by word
  simp only [hlenpos, _root_.decide_true]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, hlenpos⟩)]
  icases (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).1
    $$ Hs with ⟨Hpre, Hbuf, Hpost⟩
  isimp only [hbytes] at Hbuf
  wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _
    ((head :: payload) ++ rem) dq head (by decide) $$ [Hbuf] with Hbuf
  · iframe Hbuf; ipureintro; rfl
  have hsub : head - W8 96 = head &&& W8 31 := by
    simpa only [hm] using byte_major_remainder head
  have hremaining : (rem.length : Int) = sint.Z v.buf'.len - (sint.Z v.pos' + 1 + payload.length) := by
    have hcount := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega),
      List.length_append, List.length_cons] at hcount
    simp only [sint.nat, sint.Z] at Hlen hcount hp ⊢
    omega
  by_cases hbyte : head &&& W8 31 = W8 24
  · obtain ⟨b, hpay, harg⟩ := headArgument_byte_inv head payload argument enc hbyte
    have hlong : sint.Z (W64 1) < sint.Z (v.buf'.len - v.pos') := by
      simp only [hpay, List.length_cons, List.length_nil] at hremaining
      word
    simp only [hsub, hbyte]
    wp_auto
    simp only [hlong, _root_.decide_true]
    wp_auto
    rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, hlong⟩)]
    wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _
      ((head :: payload) ++ rem) dq b (by decide) $$ [Hbuf] with Hbuf
    · iframe Hbuf; ipureintro; simp [hpay]
    by_cases hsmall : sint.Z (W64 (uint.Z b)) < sint.Z (W64 24)
    all_goals
      simp only [hsmall, _root_.decide_true, _root_.decide_false]
      wp_auto
      have hbad : ¬ sint.Z (W64 2 + W64 (uint.Z b)) ≤ sint.Z (v.buf'.len - v.pos') := by
        simp only [hpay, List.length_cons, List.length_nil] at hremaining
        rw [harg] at hshort
        word
      simp only [hbad, _root_.decide_false]
      wp_auto
      ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
      · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
        rw [hbytes]
        iframe Hpre Hbuf Hpost
      wp_apply wp_keySlow_text_truncated c v xs payload rem head argument dq kind enc hm hshort hp hbytes
        $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
      · iframe Hc Hs Hkind
      wp_end
  · have hnot24 : W64 (uint.Z (head - W8 96)) ≠ W64 24 := by rw [hsub]; word
    simp only [hnot24, _root_.decide_false]
    wp_auto
    by_cases hsmall : uint.Z (head &&& W8 31) < 24
    · obtain ⟨hpay, harg⟩ := headArgument_inline_inv head payload argument enc hsmall
      have hlt : sint.Z (W64 (uint.Z (head - W8 96))) < sint.Z (W64 24) := by rw [hsub]; word
      simp only [hlt, _root_.decide_true]
      wp_auto
      have hbad : ¬ sint.Z (W64 1 + W64 (uint.Z (head - W8 96))) ≤ sint.Z (v.buf'.len - v.pos') := by
        simp only [hpay, List.length_nil] at hremaining
        rw [harg] at hshort
        rw [hsub]
        word
      simp only [hbad, _root_.decide_false]
      wp_auto
      ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
      · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
        rw [hbytes]
        iframe Hpre Hbuf Hpost
      wp_apply wp_keySlow_text_truncated c v xs payload rem head argument dq kind enc hm hshort hp hbytes
        $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
      · iframe Hc Hs Hkind
      wp_end
    · have hlt : ¬ sint.Z (W64 (uint.Z (head - W8 96))) < sint.Z (W64 24) := by rw [hsub]; word
      simp only [hlt, _root_.decide_false]
      wp_auto
      ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
      · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
        rw [hbytes]
        iframe Hpre Hbuf Hpost
      wp_apply wp_keySlow_text_truncated c v xs payload rem head argument dq kind enc hm hshort hp hbytes
        $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
      · iframe Hc Hs Hkind
      wp_end


/-- Key rejection propagates through alternate field lookup without changing its state. -/
theorem wp_otherField_tag (c st : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument)
    (htext : head &&& W8 224 ≠ W8 96)
    (huint : head &&& W8 224 ≠ W8 0) (hneg : head &&& W8 224 ≠ W8 32)
    (htag : head &&& W8 224 = W8 192)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"otherField")) (Val #st))
    {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"tag where %s is expected") args ∗
       args ↦* [interface.mkOk go.string #(go!"a text or integer key")] }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_Key_tag c v xs payload rem head argument dq kind enc htext huint hneg htag hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hc Hs Hkind
  wp_end

/-- Key rejection propagates through alternate field lookup without changing its state. -/
theorem wp_otherField_type (c st : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument)
    (htext : head &&& W8 224 ≠ W8 96)
    (huint : head &&& W8 224 ≠ W8 0) (hneg : head &&& W8 224 ≠ W8 32)
    (htag : head &&& W8 224 ≠ W8 192)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrType ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"otherField")) (Val #st))
    {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"expected %s, found %s") args ∗
       args ↦* [interface.mkOk go.string #(go!"a text or integer key"), interface.mkOk go.string #(headDescription head)] }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_Key_type c v xs payload rem head argument dq kind enc htext huint hneg htag hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hc Hs Hkind
  wp_end

/-- Key rejection propagates through alternate field lookup without changing its state. -/
theorem wp_otherField_text_truncated (c st : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument)
    (hm : head &&& W8 224 = W8 96)
    (hshort : rem.length < uint.Z argument)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"otherField")) (Val #st))
    {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"key longer than the input") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_Key_text_truncated c v xs payload rem head argument dq kind enc hm hshort hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hc Hs Hkind
  wp_end

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_keySlow_tag

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_keySlow_type

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_keySlow_text_truncated

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Key_tag

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Key_type

#print axioms Perennial.github_com.NethermindEth.juno.core.headArgument_byte_inv
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Key_text_truncated

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_otherField_tag

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_otherField_type

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_otherField_text_truncated
