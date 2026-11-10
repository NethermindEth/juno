import ProofCheck.ASCII

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

/-- A scan borrows only the requested range and restores the entire input. -/
theorem wp_ascii_range (data : GoSlice) (xs key : List w8) (low high : w64) (dq : DFrac)
    (hbound : 0 ≤ sint.Z low ∧ sint.Z low ≤ sint.Z high ∧ sint.Z high ≤ sint.Z data.len)
    (hkey : subslice (sint.nat low) (sint.nat high) xs = key) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗ data ↦*{dq} xs }}
      (App (Val (@! cursor.ascii)) (Val #(slice.slice data w8 low high)))
    {{ (ok : Bool), RET #ok; data ↦*{dq} xs ∗ ⌜ok = true ↔ ASCIIBytes key⌝ }} := by
  iintro %Φ ⟨#Hpkg, Hs⟩ HΦ
  icases (ownSlice_slice low high data dq xs hbound).1 $$ Hs with ⟨Hpre, Hkey, Hpost⟩
  isimp only [hkey] at Hkey
  wp_apply wp_ascii (slice.slice data w8 low high) key dq $$ [Hkey] as %ok ⟨Hkey, %Hok⟩
  · iframe Hkey
  ihave Hs : (data ↦*{dq} xs : IProp GF) $$ [Hpre Hkey Hpost]
  · iapply (ownSlice_slice low high data dq xs hbound).2
    rw [hkey]
    iframe Hpre Hkey Hpost
  iapply HΦ
  iframe Hs
  ipureintro
  exact Hok

/-- Short ASCII text keys are returned as the exact borrowed input range.
    Both inline and one-byte lengths are covered, including non-preferred encodings. -/
theorem wp_Key_ascii (c : Loc) (v : cursor.Cursor) (xs payload key rem : List w8)
    (head : w8) (dq : DFrac)
    (enc : HeadArgument head payload (W64 key.length))
    (hm : head &&& W8 224 = W8 96)
    (hshort : uint.Z (head &&& W8 31) < 24 ∨ head &&& W8 31 = W8 24)
    (hascii : ASCIIBytes key)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ key ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Key")) (Val #()))
    {{ RET (PairV (PairV #(slice.slice (slice.slice v.buf' w8 v.pos' v.buf'.len) w8
          (W64 (1 + payload.length)) (W64 (1 + payload.length + key.length))) #true) #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + key.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + key.length)) = sint.Z v.pos' + 1 + payload.length + key.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + key.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + key.length))) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ key) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hheadbound : sint.Z v.pos' < sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
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
    (((head :: payload) ++ key) ++ rem) dq head (by decide) $$ [Hbuf] with Hbuf
  · iframe Hbuf; ipureintro; rfl
  have hsub : head - W8 96 = head &&& W8 31 := by
    simpa only [hm] using byte_major_remainder head
  have htotal : sint.Z v.pos' + 1 + payload.length + key.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    omega
  have hklen : (key.length : Int) < 2 ^ 63 := by word
  wp_join iprop(length_ptr ↦ W64 key.length ∗ size_ptr ↦ W64 (1 + payload.length) ∗
      slice.slice v.buf' w8 v.pos' v.buf'.len ↦*{dq} ((head :: payload) ++ key ++ rem) ∗
      buf_ptr ↦ slice.slice v.buf' w8 v.pos' v.buf'.len)
      at next with [length size Hbuf buf] as ⟨Hlength, Hsize, Hbuf, buf⟩
  · rcases hshort with hsmall | hbyte
    · obtain ⟨hpay, harg⟩ := headArgument_inline_inv head payload (W64 key.length) enc hsmall
      have hnot24 : W64 (uint.Z (head - W8 96)) ≠ W64 24 := by rw [hsub]; word
      simp only [hnot24, _root_.decide_false]
      wp_auto
      wp_join_done
      have hlength : W64 (uint.Z (head - W8 96)) = W64 key.length := by rw [hsub]; exact harg.symm
      have hsize : W64 1 = W64 (1 + payload.length) := by rw [hpay]; rfl
      isimp only [hlength] at length
      iframe Hbuf length buf
      rw [← hsize]
      iframe size
    · obtain ⟨b, hpay, harg⟩ := headArgument_byte_inv head payload (W64 key.length) enc hbyte
      have hlong : sint.Z (W64 1) < sint.Z (v.buf'.len - v.pos') := by
        simp only [hpay, List.length_cons, List.length_nil] at htotal
        word
      simp only [hsub, hbyte]
      wp_auto
      simp only [hlong, _root_.decide_true]
      wp_auto
      rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, hlong⟩)]
      wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _
        ((head :: payload) ++ key ++ rem) dq b (by decide) $$ [Hbuf] with Hbuf
      · iframe Hbuf; ipureintro; simp [hpay]
      wp_join_done
      have hsize : W64 2 = W64 (1 + payload.length) := by rw [hpay]; rfl
      isimp only [← harg] at length
      iframe Hbuf length buf
      rw [← hsize]
      iframe size
  have hfast : key.length < 24 ∨ payload.length = 1 := by
    rcases hshort with hsmall | hbyte
    · obtain ⟨_, harg⟩ := headArgument_inline_inv head payload (W64 key.length) enc hsmall
      left; word
    · obtain ⟨b, hpay, _⟩ := headArgument_byte_inv head payload (W64 key.length) enc hbyte
      right; simp [hpay]
  by_cases hsmall : key.length < 24
  all_goals
    first
    | have hlt : sint.Z (W64 key.length) < sint.Z (W64 24) := by word
      simp only [hlt, _root_.decide_true]
      wp_auto
    | have hlt : ¬ sint.Z (W64 key.length) < sint.Z (W64 24) := by word
      have hpayload : payload.length = 1 := hfast.resolve_left hsmall
      simp only [hlt, _root_.decide_false]
      wp_auto
      have hsize : W64 (1 + payload.length) = W64 2 := by rw [hpayload]; rfl
      have hsizedec : decide (W64 (1 + payload.length) = W64 2) = true := by simp only [hsize, _root_.decide_true]
      simp only [hsizedec]
      wp_auto
  all_goals
    have hgood : sint.Z (W64 (1 + payload.length) + W64 key.length) ≤ sint.Z (v.buf'.len - v.pos') := by word
    simp only [hgood, _root_.decide_true]
    wp_auto
    have hbounds : 0 ≤ sint.Z (W64 (1 + payload.length)) ∧
        sint.Z (W64 (1 + payload.length)) ≤ sint.Z (W64 (1 + payload.length) + W64 key.length) ∧
        sint.Z (W64 (1 + payload.length) + W64 key.length) ≤ sint.Z (v.buf'.len - v.pos') := by word
    have hcap : sint.Z (v.buf'.len - v.pos') ≤ sint.Z (v.buf'.cap - v.pos') := by word
    rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hbounds.1, hbounds.2.1, by omega⟩)]
    have hlo : sint.nat (W64 (1 + payload.length)) = 1 + payload.length := by word
    have hhi : sint.nat (W64 (1 + payload.length) + W64 key.length) = 1 + payload.length + key.length := by word
    have hkey : subslice (sint.nat (W64 (1 + payload.length)))
        (sint.nat (W64 (1 + payload.length) + W64 key.length)) ((head :: payload) ++ key ++ rem) = key := by
      rw [hlo, hhi]
      exact (list_slice3 (head :: payload) key rem (1 + payload.length) (1 + payload.length + key.length)
        (by simp; omega) (by omega) (by omega)).2.1
    wp_apply wp_ascii_range (slice.slice v.buf' w8 v.pos' v.buf'.len)
      ((head :: payload) ++ key ++ rem) key (W64 (1 + payload.length))
      (W64 (1 + payload.length) + W64 key.length) dq hbounds hkey
      $$ [Hbuf] as %ok ⟨Hbuf, %Hok⟩
    · iframe Hbuf
    have hok : ok = true := Hok.mpr hascii
    subst ok
    wp_auto
    rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hbounds.1, hbounds.2.1, by omega⟩)]
    wp_auto
    ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
    · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
      rw [hbytes]
      iframe Hpre Hbuf Hpost
    have hsum : W64 (1 + payload.length) + W64 key.length = W64 (1 + payload.length + key.length) := by word
    isimp only [hsum] at Hc
    simp only [hsum]
    iapply HΦ
    iframe Hc Hs
    ipureintro
    simpa only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
      Int.add_comm (payload.length : Int) 1, Int.add_assoc] using Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_ascii_range
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Key_ascii
