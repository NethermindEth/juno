import ProofCheck.FeltOuterReject

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


/-- The actual Skip worker preserves its cursor/input on an incomplete head,
    regardless of the head's major type and the caller's nesting depth. -/
theorem wp_skip_head_truncated (c : Loc) (v : cursor.Cursor) (depth : w64)
    (xs rem : List w8) (head : w8) (size : w64) (dq : DFrac) (kind : GoInterface)
    (hinfo : 24 ≤ uint.Z (head &&& W8 31) ∧ uint.Z (head &&& W8 31) ≤ 27)
    (hsize : W64 1 <<< W64 (uint.Z ((head &&& W8 31) - W8 24)) = size)
    (htrunc : rem.length < sint.Z size)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #depth))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends inside the head") slice.nil }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hi := input_suffix_head xs rem (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
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
  wp_apply wp_skip_head_reject c v depth xs rem head dq
    iprop(globalAddr cursor.ErrTruncated ↦□ kind)
    (fun err => iprop(err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends inside the head") slice.nil))
    hne hp hbytes
    (wp_head_reject c v xs head dq iprop(globalAddr cursor.ErrTruncated ↦□ kind)
      (fun err => iprop(err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends inside the head") slice.nil))
      hp.1 hlt hi hinfo.1 (wp_headSlow_truncated c v xs head size dq kind hp.1 hlt hi hinfo hsize hshort))
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Public Skip retains the incomplete head's initial offset and full cursor state. -/
theorem wp_Skip_head_truncated (c : Loc) (v : cursor.Cursor)
    (xs rem : List w8) (head : w8) (size : w64) (dq : DFrac) (kind : GoInterface)
    (hinfo : 24 ≤ uint.Z (head &&& W8 31) ∧ uint.Z (head &&& W8 31) ≤ 27)
    (hsize : W64 1 <<< W64 (uint.Z ((head &&& W8 31) - W8 24)) = size)
    (htrunc : rem.length < sint.Z size)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends inside the head") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_skip_head_truncated c v v.depth' xs rem head size dq kind hinfo hsize htrunc hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- An incomplete ignored head after arbitrary valid skipped contents restores
    the whole felt while retaining the beginning of the bad inner head as offset. -/
theorem wp_readFelt_extras_head_truncated (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs payload rem : List w8) (head bad : w8) (count size : w64)
    (items : List UnsignedWire) (extras : List (List w8)) (dq : DFrac) (kind : GoInterface)
    (d : Nat) (hd : sint.Z v.depth' = d) (hdepth : d < 32)
    (enc : HeadArgument head payload count) (hm : head &&& W8 224 = W8 128)
    (hcount : items.length = 4) (hmore : 4 + extras.length < uint.Z count)
    (valid : ∀ item ∈ extras, SkipValid (d + 1) item)
    (hinfo : 24 ≤ uint.Z (bad &&& W8 31) ∧ uint.Z (bad &&& W8 31) ≤ 27)
    (hsize : W64 1 <<< W64 (uint.Z ((bad &&& W8 31) - W8 24)) = size)
    (htrunc : rem.length < sint.Z size)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ unsignedBytes items ++ extras.flatten ++ (bad :: rem)) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind
         (v.pos' + W64 (1 + payload.length + (unsignedBytes items).length + extras.flatten.length))
         (go!"input ends inside the head") slice.nil }} := by
  wp_start_folded as ⟨#Hkind, Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  let pos := v.pos' + W64 (1 + payload.length + (unsignedBytes items).length + extras.flatten.length)
  have Hnext := input_consumption xs ((head :: payload) ++ unsignedBytes items ++ extras.flatten)
    (bad :: rem) v.pos' v.buf'.len hp Hlen.1 hbytes
  have hpnext : 0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    dsimp only [pos]; constructor <;> word
  have hsuffix : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = bad :: rem := by
    simpa only [pos, List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
      Int.add_comm, Int.add_left_comm, Int.add_assoc] using Hnext.2.2
  wp_apply wp_readFelt_extras_reject c dst old v xs payload (bad :: rem) head count items extras dq
    iprop(globalAddr cursor.ErrTruncated ↦□ kind)
    (fun err => iprop(err ↦ cursor.DecodeError.mk kind pos (go!"input ends inside the head") slice.nil))
    d hd hdepth enc hm hcount hmore valid hp hbytes
    (wp_Skip_head_truncated c {v with pos' := pos, depth' := v.depth' + W64 1} xs rem bad size dq kind hinfo hsize htrunc hpnext hsuffix)
    $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
  · iframe Hkind Hdst Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_skip_head_truncated
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Skip_head_truncated
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_extras_head_truncated
