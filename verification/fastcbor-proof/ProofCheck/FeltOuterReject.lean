import ProofCheck.SkipHeadFault

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


/-- The typed felt reader preserves its prior pointer and original input/cursor
    on an outer type error, including at the nesting limit. -/
theorem wp_readFelt_head_type (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (argument : w64)
    (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument)
    (hm : head &&& W8 224 ≠ W8 128) (htag : head &&& W8 224 ≠ W8 192)
    (hn : head ≠ W8 246) (hu : head ≠ W8 247)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr cursor.ErrType ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice, err ↦ cursor.DecodeError.mk kind v.pos' (go!"expected %s, found %s") args ∗
       args ↦* [interface.mkOk go.string #(headDescription (W8 128)), interface.mkOk go.string #(headDescription head)] }} := by
  wp_start_folded as ⟨#Hkind, Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hi := input_suffix_head xs (payload ++ rem) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have Hnext := input_consumption xs (head :: payload) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    omega
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem := by
    have hn : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
    rw [hn]
    exact input_suffix_after xs [head] (payload ++ rem) _ _ hbytes
  have hne : head ≠ W8 132 := by intro h; subst head; contradiction
  wp_apply wp_readFelt_expect_reject c dst old v xs (payload ++ rem) head dq
    iprop(globalAddr cursor.ErrType ↦□ kind) (fun err => iprop(∃ args : GoSlice, err ↦ cursor.DecodeError.mk kind v.pos' (go!"expected %s, found %s") args ∗
       args ↦* [interface.mkOk go.string #(headDescription (W8 128)), interface.mkOk go.string #(headDescription head)]))
    hne hn hu hp hbytes
    (wp_expect_type c v xs payload rem head (W8 128) argument dq kind enc (by decide) hm htag hp.1 hbound hi hafter)
    $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
  · iframe Hkind Hdst Hc Hs
  wp_end

/-- The typed felt reader preserves its prior pointer and original input/cursor
    on an outer tag error, including at the nesting limit. -/
theorem wp_readFelt_head_tag (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (argument : w64)
    (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument)
    (htag : head &&& W8 224 = W8 192)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice, err ↦ cursor.DecodeError.mk kind v.pos' (go!"tag where %s is expected") args ∗
       args ↦* [interface.mkOk go.string #(headDescription (W8 128))] }} := by
  wp_start_folded as ⟨#Hkind, Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hi := input_suffix_head xs (payload ++ rem) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have Hnext := input_consumption xs (head :: payload) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    omega
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem := by
    have hn : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
    rw [hn]
    exact input_suffix_after xs [head] (payload ++ rem) _ _ hbytes
  have hne : head ≠ W8 132 := by intro h; subst head; contradiction
  have hm : head &&& W8 224 ≠ W8 128 := by rw [htag]; decide
  have hn : head ≠ W8 246 := by intro h; subst head; contradiction
  have hu : head ≠ W8 247 := by intro h; subst head; contradiction
  wp_apply wp_readFelt_expect_reject c dst old v xs (payload ++ rem) head dq
    iprop(globalAddr cursor.ErrUnsupported ↦□ kind) (fun err => iprop(∃ args : GoSlice, err ↦ cursor.DecodeError.mk kind v.pos' (go!"tag where %s is expected") args ∗
       args ↦* [interface.mkOk go.string #(headDescription (W8 128))]))
    hne hn hu hp hbytes
    (wp_expect_tag c v xs payload rem head (W8 128) argument dq kind enc (by decide) hm htag hp.1 hbound hi hafter)
    $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
  · iframe Hkind Hdst Hc Hs
  wp_end

/-- The typed felt reader preserves its prior pointer and original input/cursor
    on an incomplete outer head, including at the nesting limit. -/
theorem wp_readFelt_head_truncated (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs rem : List w8) (head : w8) (size : w64)
    (dq : DFrac) (kind : GoInterface)
    (hinfo : 24 ≤ uint.Z (head &&& W8 31) ∧ uint.Z (head &&& W8 31) ≤ 27)
    (hsize : W64 1 <<< W64 (uint.Z ((head &&& W8 31) - W8 24)) = size)
    (htrunc : rem.length < sint.Z size)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends inside the head") slice.nil }} := by
  wp_start_folded as ⟨#Hkind, Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hi := input_suffix_head xs (rem) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have Hnext := input_consumption xs [head] rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by
    simp only [List.length_cons, List.length_nil] at Hnext
    omega
  have hshort : sint.Z v.buf'.len - (sint.Z v.pos' + 1) < sint.Z size := by
    have hlen := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega), List.length_cons] at hlen
    simp only [sint.nat, sint.Z] at Hlen hp hlen htrunc ⊢
    omega
  have hne : head ≠ W8 132 := by intro h; subst head; contradiction
  have hn : head ≠ W8 246 := by intro h; subst head; contradiction
  have hu : head ≠ W8 247 := by intro h; subst head; contradiction
  wp_apply wp_readFelt_expect_reject c dst old v xs (rem) head dq
    iprop(globalAddr cursor.ErrTruncated ↦□ kind) (fun err => iprop(err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends inside the head") slice.nil))
    hne hn hu hp hbytes
    (wp_expect_truncated c v xs head (W8 128) size dq kind (by decide) hp.1 hlt hi hinfo hsize hshort)
    $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
  · iframe Hkind Hdst Hc Hs
  wp_end

/-- EOF is not null and does not load from the input slice. -/
theorem wp_null_eof (c : Loc) (v : cursor.Cursor) (heof : v.pos' = v.buf'.len) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗ c ↦ v }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Null")) (Val #()))
    {{ RET #false; c ↦ v }} := by
  wp_start as Hc
  wp_auto
  simp only [heof, Int.lt_irrefl, _root_.decide_false]
  wp_auto
  wp_end

/-- Every expectation reports EOF before checking the requested major type. -/
theorem wp_expect_eof (c : Loc) (v : cursor.Cursor) (want : w8) (kind : GoInterface)
    (heof : v.pos' = v.buf'.len) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"expect")) (Val #want))
    {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends before the item") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc⟩
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
  wp_apply wp_fail c v kind (go!"input ends before the item") slice.nil $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc
  isimp only [heof] at Herr
  wp_end

/-- The short shortcut examines only the length of an empty slice and no destination. -/
theorem wp_shortLimbs_eof (dst : Loc) (data : GoSlice) (heof : data.len = W64 0) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (App (Val (@! cursor.shortLimbs)) (Val #data)) (Val #dst))
    {{ RET #(W64 0); True }} := by
  wp_start
  wp_auto
  simp only [heof]
  wp_auto
  wp_end

/-- The slow felt reader reports EOF at the original offset and never writes dst. -/
theorem wp_limbsSlow_eof (c dst : Loc) (old : GoArray w64 4) (v : cursor.Cursor)
    (kind : GoInterface) (heof : v.pos' = v.buf'.len) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ dst ↦ old ∗ c ↦ v }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"limbsSlow")) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends before the item") slice.nil }} := by
  wp_start as ⟨#Hkind, Hdst, Hc⟩
  wp_auto
  wp_apply wp_expect_eof c v (W8 128) kind heof $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hkind Hc
  wp_end

/-- Both depth branches of public Limbs retain the EOF diagnostic and old output. -/
theorem wp_limbs_eof (c dst : Loc) (old : GoArray w64 4) (v : cursor.Cursor)
    (xs : List w8) (dq : DFrac) (kind : GoInterface) (heof : v.pos' = v.buf'.len) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Limbs")) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends before the item") slice.nil }} := by
  wp_start as ⟨#Hkind, Hdst, Hc, Hs⟩
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  wp_auto
  by_cases hd : sint.Z (W64 32) ≤ sint.Z v.depth'
  · simp only [hd, _root_.decide_true]
    wp_auto
    wp_apply wp_limbsSlow_eof c dst old v kind heof $$ [Hdst Hc] as %err ⟨Hdst, Hc, Herr⟩
    · iframe Hkind Hdst Hc
    wp_end
  · simp only [hd, _root_.decide_false]
    wp_auto
    rw [ite_eq_left_of_eq_true _ _ (eq_true (by rw [heof]; exact ⟨Hwf.1, Int.le_refl _, Hwf.2⟩))]
    wp_auto
    simp only [heof, BitVec.sub_self]
    wp_auto
    have hz : (slice.slice v.buf' w8 v.buf'.len v.buf'.len).len = W64 0 := by
      simp only [slice.slice, BitVec.sub_self]
      rfl
    wp_apply wp_shortLimbs_eof dst (slice.slice v.buf' w8 v.buf'.len v.buf'.len) hz
    wp_apply wp_limbsSlow_eof c dst old v kind heof $$ [Hdst Hc] as %err ⟨Hdst, Hc, Herr⟩
    · iframe Hkind Hdst Hc
    isimp only [heof] at Herr
    wp_end

/-- A missing nullable felt is an error, not a null value; the old pointer survives. -/
theorem wp_readFelt_eof (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs : List w8) (dq : DFrac) (kind : GoInterface) (heof : v.pos' = v.buf'.len) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends before the item") slice.nil }} := by
  wp_start as ⟨#Hkind, Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_null_eof c v heof $$ [Hc] with Hc
  · iframe Hc
  wp_apply wp_limbs_eof c «$r0_ptr» (zero_val (GoArray w64 4)) v xs dq kind heof
    $$ [«$r0» Hc Hs] as %err ⟨Hv, Hc, Hs, Herr⟩
  · iframe Hkind «$r0» Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_head_type
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_head_tag
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_head_truncated
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_null_eof
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_expect_eof
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_shortLimbs_eof
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbsSlow_eof
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbs_eof
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_eof
