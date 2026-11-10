import ProofCheck.BigIntValid
import ProofCheck.HeadKey
import ProofCheck.LateError

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

theorem wp_bigInt_eof (c : Loc) (v : cursor.Cursor) (kind : GoInterface)
    (heof : v.pos' = v.buf'.len) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"BigInt")) (Val #()))
    {{ (err : Loc), RET (PairV #null #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends before the item") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc⟩
  wp_auto
  wp_apply wp_head_eof c v kind heof $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hkind Hc
  wp_end

theorem wp_bigInt_head_fault (c : Loc) (v : cursor.Cursor) (xs : List w8)
    (head : w8) (dq : DFrac) (reason : HeadError) (kind : GoInterface)
    (fault : HeadFault head reason) (hp : 0 ≤ sint.Z v.pos') (hlt : sint.Z v.pos' < sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       headFaultAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"BigInt")) (Val #()))
    {{ (err : Loc), RET (PairV #null #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ HeadFaultError err v.pos' head reason kind }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  have hinfo := headFault_info head reason fault
  wp_apply wp_head_reject c v xs head dq iprop(headFaultAddr (GF := GF) reason ↦□ kind)
    (fun err => HeadFaultError err v.pos' head reason kind) hp hlt hi (by omega)
    (wp_headSlow_fault c v xs head dq reason kind fault hp hlt hi)
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

theorem wp_bigInt_head_truncated (c : Loc) (v : cursor.Cursor) (xs : List w8)
    (head : w8) (size : w64) (dq : DFrac) (kind : GoInterface)
    (hp : 0 ≤ sint.Z v.pos') (hlt : sint.Z v.pos' < sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hinfo : 24 ≤ uint.Z (head &&& W8 31) ∧ uint.Z (head &&& W8 31) ≤ 27)
    (hsize : W64 1 <<< W64 (uint.Z ((head &&& W8 31) - W8 24)) = size)
    (htrunc : sint.Z v.buf'.len - (sint.Z v.pos' + 1) < sint.Z size) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"BigInt")) (Val #()))
    {{ (err : Loc), RET (PairV #null #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends inside the head") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_head_reject c v xs head dq iprop(globalAddr cursor.ErrTruncated ↦□ kind)
    (fun err => iprop(err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends inside the head") slice.nil))
    hp hlt hi hinfo.1
    (wp_headSlow_truncated c v xs head size dq kind hp hlt hi hinfo hsize htrunc)
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- A complete non-integer, non-tag head rolls back and reports the original head. -/
theorem wp_bigInt_type (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument)
    (hu : head &&& W8 224 ≠ W8 0) (hn : head &&& W8 224 ≠ W8 32) (ht : head &&& W8 224 ≠ W8 192)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrType ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"BigInt")) (Val #()))
    {{ (err : Loc), RET (PairV #null #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"expected %s, found %s") args ∗
       args ↦* [interface.mkOk go.string #(go!"an integer"), interface.mkOk go.string #(headDescription head)] }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hfull := input_consumption xs (head :: payload) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hi := input_suffix_head xs (payload ++ rem) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by simp only [List.length_cons] at Hfull; word
  wp_auto
  wp_apply wp_head_prefix c v xs payload rem head argument dq enc hp hbytes
    $$ [Hc Hs] with ⟨Hc, Hs, %Hnext⟩
  · iframe Hc Hs
  simp only [hu, _root_.decide_false]
  wp_auto
  simp only [hn, _root_.decide_false]
  wp_auto
  simp only [ht, _root_.decide_false]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp.1, hlt⟩)]
  wp_apply wp_load_slice_index v.buf' (sint.Z v.pos') xs dq head hp.1 $$ [Hs] with Hs
  · iframe Hs; ipureintro; exact hi
  wp_apply wp_mismatch_type c v (go!"an integer") head kind ht $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hkind Hc
  wp_end

/-- Tags other than 2 and 3 roll back and retain the full uint64 tag argument. -/
theorem wp_bigInt_tag_rejected (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 192)
    (h2 : argument ≠ W64 2) (h3 : argument ≠ W64 3)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"BigInt")) (Val #()))
    {{ (err : Loc), RET (PairV #null #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"tag %d where an integer is expected") args ∗
       args ↦* [interface.mkOk go.uint64 #argument] }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_head_prefix c v xs payload rem head argument dq enc hp hbytes
    $$ [Hc Hs] with ⟨Hc, Hs, %Hnext⟩
  · iframe Hc Hs
  simp only [major]
  wp_auto
  simp only [h2, _root_.decide_false]
  wp_auto
  simp only [h3, _root_.decide_false]
  wp_auto
  wp_apply wp_slice_literal (V := GoInterface) [interface.mkOk go.uint64 #argument]
  isplitr
  · ipureintro; rfl
  iintro %args ⟨Hargs, -⟩
  wp_auto
  wp_apply wp_fail c v kind (go!"tag %d where an integer is expected") _ $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc
  wp_end

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bigInt_eof
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bigInt_head_fault
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bigInt_head_truncated
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bigInt_type
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bigInt_tag_rejected
