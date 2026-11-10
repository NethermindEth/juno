import ProofCheck.Payload
import ProofCheck.KeyASCII
import ProofCheck.FieldHead
import ProofCheck.KeyTextSlow

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

/-- A complete short non-ASCII key reaches keySlow with unchanged cursor/input.
    The supplied slow-path WP is explicit and may describe success or failure. -/
theorem wp_Key_nonascii_slow (c : Loc) (v : cursor.Cursor) (xs payload body rem : List w8)
    (head : w8) (dq : DFrac)
    (R : IProp GF) (Q : GoSlice → Bool → GoInterface → IProp GF)
    (enc : HeadArgument head payload (W64 body.length))
    (hm : head &&& W8 224 = W8 96) (hnotascii : ¬ ASCIIBytes body)
    (hshort : uint.Z (head &&& W8 31) < 24 ∨ head &&& W8 31 = W8 24)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ body ++ rem)
    (slow :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"keySlow")) (Val #()))
    {{ (key : GoSlice) (isText : Bool) (err : GoInterface), RET (PairV (PairV #key #isText) #err);
       Q key isText err }}) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Key")) (Val #()))
    {{ (key : GoSlice) (isText : Bool) (err : GoInterface), RET (PairV (PairV #key #isText) #err);
       Q key isText err }} := by
  wp_start as ⟨HR, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ body) rem v.pos' v.buf'.len hp Hlen.1 hbytes
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
    (((head :: payload) ++ body) ++ rem) dq head (by decide) $$ [Hbuf] with Hbuf
  · iframe Hbuf; ipureintro; rfl
  have hsub : head - W8 96 = head &&& W8 31 := by
    simpa only [hm] using byte_major_remainder head
  have htotal : sint.Z v.pos' + 1 + payload.length + body.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext ⊢
    omega
  have hklen : (body.length : Int) < 2 ^ 63 := by word
  wp_join iprop(length_ptr ↦ W64 body.length ∗ size_ptr ↦ W64 (1 + payload.length) ∗
      slice.slice v.buf' w8 v.pos' v.buf'.len ↦*{dq} ((head :: payload) ++ body ++ rem) ∗
      buf_ptr ↦ slice.slice v.buf' w8 v.pos' v.buf'.len)
      at next with [length size Hbuf buf] as ⟨Hlength, Hsize, Hbuf, buf⟩
  · rcases hshort with hsmall | hbyte
    · obtain ⟨hpay, harg⟩ := headArgument_inline_inv head payload (W64 body.length) enc hsmall
      have hnot24 : W64 (uint.Z (head - W8 96)) ≠ W64 24 := by rw [hsub]; word
      simp only [hnot24, _root_.decide_false]
      wp_auto
      wp_join_done
      have hlength : W64 (uint.Z (head - W8 96)) = W64 body.length := by rw [hsub]; exact harg.symm
      have hsize : W64 1 = W64 (1 + payload.length) := by rw [hpay]; rfl
      isimp only [hlength] at length
      iframe Hbuf length buf
      rw [← hsize]
      iframe size
    · obtain ⟨lenByte, hpay, harg⟩ := headArgument_byte_inv head payload (W64 body.length) enc hbyte
      have hlong : sint.Z (W64 1) < sint.Z (v.buf'.len - v.pos') := by
        simp only [hpay, List.length_cons, List.length_nil] at htotal
        word
      simp only [hsub, hbyte]
      wp_auto
      simp only [hlong, _root_.decide_true]
      wp_auto
      rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, hlong⟩)]
      wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _
        ((head :: payload) ++ body ++ rem) dq lenByte (by decide) $$ [Hbuf] with Hbuf
      · iframe Hbuf; ipureintro; simp [hpay]
      wp_join_done
      have hsize : W64 2 = W64 (1 + payload.length) := by rw [hpay]; rfl
      isimp only [← harg] at length
      iframe Hbuf length buf
      rw [← hsize]
      iframe size
  have hfast : body.length < 24 ∨ payload.length = 1 := by
    rcases hshort with hsmall | hbyte
    · obtain ⟨_, harg⟩ := headArgument_inline_inv head payload (W64 body.length) enc hsmall
      left; word
    · obtain ⟨lenByte, hpay, _⟩ := headArgument_byte_inv head payload (W64 body.length) enc hbyte
      right; simp [hpay]
  by_cases hsmall : body.length < 24
  all_goals
    first
    | have hlt : sint.Z (W64 body.length) < sint.Z (W64 24) := by word
      simp only [hlt, _root_.decide_true]
      wp_auto
    | have hlt : ¬ sint.Z (W64 body.length) < sint.Z (W64 24) := by word
      have hpayload : payload.length = 1 := hfast.resolve_left hsmall
      simp only [hlt, _root_.decide_false]
      wp_auto
      have hsize : W64 (1 + payload.length) = W64 2 := by rw [hpayload]; rfl
      have hsizedec : decide (W64 (1 + payload.length) = W64 2) = true := by simp only [hsize, _root_.decide_true]
      simp only [hsizedec]
      wp_auto
  all_goals
    have hgood : sint.Z (W64 (1 + payload.length) + W64 body.length) ≤ sint.Z (v.buf'.len - v.pos') := by word
    simp only [hgood, _root_.decide_true]
    wp_auto
    have hbounds : 0 ≤ sint.Z (W64 (1 + payload.length)) ∧
        sint.Z (W64 (1 + payload.length)) ≤ sint.Z (W64 (1 + payload.length) + W64 body.length) ∧
        sint.Z (W64 (1 + payload.length) + W64 body.length) ≤ sint.Z (v.buf'.len - v.pos') := by word
    have hcap : sint.Z (v.buf'.len - v.pos') ≤ sint.Z (v.buf'.cap - v.pos') := by word
    rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hbounds.1, hbounds.2.1, by omega⟩)]
    have hlo : sint.nat (W64 (1 + payload.length)) = 1 + payload.length := by word
    have hhi : sint.nat (W64 (1 + payload.length) + W64 body.length) = 1 + payload.length + body.length := by word
    have hkey : subslice (sint.nat (W64 (1 + payload.length)))
        (sint.nat (W64 (1 + payload.length) + W64 body.length)) ((head :: payload) ++ body ++ rem) = body := by
      rw [hlo, hhi]
      exact (list_slice3 (head :: payload) body rem (1 + payload.length) (1 + payload.length + body.length)
        (by simp; omega) (by omega) (by omega)).2.1
    wp_apply wp_ascii_range (slice.slice v.buf' w8 v.pos' v.buf'.len)
      ((head :: payload) ++ body ++ rem) body (W64 (1 + payload.length))
      (W64 (1 + payload.length) + W64 body.length) dq hbounds hkey
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
    wp_apply slow $$ [HR Hc Hs] as %keySlice %isText %err HQ
    · iframe HR Hc Hs
    wp_end


/-- Complete wider text heads bypass the ASCII shortcut without changing state.
    The slow-path WP is an explicit composition premise. -/
theorem wp_Key_wide_slow (c : Loc) (v : cursor.Cursor) (xs payload body rem : List w8)
    (head : w8) (dq : DFrac)
    (R : IProp GF) (Q : GoSlice → Bool → GoInterface → IProp GF)
    (enc : HeadArgument head payload (W64 body.length))
    (hm : head &&& W8 224 = W8 96) 
    (hwide : 24 ≤ uint.Z (head &&& W8 31)) (hnotbyte : head &&& W8 31 ≠ W8 24)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ body ++ rem)
    (slow :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"keySlow")) (Val #()))
    {{ (key : GoSlice) (isText : Bool) (err : GoInterface), RET (PairV (PairV #key #isText) #err);
       Q key isText err }}) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Key")) (Val #()))
    {{ (key : GoSlice) (isText : Bool) (err : GoInterface), RET (PairV (PairV #key #isText) #err);
       Q key isText err }} := by
  wp_start as ⟨HR, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ body) rem v.pos' v.buf'.len hp Hlen.1 hbytes
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
    ((head :: payload) ++ body ++ rem) dq head (by decide) $$ [Hbuf] with Hbuf
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
  wp_apply slow $$ [HR Hc Hs] as %keySlice %isText %err HQ
  · iframe HR Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Key_nonascii_slow
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Key_wide_slow
