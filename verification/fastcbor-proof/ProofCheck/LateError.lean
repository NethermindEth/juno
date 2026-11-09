import ProofCheck.Number
import Perennial.Golang.Theory.String

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


/-- At end of input, Head reports truncation at the current offset without changing the cursor. -/
theorem wp_head_eof (c : Loc) (v : cursor.Cursor) (kind : GoInterface)
    (heof : v.pos' = v.buf'.len) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"head")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #(W8 0) #(W64 0))
        #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends before the item") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc⟩
  wp_auto
  simp only [heof, Int.lt_irrefl, _root_.decide_false]
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

/-- Key propagates the EOF error, including its offset, after a previous value was consumed. -/
theorem wp_key_eof (c : Loc) (v : cursor.Cursor) (kind : GoInterface)
    (heof : v.pos' = v.buf'.len)
    (hwf : 0 ≤ sint.Z v.buf'.len ∧ sint.Z v.buf'.len ≤ sint.Z v.buf'.cap) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Key")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #slice.nil #false)
        #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends before the item") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc⟩
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true (by rw [heof]; exact ⟨hwf.1, Int.le_refl _, hwf.2⟩))]
  wp_auto
  simp only [slice.slice, heof, BitVec.sub_self]
  wp_auto
  wp_method_call
  wp_call
  wp_auto
  wp_apply wp_head_eof c v kind heof $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc Hkind
  isimp only [heof] at Herr
  wp_end

