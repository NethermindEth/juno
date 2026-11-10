import ProofCheck.FieldHead

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

/-- After any valid prefix shorter than four limbs, an actual Uint64 error
    restores the outer cursor/depth and leaves the destination unchanged. The
    caller supplies the proved leaf contract, including the inner error offset. -/
theorem wp_limbsSlow_prefix_reject (c dst : Loc) (old : GoArray w64 4) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (count : w64) (items : List UnsignedWire)
    (dq : DFrac) (R : IProp GF) (E : Loc → IProp GF)
    (enc : HeadArgument head payload count) (hm : head &&& W8 224 = W8 128)
    (hprefix : items.length < 4) (hcount : items.length < uint.Z count)
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
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"limbsSlow")) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ E err }} := by
  wp_start as ⟨HR, Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hw := headArgument_width head payload count enc
  have htotal : sint.Z v.pos' + 1 + payload.length + (unsignedBytes items).length ≤ sint.Z v.buf'.len := by
    have hlength := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega),
      List.length_append, List.length_cons] at hlength
    simp only [sint.nat, sint.Z] at Hlen hlength hp ⊢
    omega
  have hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by omega
  have hi := input_suffix_head xs (payload ++ unsignedBytes items ++ rem)
    (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hp1 : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs =
      payload ++ (unsignedBytes items ++ rem) := by
    rw [hp1]
    exact input_suffix_after xs [head] _ _ _ (by simpa only [List.cons_append, List.singleton_append, List.nil_append, List.append_assoc] using hbytes)
  wp_auto
  rw [← hm]
  wp_apply wp_expect_encoded c v xs payload (unsignedBytes items ++ rem) head count dq
    enc hp.1 hbound hi hafter $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_enter c {v with pos' := v.pos' + W64 (1 + payload.length)} v.pos' hdepth
    $$ [Hc] with Hc
  · iframe Hc
  have hstart : sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length := by word
  have hremaining : subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs =
      unsignedBytes items ++ rem := by
    have hn : sint.nat (v.pos' + W64 (1 + payload.length)) = sint.nat v.pos' + (head :: payload).length := by
      simp only [List.length_cons]; word
    rw [hn]
    exact input_suffix_after xs (head :: payload) _ _ _ (by simpa only [List.append_assoc] using hbytes)
  ihave HI : (∃ (done todo : List UnsignedWire) (pos oldi : w64),
      "Hrange" ∷ «$range_i_ptr» ↦ W64 done.length ∗
      "Hi" ∷ i_ptr ↦ oldi ∗
      "Hlimbs" ∷ limbs_ptr ↦ paddedLimbs done ∗
      "Hc" ∷ c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗
      "%Hsplit" ∷ ⌜items = done ++ todo⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hposition" ∷ ⌜sint.Z pos = sint.Z v.pos' + 1 + payload.length + (unsignedBytes done).length⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = unsignedBytes todo ++ rem⌝ : IProp GF)
      $$ [«$range_i» i limbs Hc]
  · iexists [], items, (v.pos' + W64 (1 + payload.length)), (W64 0)
    iframe
    ipureintro
    exact ⟨rfl, ⟨by word, by word⟩, by simpa [unsignedBytes] using hstart, hremaining⟩
  wp_for HI
  have hlength : items.length = done.length + todo.length := by rw [Hsplit, List.length_append]
  have hsmall : done.length < 4 := by omega
  have hcond : uint.Z (W64 done.length) < uint.Z count := by word
  simp only [hcond, _root_.decide_true, ite_true]
  wp_auto
  have hnotfull : ¬ uint.Z (W64 4) ≤ uint.Z (W64 done.length) := by word
  simp only [hnotfull, _root_.decide_false]
  wp_auto
  have hindex : sint.Z (W64 done.length) = done.length := by word
  rw [ite_eq_left_of_eq_true _ _ (eq_true (by rw [hindex]; exact ⟨by omega, by omega⟩))]
  wp_auto
  cases todo with
  | nil =>
    have hdoneall : done = items := by simpa using Hsplit.symm
    have hposfinal : pos = v.pos' + W64 (1 + payload.length + (unsignedBytes items).length) := by
      rw [hdoneall] at Hposition
      word
    isimp only [hposfinal] at Hc
    wp_apply hbad $$ [HR Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe HR Hc Hs
    icases array_acc limbs_ptr (done.length : Int) (DFrac.own 1) 4 (paddedLimbs done) (W64 0)
        (by omega) (by simpa using paddedLimbs_next done hsmall) $$ Hlimbs with ⟨Hword, Hrestore⟩
    simp only [hindex]
    wp_store
    wp_auto
    wp_for_post
    have hrestoredepth : v.depth' + W64 1 - W64 1 = v.depth' := by word
    isimp only [hrestoredepth] at Hc
    iapply HΦ
    iframe Hdst Hc Hs Herr
  | cons item todo =>
    wp_apply wp_uint64_wire c {v with pos' := pos, depth' := v.depth' + W64 1} xs
        (unsignedBytes todo ++ rem) item dq Hpos
        (by simpa [unsignedBytes, List.append_assoc] using Hremaining)
        $$ [Hc Hs] with ⟨Hc, Hs, %Hnext⟩
    · iframe Hc Hs
    icases array_acc limbs_ptr (done.length : Int) (DFrac.own 1) 4 (paddedLimbs done) (W64 0)
        (by omega) (by simpa using paddedLimbs_next done hsmall) $$ Hlimbs with ⟨Hword, Hrestore⟩
    simp only [hindex]
    wp_store
    wp_auto
    ihave Hlimbs := Hrestore $$ %item.value Hword
    isimp only [Int.toNat_natCast, paddedLimbs_step done item hsmall] at Hlimbs
    ieval (change (c ↦ {v with pos' := pos + W64 item.bytes.length, depth' := v.depth' + W64 1} : IProp GF) ⊢ _) at Hc
    dsimp only at Hnext
    have hbyteslen : (unsignedBytes (done ++ [item])).length = (unsignedBytes done).length + item.bytes.length := by
      simp [unsignedBytes]
    have hinc : W64 done.length + W64 1 = W64 (done ++ [item]).length := by simp only [List.length_append, List.length_singleton]; word
    wp_for_post
    isimp only [hinc] at Hrange
    iframe
    iexists (done ++ [item]), todo, (pos + W64 item.bytes.length), (W64 done.length)
    iframe
    ipureintro
    refine ⟨?_, ⟨?_, Hnext.2.1⟩, ?_, Hnext.2.2⟩
    · simpa only [List.append_assoc, List.singleton_append] using Hsplit
    · rw [Hnext.1]; omega
    · rw [Hnext.1, hbyteslen]; omega

/-- Non-inline-four heads cannot enter either shortcut. Preserve the actual
    slow-reader rejection, including its owned diagnostic. -/
theorem wp_limbs_nonfast_reject (c dst : Loc) (old : GoArray w64 4) (v : cursor.Cursor)
    (xs rest : List w8) (head : w8) (dq : DFrac) (R : IProp GF) (E : Loc → IProp GF)
    (hne : head ≠ W8 132)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hdepth : sint.Z v.depth' < 32)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rest)
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
  have hnd : ¬ sint.Z (W64 32) ≤ sint.Z v.depth' := by change ¬ 32 ≤ sint.Z v.depth'; omega
  wp_auto
  simp only [hnd, _root_.decide_false]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp.1, hp.2, Hwf.2⟩)]
  icases (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs
      ⟨hp.1, hp.2, Int.le_refl _⟩).1 $$ Hs with ⟨Hpre, Hbuf, Hpost⟩
  isimp only [hbytes] at Hbuf
  wp_auto
  by_cases hlong : sint.Z (W64 37) ≤ sint.Z (v.buf'.len - v.pos')
  · simp only [hlong, _root_.decide_true]
    wp_auto
    rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 0) < sint.Z (v.buf'.len - v.pos'); word⟩)]
    wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _
        (head :: rest) dq head (by decide) $$ [Hbuf] with Hbuf
    · iframe Hbuf; ipureintro; rfl
    simp only [hne, _root_.decide_false]
    wp_auto
    wp_apply wp_shortLimbs_otherHead dst (slice.slice v.buf' w8 v.pos' v.buf'.len)
      (head :: rest) head dq rfl hne $$ [Hbuf] with Hbuf
    · iframe Hbuf
    ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
    · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
      rw [hbytes]
      iframe Hpre Hbuf Hpost
    wp_apply hslow
      $$ [HR Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe HR Hdst Hc Hs
    wp_end
  · simp only [hlong, _root_.decide_false]
    wp_auto
    wp_apply wp_shortLimbs_otherHead dst (slice.slice v.buf' w8 v.pos' v.buf'.len)
      (head :: rest) head dq rfl hne $$ [Hbuf] with Hbuf
    · iframe Hbuf
    ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
    · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
      rw [hbytes]
      iframe Hpre Hbuf Hpost
    wp_apply hslow
      $$ [HR Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe HR Hdst Hc Hs
    wp_end

/-- Typed felt fields retain their previous pointer when a later limb fails.
    This composes the actual slow loop and dispatch for non-inline-four heads. -/
theorem wp_readFelt_nonfast_prefix_reject (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (count : w64) (items : List UnsignedWire)
    (dq : DFrac) (R : IProp GF) (E : Loc → IProp GF)
    (enc : HeadArgument head payload count) (hm : head &&& W8 224 = W8 128)
    (hne : head ≠ W8 132) (hprefix : items.length < 4) (hcount : items.length < uint.Z count)
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
  wp_apply wp_limbs_nonfast_reject c «$r0_ptr» (zero_val (GoArray w64 4)) v xs
    (payload ++ unsignedBytes items ++ rem) head dq R E hne hp hdepth hbytes
    (wp_limbsSlow_prefix_reject c «$r0_ptr» (zero_val (GoArray w64 4)) v xs payload rem head count items
      dq R E enc hm hprefix hcount hp hdepth hbytes hbad)
    $$ [HR «$r0» Hc Hs] as %err ⟨Hv, Hc, Hs, Herr⟩
  · iframe HR «$r0» Hc Hs
  wp_end

/-- An actual type error after arbitrary valid limbs preserves the old
    typed pointer and rolls back the array, retaining the inner diagnostic offset.
    The inline four-element shortcut remains a separate proof obligation. -/
theorem wp_readFelt_nonfast_limb_type (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs payload badpayload rem : List w8) (head bad : w8) (count argument : w64)
    (items : List UnsignedWire) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload count) (hm : head &&& W8 224 = W8 128)
    (hne : head ≠ W8 132) (hprefix : items.length < 4) (hcount : items.length < uint.Z count)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hdepth : sint.Z v.depth' < 32)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ unsignedBytes items ++ ((bad :: badpayload) ++ rem))
    (badenc : HeadArgument bad badpayload argument)
    (hbadmajor : bad &&& W8 224 ≠ W8 0) (hbadtag : bad &&& W8 224 ≠ W8 192) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr cursor.ErrType ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice, err ↦ cursor.DecodeError.mk kind (v.pos' + W64 (1 + payload.length + (unsignedBytes items).length)) (go!"expected %s, found %s") args ∗
       args ↦* [interface.mkOk go.string #(headDescription (W8 0)), interface.mkOk go.string #(headDescription bad)] }} := by
  wp_start_folded as ⟨#Hkind, Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  let pos := (v.pos' + W64 (1 + payload.length + (unsignedBytes items).length))
  have Hnext := input_consumption xs ((head :: payload) ++ unsignedBytes items)
    ((bad :: badpayload) ++ rem) v.pos' v.buf'.len hp Hlen.1 hbytes
  have hpnext : 0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    dsimp only [pos]; constructor <;> word
  have hsuffix : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = (bad :: badpayload) ++ rem := by
    simpa only [pos, List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm, Int.add_left_comm, Int.add_assoc] using Hnext.2.2
  have hi := input_suffix_head xs (badpayload ++ rem) (sint.nat pos) (sint.nat v.buf'.len) bad hsuffix
  have Hbad := input_consumption xs (bad :: badpayload) rem pos v.buf'.len hpnext Hlen.1 hsuffix
  have hbound : sint.Z pos + 1 + badpayload.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hbad
    omega
  have hafter : subslice (sint.nat (pos + W64 1)) (sint.nat v.buf'.len) xs = badpayload ++ rem := by
    have hn : sint.nat (pos + W64 1) = sint.nat pos + 1 := by word
    rw [hn]
    exact input_suffix_after xs [bad] (badpayload ++ rem) _ _ hsuffix
  wp_apply wp_readFelt_nonfast_prefix_reject c dst old v xs payload ((bad :: badpayload) ++ rem) head count items dq
    iprop(globalAddr cursor.ErrType ↦□ kind)
    (fun err => iprop(∃ args : GoSlice, err ↦ cursor.DecodeError.mk kind (v.pos' + W64 (1 + payload.length + (unsignedBytes items).length)) (go!"expected %s, found %s") args ∗
       args ↦* [interface.mkOk go.string #(headDescription (W8 0)), interface.mkOk go.string #(headDescription bad)]))
    enc hm hne hprefix hcount hp hdepth hbytes
    (wp_uint64_type c {v with pos' := pos, depth' := v.depth' + W64 1} xs badpayload rem bad argument dq kind badenc hbadmajor hbadtag hpnext.1 hbound hi hafter)
    $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
  · iframe Hkind Hdst Hc Hs
  wp_end

/-- An actual tag error after arbitrary valid limbs preserves the old
    typed pointer and rolls back the array, retaining the inner diagnostic offset.
    The inline four-element shortcut remains a separate proof obligation. -/
theorem wp_readFelt_nonfast_limb_tag (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs payload badpayload rem : List w8) (head bad : w8) (count argument : w64)
    (items : List UnsignedWire) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload count) (hm : head &&& W8 224 = W8 128)
    (hne : head ≠ W8 132) (hprefix : items.length < 4) (hcount : items.length < uint.Z count)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hdepth : sint.Z v.depth' < 32)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ unsignedBytes items ++ ((bad :: badpayload) ++ rem))
    (badenc : HeadArgument bad badpayload argument)
    (hbadtag : bad &&& W8 224 = W8 192) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice, err ↦ cursor.DecodeError.mk kind (v.pos' + W64 (1 + payload.length + (unsignedBytes items).length)) (go!"tag where %s is expected") args ∗
       args ↦* [interface.mkOk go.string #(headDescription (W8 0))] }} := by
  wp_start_folded as ⟨#Hkind, Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  let pos := (v.pos' + W64 (1 + payload.length + (unsignedBytes items).length))
  have Hnext := input_consumption xs ((head :: payload) ++ unsignedBytes items)
    ((bad :: badpayload) ++ rem) v.pos' v.buf'.len hp Hlen.1 hbytes
  have hpnext : 0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    dsimp only [pos]; constructor <;> word
  have hsuffix : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = (bad :: badpayload) ++ rem := by
    simpa only [pos, List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm, Int.add_left_comm, Int.add_assoc] using Hnext.2.2
  have hi := input_suffix_head xs (badpayload ++ rem) (sint.nat pos) (sint.nat v.buf'.len) bad hsuffix
  have Hbad := input_consumption xs (bad :: badpayload) rem pos v.buf'.len hpnext Hlen.1 hsuffix
  have hbound : sint.Z pos + 1 + badpayload.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hbad
    omega
  have hafter : subslice (sint.nat (pos + W64 1)) (sint.nat v.buf'.len) xs = badpayload ++ rem := by
    have hn : sint.nat (pos + W64 1) = sint.nat pos + 1 := by word
    rw [hn]
    exact input_suffix_after xs [bad] (badpayload ++ rem) _ _ hsuffix
  wp_apply wp_readFelt_nonfast_prefix_reject c dst old v xs payload ((bad :: badpayload) ++ rem) head count items dq
    iprop(globalAddr cursor.ErrUnsupported ↦□ kind)
    (fun err => iprop(∃ args : GoSlice, err ↦ cursor.DecodeError.mk kind (v.pos' + W64 (1 + payload.length + (unsignedBytes items).length)) (go!"tag where %s is expected") args ∗
       args ↦* [interface.mkOk go.string #(headDescription (W8 0))]))
    enc hm hne hprefix hcount hp hdepth hbytes
    (wp_uint64_tag c {v with pos' := pos, depth' := v.depth' + W64 1} xs badpayload rem bad argument dq kind badenc (by rw [hbadtag]; decide) hbadtag hpnext.1 hbound hi hafter)
    $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
  · iframe Hkind Hdst Hc Hs
  wp_end

/-- An actual truncated error after arbitrary valid limbs preserves the old
    typed pointer and rolls back the array, retaining the inner diagnostic offset.
    The inline four-element shortcut remains a separate proof obligation. -/
theorem wp_readFelt_nonfast_limb_truncated (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs payload rem : List w8) (head bad : w8) (count size : w64)
    (items : List UnsignedWire) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload count) (hm : head &&& W8 224 = W8 128)
    (hne : head ≠ W8 132) (hprefix : items.length < 4) (hcount : items.length < uint.Z count)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hdepth : sint.Z v.depth' < 32)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ unsignedBytes items ++ (bad :: rem))
    (hinfo : 24 ≤ uint.Z (bad &&& W8 31) ∧ uint.Z (bad &&& W8 31) ≤ 27)
    (hsize : W64 1 <<< W64 (uint.Z ((bad &&& W8 31) - W8 24)) = size)
    (htrunc : rem.length < sint.Z size) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ err ↦ cursor.DecodeError.mk kind (v.pos' + W64 (1 + payload.length + (unsignedBytes items).length)) (go!"input ends inside the head") slice.nil }} := by
  wp_start_folded as ⟨#Hkind, Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  let pos := (v.pos' + W64 (1 + payload.length + (unsignedBytes items).length))
  have Hnext := input_consumption xs ((head :: payload) ++ unsignedBytes items)
    (bad :: rem) v.pos' v.buf'.len hp Hlen.1 hbytes
  have hpnext : 0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    dsimp only [pos]; constructor <;> word
  have hsuffix : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = bad :: rem := by
    simpa only [pos, List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm, Int.add_left_comm, Int.add_assoc] using Hnext.2.2
  have hi := input_suffix_head xs (rem) (sint.nat pos) (sint.nat v.buf'.len) bad hsuffix
  have Hbad := input_consumption xs [bad] rem pos v.buf'.len hpnext Hlen.1 hsuffix
  have hlt : sint.Z pos < sint.Z v.buf'.len := by
    simp only [List.length_cons, List.length_nil] at Hbad
    omega
  have hshort : sint.Z v.buf'.len - (sint.Z pos + 1) < sint.Z size := by
    have hlen := congrArg List.length hsuffix
    simp only [subslice_length (sint.nat pos) (sint.nat v.buf'.len) xs (by omega), List.length_cons] at hlen
    simp only [sint.nat, sint.Z] at Hlen hpnext hlen htrunc ⊢
    omega
  wp_apply wp_readFelt_nonfast_prefix_reject c dst old v xs payload (bad :: rem) head count items dq
    iprop(globalAddr cursor.ErrTruncated ↦□ kind)
    (fun err => iprop(err ↦ cursor.DecodeError.mk kind (v.pos' + W64 (1 + payload.length + (unsignedBytes items).length)) (go!"input ends inside the head") slice.nil))
    enc hm hne hprefix hcount hp hdepth hbytes
    (wp_uint64_truncated c {v with pos' := pos, depth' := v.depth' + W64 1} xs bad size dq kind hpnext.1 hlt hi hinfo hsize hshort)
    $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
  · iframe Hkind Hdst Hc Hs
  wp_end

/-- Unsigned decoding at EOF reports the current offset without loading input. -/
theorem wp_uint64_eof (c : Loc) (v : cursor.Cursor) (kind : GoInterface)
    (heof : v.pos' = v.buf'.len) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Uint64")) (Val #()))
    {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends before the item") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc⟩
  wp_auto
  wp_method_call
  wp_call
  wp_auto
  simp only [heof, Int.lt_irrefl, _root_.decide_false]
  wp_auto
  wp_method_call
  wp_call
  wp_auto
  wp_method_call
  wp_call
  wp_auto
  simp only [heof, Int.le_refl, _root_.decide_true]
  wp_auto
  wp_apply wp_fail c v kind (go!"input ends before the item") slice.nil $$ [Hc]
    as %err ⟨Hc, Herr⟩
  · iframe Hc
  isimp only [heof] at Herr
  wp_end

/-- EOF after any valid prefix shorter than four limbs restores the array start
    but retains the end-of-input diagnostic, without publishing a partial felt. -/
theorem wp_readFelt_nonfast_limb_eof (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs payload : List w8) (head : w8) (count : w64) (items : List UnsignedWire)
    (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload count) (hm : head &&& W8 224 = W8 128)
    (hne : head ≠ W8 132) (hprefix : items.length < 4) (hcount : items.length < uint.Z count)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hdepth : sint.Z v.depth' < 32)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ unsignedBytes items) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.buf'.len (go!"input ends before the item") slice.nil }} := by
  wp_start_folded as ⟨#Hkind, Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  let pos := v.pos' + W64 (1 + payload.length + (unsignedBytes items).length)
  have heof : pos = v.buf'.len := by
    have hlen := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega),
      List.length_append, List.length_cons] at hlen
    dsimp only [pos]
    simp only [sint.nat] at hlen Hlen
    word
  have hleaf :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         globalAddr cursor.ErrTruncated ↦□ kind ∗
         c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Uint64")) (Val #()))
      {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
         c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗ v.buf' ↦*{dq} xs ∗
         err ↦ cursor.DecodeError.mk kind v.buf'.len (go!"input ends before the item") slice.nil }} := by
    wp_start_folded as ⟨#Hg, Hcc, Hss⟩
    wp_apply wp_uint64_eof c {v with pos' := pos, depth' := v.depth' + W64 1} kind heof
      $$ [Hcc] as %err ⟨Hcc, Herr⟩
    · iframe Hg Hcc
    isimp only [heof] at Herr
    wp_end
  wp_apply wp_readFelt_nonfast_prefix_reject c dst old v xs payload [] head count items dq
    iprop(globalAddr cursor.ErrTruncated ↦□ kind)
    (fun err => iprop(err ↦ cursor.DecodeError.mk kind v.buf'.len (go!"input ends before the item") slice.nil))
    enc hm hne hprefix hcount hp hdepth (by simpa only [List.append_nil] using hbytes) hleaf
    $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
  · iframe Hkind Hdst Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbsSlow_prefix_reject
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbs_nonfast_reject
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_nonfast_prefix_reject
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_nonfast_limb_type
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_nonfast_limb_tag
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_nonfast_limb_truncated
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_uint64_eof
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_nonfast_limb_eof
