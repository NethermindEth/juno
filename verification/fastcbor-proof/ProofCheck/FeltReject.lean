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


end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbsSlow_prefix_reject
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbs_nonfast_reject
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_uint64_eof