/-- The alternate-key path preserves the Key error instead of converting it to an unknown field. -/
theorem wp_otherField_eof (c st : Loc) (v : cursor.Cursor) (kind : GoInterface)
    (heof : v.pos' = v.buf'.len)
    (hwf : 0 ≤ sint.Z v.buf'.len ∧ sint.Z v.buf'.len ≤ sint.Z v.buf'.cap) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"otherField")) (Val #st))
    {{ (err : Loc), RET (PairV #(W64 (-1))
        #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends before the item") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc⟩
  wp_auto
  wp_apply wp_key_eof c v kind heof hwf $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc Hkind
  wp_end

/-- Field lookup on an empty suffix does not touch the field array or input storage. -/
theorem wp_find_eof (st fields : Loc) (v : cursor.Struct) (order names buf : GoSlice)
    (hfields : v.fields' = fields) (hnext : v.next' = W64 2)
    (horder : order.len = W64 15) (hbuf : buf.len = W64 0) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       st ↦ v ∗ fields ↦ cursor.Fields.mk order names (W64 32767) }}
      (App (Val (st @!! go.PointerType cursor.Struct.ty @!! go!"find")) (Val #buf))
    {{ RET #(W64 (-1)); st ↦ v ∗ fields ↦ cursor.Fields.mk order names (W64 32767) }} := by
  wp_start as ⟨Hst, Hfields⟩
  wp_auto
  simp only [hfields]
  wp_auto
  simp only [hbuf]
  wp_auto
  isimp only [hnext] at i
  wp_for
  simp only [horder]
  wp_auto
  simp only [hbuf]
  have hf : (#false : val) ≠ #true := fun h => Bool.noConfusion (go.intoVal_inj h)
  simp only [Int.lt_irrefl, _root_.decide_false, hf, Bool.false_eq_true, ite_false, _root_.decide_true, ite_true]
  wp_auto
  wp_end
/-- The malformed record declares two pairs; its available bytes pass the map-length guard. -/
theorem wp_openHeaderNumber_two (c fields : Loc) (data : GoSlice) (b : w8) (dq : DFrac)
    (hdata : data.len = W64 9) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ cursor.Cursor.mk data (W64 0) (W64 0) (zero_val (GoArray w64 4)) ∗
       data ↦*{dq} (W8 162 :: (go!"fNumber" ++ [b])) }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"OpenStruct")) (Val #fields))
    {{ RET (PairV #(cursor.Struct.mk fields (W64 2) (W64 0) (W64 0) true) #interface.nil);
       c ↦ cursor.Cursor.mk data (W64 1) (W64 1) (zero_val (GoArray w64 4)) ∗
       data ↦*{dq} (W8 162 :: (go!"fNumber" ++ [b])) }} := by
  wp_start as ⟨Hc, Hdata⟩
  wp_auto
  wp_apply wp_null_other c (cursor.Cursor.mk data (W64 0) (W64 0) (zero_val (GoArray w64 4))) (W8 162 :: (go!"fNumber" ++ [b])) (W8 162) dq
    (by change 0 ≤ sint.Z (W64 0); decide) rfl (by decide) (by decide) $$ [Hc Hdata] with ⟨Hc, Hdata⟩
  · iframe Hc Hdata
  wp_method_call
  wp_call
  wp_auto
  simp only [hdata]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true (by rw [hdata]; decide))]
  wp_apply wp_load_slice_index (t := go.byte) data (sint.Z (W64 0))
    (W8 162 :: (go!"fNumber" ++ [b])) dq (W8 162) (by decide) $$ [Hdata] with Hdata
  · iframe Hdata; ipureintro; rfl
  simp only [hdata]
  wp_auto
  simp only [show W64 (uint.Z (W8 162 - W8 160)) = W64 2 from rfl, show (0 : w64) = W64 0 from rfl]
  wp_end


/-- Looking for the missing second pair returns truncation after the Number value. -/
theorem wp_nextHeader_eof (c st fields : Loc) (order names data : GoSlice)
    (kind : GoInterface) (horder : order.len = W64 15) (hdata : data.len = W64 9)
    (hcap : sint.Z data.len ≤ sint.Z data.cap) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗
       st ↦ cursor.Struct.mk fields (W64 1) (W64 2) (W64 4) true ∗
       c ↦ cursor.Cursor.mk data (W64 9) (W64 1) (zero_val (GoArray w64 4)) ∗
       fields ↦ cursor.Fields.mk order names (W64 32767) }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       st ↦ cursor.Struct.mk fields (W64 0) (W64 2) (W64 4) true ∗
       c ↦ cursor.Cursor.mk data (W64 9) (W64 1) (zero_val (GoArray w64 4)) ∗
       fields ↦ cursor.Fields.mk order names (W64 32767) ∗
       err ↦ cursor.DecodeError.mk kind (W64 9) (go!"input ends before the item") slice.nil }} := by
  wp_start as ⟨#Hkind, Hst, Hc, Hfields⟩
  wp_auto
  simp only [horder]
  wp_auto
  simp only [hdata]
  wp_auto
  wp_method_call
  wp_call
  wp_auto
  have hb : 0 ≤ sint.Z (W64 9) ∧ sint.Z (W64 9) ≤ sint.Z data.len ∧
      sint.Z data.len ≤ sint.Z data.cap := ⟨by decide, by rw [hdata]; exact Int.le_refl _, hcap⟩
  rw [ite_eq_left_of_eq_true _ _ (eq_true hb)]
  wp_auto
  ieval (change (st ↦ cursor.Struct.mk fields (W64 0) (W64 2) (W64 4) true : IProp GF) ⊢ _) at Hst
  wp_apply wp_find_eof st fields (cursor.Struct.mk fields (W64 0) (W64 2) (W64 4) true)
    order names (slice.slice data w8 (W64 9) data.len) rfl rfl horder
    (by simp only [slice.slice, hdata]; rfl) $$ [Hst Hfields] with ⟨Hst, Hfields⟩
  · iframe Hst Hfields
  wp_apply wp_otherField_eof c st (cursor.Cursor.mk data (W64 9) (W64 1) (zero_val (GoArray w64 4)))
    kind (by exact hdata.symm) (by exact ⟨by rw [hdata]; decide, hcap⟩)
    $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc Hkind
  wp_end
/-- The reader writes Number before returning the error for the missing second pair. -/
theorem wp_readHeaderNumber_lateError (c dst fields : Loc) (order names data : GoSlice)
    (kind : GoInterface) (h : Header) (b : w8) (dq : DFrac) (hb : uint.Z b < 24)
    (horder : order.len = W64 15) (hdata : data.len = W64 9) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr cborHeaderFields ↦□ fields ∗ globalAddr cursor.ErrTruncated ↦□ kind ∗ dst ↦ h ∗
       c ↦ cursor.Cursor.mk data (W64 0) (W64 0) (zero_val (GoArray w64 4)) ∗
       fields ↦ cursor.Fields.mk order names (W64 32767) ∗
       order ↦* headerFieldKeys ∗ data ↦*{dq} (W8 162 :: (go!"fNumber" ++ [b])) }}
      (App (App (Val (@! cborReadHeader)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err); dst ↦ {h with Number' := W64 (uint.Z b)} ∗
       c ↦ cursor.Cursor.mk data (W64 9) (W64 1) (zero_val (GoArray w64 4)) ∗
       fields ↦ cursor.Fields.mk order names (W64 32767) ∗
       order ↦* headerFieldKeys ∗ data ↦*{dq} (W8 162 :: (go!"fNumber" ++ [b])) ∗
       err ↦ cursor.DecodeError.mk kind (W64 9) (go!"input ends before the item") slice.nil }} := by
  wp_start as ⟨#Hglobal, #Hkind, Hdst, Hc, Hfields, Horder, Hdata⟩
  wp_auto
  wp_apply wp_openHeaderNumber_two c fields data b dq hdata $$ [Hc Hdata] with ⟨Hc, Hdata⟩
  · iframe Hc Hdata
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hdata
  ihave HI : (∃ (done : Bool),
      "Hs" ∷ s_ptr ↦ (if done then cursor.Struct.mk fields (W64 1) (W64 2) (W64 4) true
                                else cursor.Struct.mk fields (W64 2) (W64 0) (W64 0) true) ∗
      "Hc" ∷ c ↦ cursor.Cursor.mk data (if done then W64 9 else W64 1) (W64 1) (zero_val (GoArray w64 4)) ∗
      "Hdst" ∷ dst ↦ (if done then {h with Number' := W64 (uint.Z b)} else h) : IProp GF) $$ [s Hc Hdst]
  · iexists false; isimp only [Bool.false_eq_true, ite_false]; iframe
  wp_for HI
  cases done with
  | false =>
    isimp only [Bool.false_eq_true, ite_false] at Hs Hc Hdst
    wp_apply wp_nextHeaderNumber_count c s_ptr fields order names data b (W8 162) (W64 2) dq (W64 1) (by decide) horder hdata
      $$ [Hs Hc Hfields Horder Hdata] with ⟨Hs, Hc, Hfields, Horder, Hdata⟩
    · iframe Hs Hc Hfields Horder Hdata
    wp_apply wp_headerNumber_inline_at c dst h
      (cursor.Cursor.mk data (W64 8) (W64 1) (zero_val (GoArray w64 4)))
      (W8 162 :: (go!"fNumber" ++ [b])) b dq (by change 0 ≤ sint.Z (W64 8); decide) rfl hb
      $$ [Hdst Hc Hdata] with ⟨Hdst, Hc, Hdata⟩
    · iframe Hdst Hc Hdata
    ieval (change (s_ptr ↦ cursor.Struct.mk fields (W64 1) (W64 2) (W64 4) true : IProp GF) ⊢ _) at Hs
    wp_for_post
    iframe
    iexists true
    isimp only [ite_true]
    ieval (change (c ↦ cursor.Cursor.mk data (W64 9) (W64 1) (zero_val (GoArray w64 4)) : IProp GF) ⊢ _) at Hc
    iframe Hs Hc Hdst
  | true =>
    isimp only [ite_true] at Hs Hc Hdst
    wp_apply wp_nextHeader_eof c s_ptr fields order names data kind horder hdata Hwf.2
      $$ [Hs Hc Hfields] as %err ⟨Hs, Hc, Hfields, Herr⟩
    · iframe Hs Hc Hfields Hkind
    wp_for_post
    wp_end


/-- The public First entry erases the written Number and returns nil suffix plus the original error. -/
theorem wp_decodeHeaderFirst_lateError (dst fields : Loc) (order names data : GoSlice)
    (kind : GoInterface) (b : w8) (dq : DFrac) (hb : uint.Z b < 24)
    (horder : order.len = W64 15) (hdata : data.len = W64 9) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr cborHeaderFields ↦□ fields ∗ globalAddr cursor.ErrTruncated ↦□ kind ∗
       dst ↦ (zero_val Header) ∗ fields ↦ cursor.Fields.mk order names (W64 32767) ∗
       order ↦* headerFieldKeys ∗ data ↦*{dq} (W8 162 :: (go!"fNumber" ++ [b])) }}
      (App (App (Val (@! DecodeHeaderFirst)) (Val #data)) (Val #dst))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       dst ↦ (zero_val Header) ∗ fields ↦ cursor.Fields.mk order names (W64 32767) ∗
       order ↦* headerFieldKeys ∗ data ↦*{dq} (W8 162 :: (go!"fNumber" ++ [b])) ∗
       err ↦ cursor.DecodeError.mk kind (W64 9) (go!"input ends before the item") slice.nil }} := by
  wp_start as ⟨#Hglobal, #Hkind, Hdst, Hfields, Horder, Hdata⟩
  wp_auto
  wp_apply wp_zeroHeader dst $$ [Hdst] with Hdst
  · iframe Hdst
  wp_apply wp_newCursor data
  ieval (change (c_ptr ↦ cursor.Cursor.mk data (W64 0) (W64 0) (zero_val (GoArray w64 4)) : IProp GF) ⊢ _) at c
  wp_apply wp_readHeaderNumber_lateError _ dst fields order names data kind (zero_val Header) b dq hb horder hdata
    $$ [c Hdst Hfields Horder Hdata] as %err ⟨Hdst, Hc, Hfields, Horder, Hdata, Herr⟩
  · iframe Hglobal Hkind c Hdst Hfields Horder Hdata
  wp_apply wp_clearHeader dst {(zero_val Header) with Number' := W64 (uint.Z b)} $$ [Hdst] with Hdst
  · iframe Hdst
  wp_end

/-- Full-input decoding preserves the late error and the cleared destination. -/
theorem wp_decodeHeader_lateError (dst fields : Loc) (order names data : GoSlice)
    (kind : GoInterface) (b : w8) (dq : DFrac) (hb : uint.Z b < 24)
    (horder : order.len = W64 15) (hdata : data.len = W64 9) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr cborHeaderFields ↦□ fields ∗ globalAddr cursor.ErrTruncated ↦□ kind ∗
       dst ↦ (zero_val Header) ∗ fields ↦ cursor.Fields.mk order names (W64 32767) ∗
       order ↦* headerFieldKeys ∗ data ↦*{dq} (W8 162 :: (go!"fNumber" ++ [b])) }}
      (App (App (Val (@! DecodeHeader)) (Val #data)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ (zero_val Header) ∗ fields ↦ cursor.Fields.mk order names (W64 32767) ∗
       order ↦* headerFieldKeys ∗ data ↦*{dq} (W8 162 :: (go!"fNumber" ++ [b])) ∗
       err ↦ cursor.DecodeError.mk kind (W64 9) (go!"input ends before the item") slice.nil }} := by
  wp_start as ⟨#Hglobal, #Hkind, Hdst, Hfields, Horder, Hdata⟩
  wp_auto
  wp_apply wp_decodeHeaderFirst_lateError dst fields order names data kind b dq hb horder hdata
    $$ [Hdst Hfields Horder Hdata] as %err ⟨Hdst, Hfields, Horder, Hdata, Herr⟩
  · iframe Hglobal Hkind Hdst Hfields Horder Hdata
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_head_eof
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_key_eof
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_otherField_eof
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_find_eof

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_openHeaderNumber_two
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextHeader_eof

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readHeaderNumber_lateError

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeaderFirst_lateError
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeader_lateError
