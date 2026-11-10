import ProofCheck.Payload
import ProofCheck.KeyASCII
import ProofCheck.FieldHead
import ProofCheck.Utf8Reject

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

/-- Complete text keys with an illegal initial UTF-8 byte fail atomically,
    for every supported length encoding and arbitrary following bytes. -/
theorem wp_keySlow_bad_lead (c : Loc) (v : cursor.Cursor) (xs payload tail rem : List w8)
    (head b : w8) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload (W64 (b :: tail).length))
    (hm : head &&& W8 224 = W8 96) (bad : Utf8BadLead b)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ (b :: tail) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrSyntax ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"keySlow")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #slice.nil #false) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"key is not valid UTF-8") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have hw := headArgument_width head payload (W64 (b :: tail).length) enc
  have Hnext := input_consumption xs ((head :: payload) ++ (b :: tail)) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hbound : sint.Z v.pos' + 1 + payload.length + (b :: tail).length ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext ⊢
    omega
  have hheadbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by omega
  have hi := input_suffix_head xs (payload ++ (b :: tail) ++ rem) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hp1 : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ ((b :: tail) ++ rem) := by
    rw [hp1]
    exact input_suffix_after xs [head] (payload ++ ((b :: tail) ++ rem)) _ _
      (by simpa only [List.cons_append, List.nil_append, List.append_assoc] using hbytes)
  have hstart : sint.nat (v.pos' + W64 (1 + payload.length)) = sint.nat v.pos' + (head :: payload).length := by
    simp only [List.length_cons]; word
  have hbody : subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = (b :: tail) ++ rem := by
    rw [hstart]
    exact input_suffix_after xs (head :: payload) ((b :: tail) ++ rem) _ _ (by simpa only [List.append_assoc] using hbytes)
  have hend : sint.nat (v.pos' + W64 (1 + payload.length + (b :: tail).length)) =
      sint.nat (v.pos' + W64 (1 + payload.length)) + (b :: tail).length := by word
  have hbodyrange : subslice (sint.nat (v.pos' + W64 (1 + payload.length)))
      (sint.nat (v.pos' + W64 (1 + payload.length + (b :: tail).length))) xs = b :: tail := by
    rw [hend]
    exact input_prefix xs (b :: tail) rem _ _ (by word) hbody
  wp_auto
  wp_apply wp_head_encoded c v xs payload ((b :: tail) ++ rem) head (W64 (b :: tail).length) dq enc hp.1 hheadbound hi hafter
    $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  simp only [hm]
  wp_auto
  wp_method_call
  wp_call
  wp_auto
  have hfits : ¬ uint.Z (v.buf'.len - (v.pos' + W64 (1 + payload.length))) < uint.Z (W64 (b :: tail).length) := by word
  simp only [hfits, _root_.decide_false]
  wp_auto
  have hpos : v.pos' + W64 (1 + payload.length) + W64 (b :: tail).length = v.pos' + W64 (1 + payload.length + (b :: tail).length) := by word
  rw [hpos]
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by word, by word, by word⟩)]
  wp_auto
  wp_apply wp_utf8_bad_lead_range v.buf' xs b tail
    (v.pos' + W64 (1 + payload.length))
    (v.pos' + W64 (1 + payload.length + (b :: tail).length)) dq bad
    ⟨by word, by word, by word⟩ hbodyrange $$ [Hs] with Hs
  · iframe Hs
  wp_apply wp_fail c v kind (go!"key is not valid UTF-8") slice.nil
    $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc
  wp_end

/-- The short-key ASCII shortcut rejects a non-ASCII starter, then the real
    slow reader supplies the UTF-8 error without changing the cursor. -/
theorem wp_Key_short_bad_lead (c : Loc) (v : cursor.Cursor) (xs payload tail rem : List w8)
    (head b : w8) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload (W64 (b :: tail).length))
    (hm : head &&& W8 224 = W8 96) (bad : Utf8BadLead b)
    (hshort : uint.Z (head &&& W8 31) < 24 ∨ head &&& W8 31 = W8 24)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ (b :: tail) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrSyntax ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Key")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #slice.nil #false) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"key is not valid UTF-8") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  have hnotascii : ¬ ASCIIBytes (b :: tail) := by
    intro h
    have hb := h b (by simp)
    unfold Utf8BadLead at bad
    omega
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ (b :: tail)) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hheadbound : sint.Z v.pos' < sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext ⊢
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
    (((head :: payload) ++ (b :: tail)) ++ rem) dq head (by decide) $$ [Hbuf] with Hbuf
  · iframe Hbuf; ipureintro; rfl
  have hsub : head - W8 96 = head &&& W8 31 := by
    simpa only [hm] using byte_major_remainder head
  have htotal : sint.Z v.pos' + 1 + payload.length + (b :: tail).length ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext ⊢
    omega
  have hklen : ((b :: tail).length : Int) < 2 ^ 63 := by word
  wp_join iprop(length_ptr ↦ W64 (b :: tail).length ∗ size_ptr ↦ W64 (1 + payload.length) ∗
      slice.slice v.buf' w8 v.pos' v.buf'.len ↦*{dq} ((head :: payload) ++ (b :: tail) ++ rem) ∗
      buf_ptr ↦ slice.slice v.buf' w8 v.pos' v.buf'.len)
      at next with [length size Hbuf buf] as ⟨Hlength, Hsize, Hbuf, buf⟩
  · rcases hshort with hsmall | hbyte
    · obtain ⟨hpay, harg⟩ := headArgument_inline_inv head payload (W64 (b :: tail).length) enc hsmall
      have hnot24 : W64 (uint.Z (head - W8 96)) ≠ W64 24 := by rw [hsub]; word
      simp only [hnot24, _root_.decide_false]
      wp_auto
      wp_join_done
      have hlength : W64 (uint.Z (head - W8 96)) = W64 (b :: tail).length := by rw [hsub]; exact harg.symm
      have hsize : W64 1 = W64 (1 + payload.length) := by rw [hpay]; rfl
      isimp only [hlength] at length
      iframe Hbuf length buf
      rw [← hsize]
      iframe size
    · obtain ⟨lenByte, hpay, harg⟩ := headArgument_byte_inv head payload (W64 (b :: tail).length) enc hbyte
      have hlong : sint.Z (W64 1) < sint.Z (v.buf'.len - v.pos') := by
        simp only [hpay, List.length_cons, List.length_nil] at htotal
        word
      simp only [hsub, hbyte]
      wp_auto
      simp only [hlong, _root_.decide_true]
      wp_auto
      rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, hlong⟩)]
      wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _
        ((head :: payload) ++ (b :: tail) ++ rem) dq lenByte (by decide) $$ [Hbuf] with Hbuf
      · iframe Hbuf; ipureintro; simp [hpay]
      wp_join_done
      have hsize : W64 2 = W64 (1 + payload.length) := by rw [hpay]; rfl
      isimp only [← harg] at length
      iframe Hbuf length buf
      rw [← hsize]
      iframe size
  have hfast : (b :: tail).length < 24 ∨ payload.length = 1 := by
    rcases hshort with hsmall | hbyte
    · obtain ⟨_, harg⟩ := headArgument_inline_inv head payload (W64 (b :: tail).length) enc hsmall
      left; word
    · obtain ⟨lenByte, hpay, _⟩ := headArgument_byte_inv head payload (W64 (b :: tail).length) enc hbyte
      right; simp [hpay]
  by_cases hsmall : (b :: tail).length < 24
  all_goals
    first
    | have hlt : sint.Z (W64 (b :: tail).length) < sint.Z (W64 24) := by word
      simp only [hlt, _root_.decide_true]
      wp_auto
    | have hlt : ¬ sint.Z (W64 (b :: tail).length) < sint.Z (W64 24) := by word
      have hpayload : payload.length = 1 := hfast.resolve_left hsmall
      simp only [hlt, _root_.decide_false]
      wp_auto
      have hsize : W64 (1 + payload.length) = W64 2 := by rw [hpayload]; rfl
      have hsizedec : decide (W64 (1 + payload.length) = W64 2) = true := by simp only [hsize, _root_.decide_true]
      simp only [hsizedec]
      wp_auto
  all_goals
    have hgood : sint.Z (W64 (1 + payload.length) + W64 (b :: tail).length) ≤ sint.Z (v.buf'.len - v.pos') := by word
    simp only [hgood, _root_.decide_true]
    wp_auto
    have hbounds : 0 ≤ sint.Z (W64 (1 + payload.length)) ∧
        sint.Z (W64 (1 + payload.length)) ≤ sint.Z (W64 (1 + payload.length) + W64 (b :: tail).length) ∧
        sint.Z (W64 (1 + payload.length) + W64 (b :: tail).length) ≤ sint.Z (v.buf'.len - v.pos') := by word
    have hcap : sint.Z (v.buf'.len - v.pos') ≤ sint.Z (v.buf'.cap - v.pos') := by word
    rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hbounds.1, hbounds.2.1, by omega⟩)]
    have hlo : sint.nat (W64 (1 + payload.length)) = 1 + payload.length := by word
    have hhi : sint.nat (W64 (1 + payload.length) + W64 (b :: tail).length) = 1 + payload.length + (b :: tail).length := by word
    have hkey : subslice (sint.nat (W64 (1 + payload.length)))
        (sint.nat (W64 (1 + payload.length) + W64 (b :: tail).length)) ((head :: payload) ++ (b :: tail) ++ rem) = (b :: tail) := by
      rw [hlo, hhi]
      exact (list_slice3 (head :: payload) (b :: tail) rem (1 + payload.length) (1 + payload.length + (b :: tail).length)
        (by simp; omega) (by omega) (by omega)).2.1
    wp_apply wp_ascii_range (slice.slice v.buf' w8 v.pos' v.buf'.len)
      ((head :: payload) ++ (b :: tail) ++ rem) (b :: tail) (W64 (1 + payload.length))
      (W64 (1 + payload.length) + W64 (b :: tail).length) dq hbounds hkey
      $$ [Hbuf] as %ok ⟨Hbuf, %Hok⟩
    · iframe Hbuf
    have hok : ok = false := by
      cases ok with
      | false => rfl
      | true => exact False.elim (hnotascii (Hok.mp rfl))
    subst ok
    wp_auto
    ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
    · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
      rw [hbytes]
      iframe Hpre Hbuf Hpost
    wp_apply wp_keySlow_bad_lead c v xs payload tail rem head b dq kind enc hm bad hp hbytes
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hc Hs Hkind
    wp_end

/-- Wider length heads bypass the short-key shortcut. -/
theorem wp_Key_wide_bad_lead (c : Loc) (v : cursor.Cursor) (xs payload tail rem : List w8)
    (head b : w8) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload (W64 (b :: tail).length))
    (hm : head &&& W8 224 = W8 96) (bad : Utf8BadLead b)
    (hwide : 24 ≤ uint.Z (head &&& W8 31)) (hnotbyte : head &&& W8 31 ≠ W8 24)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ (b :: tail) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrSyntax ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Key")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #slice.nil #false) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"key is not valid UTF-8") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ (b :: tail)) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hheadbound : sint.Z v.pos' < sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    omega
  have hsub : head - W8 96 = head &&& W8 31 := by
    simpa only [hm] using byte_major_remainder head
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
    ((head :: payload) ++ (b :: tail) ++ rem) dq head (by decide) $$ [Hbuf] with Hbuf
  · iframe Hbuf; ipureintro; rfl
  have hnot24 : W64 (uint.Z (head - W8 96)) ≠ W64 24 := by rw [hsub]; word
  simp only [hnot24, _root_.decide_false]
  wp_auto
  have hnotshort : ¬ sint.Z (W64 (uint.Z (head - W8 96))) < sint.Z (W64 24) := by rw [hsub]; word
  simp only [hnotshort, _root_.decide_false]
  wp_auto
  ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
  · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
    rw [hbytes]
    iframe Hpre Hbuf Hpost
  wp_apply wp_keySlow_bad_lead c v xs payload tail rem head b dq kind enc hm bad hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hc Hs Hkind
  wp_end

/-- Public Key rejection for every supported definite length width. -/
theorem wp_Key_bad_lead (c : Loc) (v : cursor.Cursor) (xs payload tail rem : List w8)
    (head b : w8) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload (W64 (b :: tail).length))
    (hm : head &&& W8 224 = W8 96) (bad : Utf8BadLead b)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ (b :: tail) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrSyntax ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Key")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #slice.nil #false) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"key is not valid UTF-8") slice.nil }} := by
  by_cases hshort : uint.Z (head &&& W8 31) < 24 ∨ head &&& W8 31 = W8 24
  · exact wp_Key_short_bad_lead c v xs payload tail rem head b dq kind enc hm bad hshort hp hbytes
  · exact wp_Key_wide_bad_lead c v xs payload tail rem head b dq kind enc hm bad
      (by omega) (fun h => hshort (Or.inr h)) hp hbytes

/-- Alternate field lookup propagates the exact UTF-8 error. -/
theorem wp_otherField_bad_lead (c st : Loc) (v : cursor.Cursor) (xs payload tail rem : List w8)
    (head b : w8) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload (W64 (b :: tail).length))
    (hm : head &&& W8 224 = W8 96) (bad : Utf8BadLead b)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ (b :: tail) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrSyntax ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"otherField")) (Val #st))
    {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"key is not valid UTF-8") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_Key_bad_lead c v xs payload tail rem head b dq kind enc hm bad hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hc Hs Hkind
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_keySlow_bad_lead

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Key_short_bad_lead

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Key_wide_bad_lead

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Key_bad_lead

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_otherField_bad_lead
