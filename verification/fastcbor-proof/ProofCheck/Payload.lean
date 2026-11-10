import ProofCheck.FeltValid

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


/-- A complete prefix in an unread suffix identifies the corresponding bounded subrange. -/
theorem input_prefix (xs front rest : List w8) (lo hi : Nat)
    (hb : lo + front.length ≤ hi) (h : subslice lo hi xs = front ++ rest) :
    subslice lo (lo + front.length) xs = front := by
  have ht := congrArg (List.take front.length) h
  simp only [subslice_comm, List.take_take, List.take_left] at ht
  rw [Nat.min_eq_left (show front.length ≤ hi - lo by omega)] at ht
  simpa only [subslice_comm, Nat.add_sub_cancel_left] using ht

/-- A complete definite payload is borrowed at its exact offset, with the whole input preserved. -/
theorem wp_payload_encoded (c : Loc) (v : cursor.Cursor) (xs payload body rem : List w8)
    (head : w8) (dq : DFrac) (enc : HeadArgument head payload (W64 body.length))
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ body ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"payload")) (Val #(head &&& W8 224)))
    {{ RET (PairV #(slice.slice v.buf' w8 (v.pos' + W64 (1 + payload.length))
        (v.pos' + W64 (1 + payload.length + body.length))) #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + body.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜0 ≤ sint.Z (v.pos' + W64 (1 + payload.length)) ∧
         sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z (v.pos' + W64 (1 + payload.length + body.length)) ∧
         sint.Z (v.pos' + W64 (1 + payload.length + body.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length)))
           (sint.nat (v.pos' + W64 (1 + payload.length + body.length))) xs = body ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + body.length))) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have hw := headArgument_width head payload (W64 body.length) enc
  have Hnext := input_consumption xs ((head :: payload) ++ body) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hbound : sint.Z v.pos' + 1 + payload.length + body.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
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
  wp_apply wp_expect_encoded c v xs payload (body ++ rem) head (W64 body.length) dq enc hp.1 hheadbound hi hafter
    $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
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
  iapply HΦ
  iframe Hc Hs
  ipureintro
  refine ⟨by word, by word, by word, hbodyrange, ?_⟩
  simpa only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
    Int.add_comm (payload.length : Int) 1] using Hnext.2.2

/-- Declared payload lengths larger than the remaining input fail before borrowing and restore position. -/
theorem wp_payload_truncated (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem)
    (hshort : rem.length < uint.Z argument) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"payload")) (Val #(head &&& W8 224)))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"byte or text string longer than the input") slice.nil }} := by
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
  have hremaining : (rem.length : Int) = sint.Z v.buf'.len - (sint.Z v.pos' + 1 + payload.length) := by
    have hcount := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega),
      List.length_append, List.length_cons] at hcount
    simp only [sint.nat, sint.Z] at Hlen hcount hp ⊢
    omega
  wp_auto
  wp_apply wp_expect_encoded c v xs payload rem head argument dq enc hp.1 hbound hi hafter $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_method_call
  wp_call
  wp_auto
  have hbad : uint.Z (v.buf'.len - (v.pos' + W64 (1 + payload.length))) < uint.Z argument := by word
  simp only [hbad, _root_.decide_true]
  wp_auto
  wp_apply wp_fail c v kind (go!"byte or text string longer than the input") slice.nil $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc
  wp_end

/-- The public borrowed-byte API preserves the input and returns its exact payload range. -/
theorem wp_bytesBorrowed_encoded (c : Loc) (v : cursor.Cursor) (xs payload body rem : List w8)
    (head : w8) (dq : DFrac) (hmajor : head &&& W8 224 = W8 64) (enc : HeadArgument head payload (W64 body.length))
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ body ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"BytesBorrowed")) (Val #()))
    {{ RET (PairV #(slice.slice v.buf' w8 (v.pos' + W64 (1 + payload.length))
        (v.pos' + W64 (1 + payload.length + body.length))) #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + body.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜0 ≤ sint.Z (v.pos' + W64 (1 + payload.length)) ∧
         sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z (v.pos' + W64 (1 + payload.length + body.length)) ∧
         sint.Z (v.pos' + W64 (1 + payload.length + body.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length)))
           (sint.nat (v.pos' + W64 (1 + payload.length + body.length))) xs = body ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + body.length))) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨Hc, Hs⟩
  wp_auto
  rw [← hmajor]
  wp_apply wp_payload_encoded c v xs payload body rem head dq enc hp hbytes $$ [Hc Hs] with ⟨Hc, Hs, %Hrange⟩
  · iframe Hc Hs
  wp_end

/-- BytesBorrowed propagates payload truncation with its documented zero return and rollback. -/
theorem wp_bytesBorrowed_truncated (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (hmajor : head &&& W8 224 = W8 64) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem)
    (hshort : rem.length < uint.Z argument) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"BytesBorrowed")) (Val #()))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"byte or text string longer than the input") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  rw [← hmajor]
  wp_apply wp_payload_truncated c v xs payload rem head argument dq kind enc hp hbytes hshort
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hc Hs Hkind
  wp_end

/-- String propagates payload truncation with its documented zero return and rollback. -/
theorem wp_string_truncated (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (hmajor : head &&& W8 224 = W8 96) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem)
    (hshort : rem.length < uint.Z argument) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"String")) (Val #()))
    {{ (err : Loc), RET (PairV #(go!"") #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"byte or text string longer than the input") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  rw [← hmajor]
  wp_apply wp_payload_truncated c v xs payload rem head argument dq kind enc hp hbytes hshort
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hc Hs Hkind
  wp_end


/-- A truncated string field preserves its previous value and propagates the owned payload error. -/
theorem wp_readString_truncated (c dst : Loc) (old : GoString) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (hmajor : head &&& W8 224 = W8 96) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem)
    (hshort : rem.length < uint.Z argument) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadString)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"byte or text string longer than the input") slice.nil }} := by
  wp_start as ⟨#Hkind, Hdst, Hc, Hs⟩
  have hi := input_suffix_head xs (payload ++ rem) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hn0 : head ≠ W8 246 := by intro h; subst head; contradiction
  have hn1 : head ≠ W8 247 := by intro h; subst head; contradiction
  wp_auto
  wp_apply wp_null_other c v xs head dq hp.1 hi hn0 hn1 $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_string_truncated c v xs payload rem head hmajor argument dq kind enc hp hbytes hshort
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hc Hs Hkind
  wp_end


end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.input_prefix
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_payload_encoded
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_payload_truncated

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bytesBorrowed_encoded
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bytesBorrowed_truncated
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_string_truncated

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readString_truncated
