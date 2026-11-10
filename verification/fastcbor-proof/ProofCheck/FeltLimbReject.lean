import ProofCheck.FeltInlineReject

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

/-- A non-unsigned major type cannot enter any short-limb integer case. -/
theorem unsignedReject_nonuint (head : w8) (rest : List w8)
    (hm : head &&& W8 224 ≠ W8 0) : UnsignedReject (head :: rest) := by
  have hbound : 32 ≤ uint.Z head := by
    have h : ∀ b : w8, b &&& W8 224 ≠ W8 0 → 32 ≤ uint.Z b := by decide
    exact h head hm
  refine ⟨by omega, ?_, ?_, ?_, ?_⟩
  all_goals intro heq; subst head; contradiction

/-- Every incomplete extended argument is rejected by the short-limb parser,
    including non-unsigned heads whose payload would otherwise be incomplete. -/
theorem unsignedReject_truncated (head : w8) (rest : List w8) (size : w64)
    (hinfo : 24 ≤ uint.Z (head &&& W8 31) ∧ uint.Z (head &&& W8 31) ≤ 27)
    (hsize : W64 1 <<< W64 (uint.Z ((head &&& W8 31) - W8 24)) = size)
    (hshort : rest.length < sint.Z size) : UnsignedReject (head :: rest) := by
  have hbound : 24 ≤ uint.Z head := by
    have h : ∀ b : w8, uint.Z (b &&& W8 31) ≤ uint.Z b := by decide
    have hb := h head
    omega
  refine ⟨hbound, ?_, ?_, ?_, ?_⟩
  · intro heq
    rw [heq] at hsize
    have hs : size = W64 1 := hsize.symm
    rw [hs] at hshort
    change (rest.length : Int) < 1 at hshort
    omega
  · intro heq
    rw [heq] at hsize
    have hs : size = W64 2 := hsize.symm
    rw [hs] at hshort
    change (rest.length : Int) < 2 at hshort
    omega
  · intro heq
    rw [heq] at hsize
    have hs : size = W64 4 := hsize.symm
    rw [hs] at hshort
    change (rest.length : Int) < 4 at hshort
    omega
  · intro heq
    rw [heq] at hsize
    have hs : size = W64 8 := hsize.symm
    rw [hs] at hshort
    change (rest.length : Int) < 8 at hshort
    omega

/-- An actual type error after arbitrary valid limbs preserves the old
    typed pointer and rolls back the array, retaining the inner diagnostic offset.
    All array-header widths, including the inline four-element head, are covered. -/
