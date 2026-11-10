import ProofCheck.Payload
import ProofCheck.KeyASCII
import ProofCheck.FieldHead
import ProofCheck.Utf8Contract

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

/-- Valid text of every supported length width is returned as the exact input
    range by keySlow, conditional on the production validator contract. -/
theorem wp_keySlow_text (validator : Utf8ValidatorContract (GF := GF))
    (c : Loc) (v : cursor.Cursor) (xs payload body rem : List w8)
    (head : w8) (dq : DFrac)
    (enc : HeadArgument head payload (W64 body.length))
    (hm : head &&& W8 224 = W8 96) (valid : Utf8Valid body)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ body ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"keySlow")) (Val #()))
    {{ RET (PairV (PairV #(slice.slice v.buf' w8 (v.pos' + W64 (1 + payload.length))
        (v.pos' + W64 (1 + payload.length + body.length))) #true) #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + body.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + body.length)) = sint.Z v.pos' + 1 + payload.length + body.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + body.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + body.length))) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have hw := headArgument_width head payload (W64 body.length) enc
  have Hnext := input_consumption xs ((head :: payload) ++ body) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hbound : sint.Z v.pos' + 1 + payload.length + body.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext ⊢
    omega
  have hheadbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by omega
  have hi := input_suffix_head xs (payload ++ body ++ rem) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hp1 : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ (body ++ rem) := by
    rw [hp1]
    exact input_suffix_after xs [head] (payload ++ (body ++ rem)) _ _
      (by simpa only [List.cons_append, List.nil_append, List.append_assoc] using hbytes)
  have hstart : sint.nat (v.pos' + W64 (1 + payload.length)) = sint.nat v.pos' + (head :: payload).length := by
    simp only [List.length_cons]; word
  have hbody : subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = body ++ rem := by
    rw [hstart]
    exact input_suffix_after xs (head :: payload) (body ++ rem) _ _ (by simpa only [List.append_assoc] using hbytes)
  have hend : sint.nat (v.pos' + W64 (1 + payload.length + body.length)) =
      sint.nat (v.pos' + W64 (1 + payload.length)) + body.length := by word
  have hbodyrange : subslice (sint.nat (v.pos' + W64 (1 + payload.length)))
      (sint.nat (v.pos' + W64 (1 + payload.length + body.length))) xs = body := by
    rw [hend]
    exact input_prefix xs body rem _ _ (by word) hbody
  wp_auto
  wp_apply wp_head_encoded c v xs payload (body ++ rem) head (W64 body.length) dq enc hp.1 hheadbound hi hafter
    $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  simp only [hm]
  wp_auto
  wp_method_call
  wp_call
  wp_auto
  have hfits : ¬ uint.Z (v.buf'.len - (v.pos' + W64 (1 + payload.length))) < uint.Z (W64 body.length) := by word
  simp only [hfits, _root_.decide_false]
  wp_auto
  have hpos : v.pos' + W64 (1 + payload.length) + W64 body.length = v.pos' + W64 (1 + payload.length + body.length) := by word
  rw [hpos]
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by word, by word, by word⟩)]
  wp_auto
  wp_apply wp_utf8_contract_range validator v.buf' xs body
    (v.pos' + W64 (1 + payload.length))
    (v.pos' + W64 (1 + payload.length + body.length)) dq
    ⟨by word, by word, by word⟩ hbodyrange $$ [Hs] as %ok ⟨Hs, %Hvalid⟩
  · iframe Hs
  have accepted : ok = true := Hvalid.mpr valid
  subst ok
  wp_auto
  iapply HΦ
  iframe Hc Hs
  ipureintro
  have length : ((head :: payload) ++ body).length = 1 + payload.length + body.length := by simp; omega
  rw [length] at Hnext
  simp only [Int.natCast_add, Int.natCast_one] at Hnext
  exact ⟨by omega, Hnext.2⟩

/-- General malformed UTF-8 key rejection through the actual slow path,
    conditional on a proof of the production validator contract. -/
theorem wp_keySlow_invalid_utf8 (validator : Utf8ValidatorContract (GF := GF))
    (c : Loc) (v : cursor.Cursor) (xs payload body rem : List w8)
    (head : w8) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload (W64 body.length))
    (hm : head &&& W8 224 = W8 96) (invalid : ¬ Utf8Valid body)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ body ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrSyntax ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"keySlow")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #slice.nil #false) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"key is not valid UTF-8") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have hw := headArgument_width head payload (W64 body.length) enc
  have Hnext := input_consumption xs ((head :: payload) ++ body) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hbound : sint.Z v.pos' + 1 + payload.length + body.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext ⊢
    omega
  have hheadbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by omega
  have hi := input_suffix_head xs (payload ++ body ++ rem) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hp1 : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ (body ++ rem) := by
    rw [hp1]
    exact input_suffix_after xs [head] (payload ++ (body ++ rem)) _ _
      (by simpa only [List.cons_append, List.nil_append, List.append_assoc] using hbytes)
  have hstart : sint.nat (v.pos' + W64 (1 + payload.length)) = sint.nat v.pos' + (head :: payload).length := by
    simp only [List.length_cons]; word
  have hbody : subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = body ++ rem := by
    rw [hstart]
    exact input_suffix_after xs (head :: payload) (body ++ rem) _ _ (by simpa only [List.append_assoc] using hbytes)
  have hend : sint.nat (v.pos' + W64 (1 + payload.length + body.length)) =
      sint.nat (v.pos' + W64 (1 + payload.length)) + body.length := by word
  have hbodyrange : subslice (sint.nat (v.pos' + W64 (1 + payload.length)))
      (sint.nat (v.pos' + W64 (1 + payload.length + body.length))) xs = body := by
    rw [hend]
    exact input_prefix xs body rem _ _ (by word) hbody
  wp_auto
  wp_apply wp_head_encoded c v xs payload (body ++ rem) head (W64 body.length) dq enc hp.1 hheadbound hi hafter
    $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  simp only [hm]
  wp_auto
  wp_method_call
  wp_call
  wp_auto
  have hfits : ¬ uint.Z (v.buf'.len - (v.pos' + W64 (1 + payload.length))) < uint.Z (W64 body.length) := by word
  simp only [hfits, _root_.decide_false]
  wp_auto
  have hpos : v.pos' + W64 (1 + payload.length) + W64 body.length = v.pos' + W64 (1 + payload.length + body.length) := by word
  rw [hpos]
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by word, by word, by word⟩)]
  wp_auto
  wp_apply wp_utf8_contract_range validator v.buf' xs body
    (v.pos' + W64 (1 + payload.length))
    (v.pos' + W64 (1 + payload.length + body.length)) dq
    ⟨by word, by word, by word⟩ hbodyrange $$ [Hs] as %ok ⟨Hs, %Hvalid⟩
  · iframe Hs
  have rejected : ok = false := by cases ok <;> simp_all
  subst ok
  wp_auto
  wp_apply wp_fail c v kind (go!"key is not valid UTF-8") slice.nil
    $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_keySlow_text
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_keySlow_invalid_utf8
