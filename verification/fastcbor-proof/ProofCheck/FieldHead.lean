import ProofCheck.HeadKey

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

/-- Extended and invalid heads cannot be an inline cached schema name. -/
theorem extended_head_outside (head : w8) (hinfo : 24 ≤ uint.Z (head &&& W8 31)) :
    ¬ (96 ≤ uint.Z head ∧ uint.Z head < 120) := by
  have h : ∀ head : w8, 24 ≤ uint.Z (head &&& W8 31) →
      ¬ (96 ≤ uint.Z head ∧ uint.Z head < 120) := by decide
  exact h head hinfo

/-- Public field reading preserves the complete head diagnostic and input,
    decrements the remaining pair count once, and preserves next/seen/open. -/
theorem wp_nextField_head_indefinite (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (order : List cursor.StaticField) (names : List GoString) (xs rest : List w8)
    (head : w8) (qf qo dq : DFrac) (kind : GoInterface)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ order.length)
    (hnonempty : ∀ entry ∈ order, entry.Encoded' ≠ []) (hsorted : FieldHeadsOrdered order)
    (hcache : CachedShortTable order) (htable : InlineFieldTable order names)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rest)
    (hinfo : 27 < uint.Z (head &&& W8 31)) (hindef : head &&& W8 31 = W8 31)
    (hmajor : 64 ≤ uint.Z (head &&& W8 224) ∧ uint.Z (head &&& W8 224) ≤ 160) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ st ↦ {s with left' := s.left' - W64 1} ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs ∗ err ↦ cursor.DecodeError.mk kind v.pos' (go!"indefinite-length item") slice.nil }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hst, Hfields, Horder, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs [head] rest v.pos' v.buf'.len hp Hlen.1 hbytes
  have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by
    simp only [List.length_cons, List.length_nil] at Hnext
    omega
  have hi := input_suffix_head xs rest (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hmiss := inlineFieldTable_head_miss order names head rest htable
    (extended_head_outside head (by omega))
  wp_apply wp_nextField_reject c st fields v v s f order xs (head :: rest) qf qo dq
    iprop(globalAddr cursor.ErrUnsupported ↦□ kind)
    (fun err => iprop(err ↦ cursor.DecodeError.mk kind v.pos' (go!"indefinite-length item") slice.nil))
    hfields hleft hstart hnonempty hsorted hcache (fun j entry _ he => hmiss j entry he) hp hbytes
    (wp_otherField_Key_reject c st v xs dq iprop(globalAddr cursor.ErrUnsupported ↦□ kind)
      (fun err => iprop(err ↦ cursor.DecodeError.mk kind v.pos' (go!"indefinite-length item") slice.nil))
      (wp_Key_head_indefinite c v xs rest head dq kind hp.1 hlt hi hbytes hinfo hindef hmajor))
    $$ [Hc Hst Hfields Horder Hs] as %err ⟨Hc, Hst, Hfields, Horder, Hs, Herr⟩
  · iframe Hkind Hc Hst Hfields Horder Hs
  wp_end

/-- Public field reading preserves the complete head diagnostic and input,
    decrements the remaining pair count once, and preserves next/seen/open. -/
theorem wp_nextField_head_break (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (order : List cursor.StaticField) (names : List GoString) (xs rest : List w8)
    (head : w8) (qf qo dq : DFrac) (kind : GoInterface)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ order.length)
    (hnonempty : ∀ entry ∈ order, entry.Encoded' ≠ []) (hsorted : FieldHeadsOrdered order)
    (hcache : CachedShortTable order) (htable : InlineFieldTable order names)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rest)
    (hinfo : 27 < uint.Z (head &&& W8 31)) (hindef : head &&& W8 31 = W8 31)
    (hmajor : head &&& W8 224 = W8 224) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrSyntax ↦□ kind ∗ c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ st ↦ {s with left' := s.left' - W64 1} ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs ∗ err ↦ cursor.DecodeError.mk kind v.pos' (go!"break code outside an indefinite-length item") slice.nil }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hst, Hfields, Horder, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs [head] rest v.pos' v.buf'.len hp Hlen.1 hbytes
  have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by
    simp only [List.length_cons, List.length_nil] at Hnext
    omega
  have hi := input_suffix_head xs rest (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hmiss := inlineFieldTable_head_miss order names head rest htable
    (extended_head_outside head (by omega))
  wp_apply wp_nextField_reject c st fields v v s f order xs (head :: rest) qf qo dq
    iprop(globalAddr cursor.ErrSyntax ↦□ kind)
    (fun err => iprop(err ↦ cursor.DecodeError.mk kind v.pos' (go!"break code outside an indefinite-length item") slice.nil))
    hfields hleft hstart hnonempty hsorted hcache (fun j entry _ he => hmiss j entry he) hp hbytes
    (wp_otherField_Key_reject c st v xs dq iprop(globalAddr cursor.ErrSyntax ↦□ kind)
      (fun err => iprop(err ↦ cursor.DecodeError.mk kind v.pos' (go!"break code outside an indefinite-length item") slice.nil))
      (wp_Key_head_break c v xs rest head dq kind hp.1 hlt hi hbytes hinfo hindef hmajor))
    $$ [Hc Hst Hfields Horder Hs] as %err ⟨Hc, Hst, Hfields, Horder, Hs, Herr⟩
  · iframe Hkind Hc Hst Hfields Horder Hs
  wp_end

/-- Public field reading preserves the complete head diagnostic and input,
    decrements the remaining pair count once, and preserves next/seen/open. -/
theorem wp_nextField_head_invalid (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (order : List cursor.StaticField) (names : List GoString) (xs rest : List w8)
    (head : w8) (qf qo dq : DFrac) (kind : GoInterface)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ order.length)
    (hnonempty : ∀ entry ∈ order, entry.Encoded' ≠ []) (hsorted : FieldHeadsOrdered order)
    (hcache : CachedShortTable order) (htable : InlineFieldTable order names)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rest)
    (hinfo : 27 < uint.Z (head &&& W8 31))
    (hindef : ¬ (head &&& W8 31 = W8 31 ∧
      64 ≤ uint.Z (head &&& W8 224) ∧ uint.Z (head &&& W8 224) ≤ 160))
    (hbreak : ¬ (head &&& W8 31 = W8 31 ∧ head &&& W8 224 = W8 224)) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrSyntax ↦□ kind ∗ c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ st ↦ {s with left' := s.left' - W64 1} ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice, err ↦ cursor.DecodeError.mk kind v.pos' (go!"invalid additional information %d for %s") args ∗
       args ↦* [interface.mkOk go.byte #(head &&& W8 31),
         interface.mkOk go.string #(headDescription head)] }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hst, Hfields, Horder, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs [head] rest v.pos' v.buf'.len hp Hlen.1 hbytes
  have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by
    simp only [List.length_cons, List.length_nil] at Hnext
    omega
  have hi := input_suffix_head xs rest (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hmiss := inlineFieldTable_head_miss order names head rest htable
    (extended_head_outside head (by omega))
  wp_apply wp_nextField_reject c st fields v v s f order xs (head :: rest) qf qo dq
    iprop(globalAddr cursor.ErrSyntax ↦□ kind)
    (fun err => iprop(∃ args : GoSlice, err ↦ cursor.DecodeError.mk kind v.pos' (go!"invalid additional information %d for %s") args ∗
       args ↦* [interface.mkOk go.byte #(head &&& W8 31),
         interface.mkOk go.string #(headDescription head)]))
    hfields hleft hstart hnonempty hsorted hcache (fun j entry _ he => hmiss j entry he) hp hbytes
    (wp_otherField_Key_reject c st v xs dq iprop(globalAddr cursor.ErrSyntax ↦□ kind)
      (fun err => iprop(∃ args : GoSlice, err ↦ cursor.DecodeError.mk kind v.pos' (go!"invalid additional information %d for %s") args ∗
       args ↦* [interface.mkOk go.byte #(head &&& W8 31),
         interface.mkOk go.string #(headDescription head)]))
      (wp_Key_head_invalid c v xs rest head dq kind hp.1 hlt hi hbytes hinfo hindef hbreak))
    $$ [Hc Hst Hfields Horder Hs] as %err ⟨Hc, Hst, Hfields, Horder, Hs, Herr⟩
  · iframe Hkind Hc Hst Hfields Horder Hs
  wp_end

/-- Public field reading preserves the complete head diagnostic and input,
    decrements the remaining pair count once, and preserves next/seen/open. -/
theorem wp_nextField_head_truncated (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (order : List cursor.StaticField) (names : List GoString) (xs rest : List w8)
    (head : w8) (size : w64) (qf qo dq : DFrac) (kind : GoInterface)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ order.length)
    (hnonempty : ∀ entry ∈ order, entry.Encoded' ≠ []) (hsorted : FieldHeadsOrdered order)
    (hcache : CachedShortTable order) (htable : InlineFieldTable order names)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rest)
    (hinfo : 24 ≤ uint.Z (head &&& W8 31) ∧ uint.Z (head &&& W8 31) ≤ 27)
    (hsize : W64 1 <<< W64 (uint.Z ((head &&& W8 31) - W8 24)) = size)
    (htrunc : sint.Z v.buf'.len - (sint.Z v.pos' + 1) < sint.Z size) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ st ↦ {s with left' := s.left' - W64 1} ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs ∗ err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends inside the head") slice.nil }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hst, Hfields, Horder, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs [head] rest v.pos' v.buf'.len hp Hlen.1 hbytes
  have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by
    simp only [List.length_cons, List.length_nil] at Hnext
    omega
  have hi := input_suffix_head xs rest (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hmiss := inlineFieldTable_head_miss order names head rest htable
    (extended_head_outside head hinfo.1)
  wp_apply wp_nextField_reject c st fields v v s f order xs (head :: rest) qf qo dq
    iprop(globalAddr cursor.ErrTruncated ↦□ kind)
    (fun err => iprop(err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends inside the head") slice.nil))
    hfields hleft hstart hnonempty hsorted hcache (fun j entry _ he => hmiss j entry he) hp hbytes
    (wp_otherField_Key_reject c st v xs dq iprop(globalAddr cursor.ErrTruncated ↦□ kind)
      (fun err => iprop(err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends inside the head") slice.nil))
      (wp_Key_head_truncated c v xs rest head size dq kind hp.1 hlt hi hbytes hinfo hsize htrunc))
    $$ [Hc Hst Hfields Horder Hs] as %err ⟨Hc, Hst, Hfields, Horder, Hs, Herr⟩
  · iframe Hkind Hc Hst Hfields Horder Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.extended_head_outside
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_head_indefinite
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_head_break
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_head_invalid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_head_truncated