theorem wp_readFelt_limb_type (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs payload badpayload rem : List w8) (head bad : w8) (count argument : w64)
    (items : List UnsignedWire) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload count) (hm : head &&& W8 224 = W8 128)
    (hprefix : items.length < 4) (hcount : items.length < uint.Z count)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hdepth : sint.Z v.depth' < 32)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ unsignedBytes items ++ ((bad :: badpayload) ++ rem))
    (badenc : HeadArgument bad badpayload argument)
    (hbadmajor : bad &&& W8 224 ≠ W8 0) (hbadtag : bad &&& W8 224 ≠ W8 192) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
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
  wp_apply wp_readFelt_prefix_reject c dst old v xs payload ((bad :: badpayload) ++ rem) head count items dq
    iprop(globalAddr cursor.ErrType ↦□ kind)
    (fun err => iprop(∃ args : GoSlice, err ↦ cursor.DecodeError.mk kind (v.pos' + W64 (1 + payload.length + (unsignedBytes items).length)) (go!"expected %s, found %s") args ∗
       args ↦* [interface.mkOk go.string #(headDescription (W8 0)), interface.mkOk go.string #(headDescription bad)]))
    enc hm (unsignedReject_nonuint bad (badpayload ++ rem) hbadmajor) hprefix hcount hp hdepth hbytes
    (wp_uint64_type c {v with pos' := pos, depth' := v.depth' + W64 1} xs badpayload rem bad argument dq kind badenc hbadmajor hbadtag hpnext.1 hbound hi hafter)
    $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
  · iframe Hkind Hdst Hc Hs
  wp_end


/-- An actual tag error after arbitrary valid limbs preserves the old
    typed pointer and rolls back the array, retaining the inner diagnostic offset.
    All array-header widths, including the inline four-element head, are covered. -/
theorem wp_readFelt_limb_tag (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs payload badpayload rem : List w8) (head bad : w8) (count argument : w64)
    (items : List UnsignedWire) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload count) (hm : head &&& W8 224 = W8 128)
    (hprefix : items.length < 4) (hcount : items.length < uint.Z count)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hdepth : sint.Z v.depth' < 32)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ unsignedBytes items ++ ((bad :: badpayload) ++ rem))
    (badenc : HeadArgument bad badpayload argument)
    (hbadtag : bad &&& W8 224 = W8 192) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
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
  wp_apply wp_readFelt_prefix_reject c dst old v xs payload ((bad :: badpayload) ++ rem) head count items dq
    iprop(globalAddr cursor.ErrUnsupported ↦□ kind)
    (fun err => iprop(∃ args : GoSlice, err ↦ cursor.DecodeError.mk kind (v.pos' + W64 (1 + payload.length + (unsignedBytes items).length)) (go!"tag where %s is expected") args ∗
       args ↦* [interface.mkOk go.string #(headDescription (W8 0))]))
    enc hm (unsignedReject_nonuint bad (badpayload ++ rem) (by rw [hbadtag]; decide)) hprefix hcount hp hdepth hbytes
    (wp_uint64_tag c {v with pos' := pos, depth' := v.depth' + W64 1} xs badpayload rem bad argument dq kind badenc (by rw [hbadtag]; decide) hbadtag hpnext.1 hbound hi hafter)
    $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
  · iframe Hkind Hdst Hc Hs
  wp_end


/-- An actual truncated error after arbitrary valid limbs preserves the old
    typed pointer and rolls back the array, retaining the inner diagnostic offset.
    All array-header widths, including the inline four-element head, are covered. -/
theorem wp_readFelt_limb_truncated (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs payload rem : List w8) (head bad : w8) (count size : w64)
    (items : List UnsignedWire) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload count) (hm : head &&& W8 224 = W8 128)
    (hprefix : items.length < 4) (hcount : items.length < uint.Z count)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hdepth : sint.Z v.depth' < 32)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ unsignedBytes items ++ (bad :: rem))
    (hinfo : 24 ≤ uint.Z (bad &&& W8 31) ∧ uint.Z (bad &&& W8 31) ≤ 27)
    (hsize : W64 1 <<< W64 (uint.Z ((bad &&& W8 31) - W8 24)) = size)
    (htrunc : rem.length < sint.Z size) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
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
  wp_apply wp_readFelt_prefix_reject c dst old v xs payload (bad :: rem) head count items dq
    iprop(globalAddr cursor.ErrTruncated ↦□ kind)
    (fun err => iprop(err ↦ cursor.DecodeError.mk kind (v.pos' + W64 (1 + payload.length + (unsignedBytes items).length)) (go!"input ends inside the head") slice.nil))
    enc hm (unsignedReject_truncated bad rem size hinfo hsize htrunc) hprefix hcount hp hdepth hbytes
    (wp_uint64_truncated c {v with pos' := pos, depth' := v.depth' + W64 1} xs bad size dq kind hpnext.1 hlt hi hinfo hsize hshort)
    $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
  · iframe Hkind Hdst Hc Hs
  wp_end


/-- EOF after any valid prefix shorter than four limbs restores the array start
    but retains the end-of-input diagnostic, without publishing a partial felt. -/
theorem wp_readFelt_limb_eof (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs payload : List w8) (head : w8) (count : w64) (items : List UnsignedWire)
    (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload count) (hm : head &&& W8 224 = W8 128)
    (hprefix : items.length < 4) (hcount : items.length < uint.Z count)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hdepth : sint.Z v.depth' < 32)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ unsignedBytes items) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
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
  wp_apply wp_readFelt_prefix_reject c dst old v xs payload [] head count items dq
    iprop(globalAddr cursor.ErrTruncated ↦□ kind)
    (fun err => iprop(err ↦ cursor.DecodeError.mk kind v.buf'.len (go!"input ends before the item") slice.nil))
    enc hm (by trivial) hprefix hcount hp hdepth (by simpa only [List.append_nil] using hbytes) hleaf
    $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
  · iframe Hkind Hdst Hc Hs
  wp_end


end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.unsignedReject_nonuint
#print axioms Perennial.github_com.NethermindEth.juno.core.unsignedReject_truncated
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_limb_type
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_limb_tag
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_limb_truncated
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_limb_eof
