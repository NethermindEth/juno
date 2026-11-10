import ProofCheck.FieldInteger

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

/-- A truncated text body cannot contain a complete cached schema key. This
    covers every argument width: a possible prefix would force an inline head
    and enough body bytes, contradicting the truncation premise. -/
theorem inlineFieldTable_text_truncated_miss (order : List cursor.StaticField) (names : List GoString)
    (head : w8) (payload rem : List w8) (argument : w64)
    (htable : InlineFieldTable order names) (enc : HeadArgument head payload argument)
    (hshort : rem.length < uint.Z argument) :
    ∀ (j : Nat) (entry : cursor.StaticField), order[j]? = some entry →
      ((head :: payload) ++ rem).take entry.Encoded'.length ≠ entry.Encoded' := by
  intro j entry hlookup hprefix
  obtain ⟨name, _, _, _, hlen, hencoded⟩ := inlineFieldTable_entry order names entry htable (List.mem_of_getElem? hlookup)
  have hbound := prefix_length_le ((head :: payload) ++ rem) entry.Encoded' hprefix
  rw [hencoded] at hprefix hbound
  have hhead := prefix_head_eq ((head :: payload) ++ rem) (W8 (96 + name.length) :: name) (by simp) hprefix
  have hh : head = W8 (96 + name.length) := by simpa using hhead
  have hbits : uint.Z (W8 (96 + name.length) &&& W8 31) = name.length := by
    have h : ∀ n : Fin 24, uint.Z (W8 (96 + n.val) &&& W8 31) = n.val := by decide
    exact h ⟨name.length, hlen⟩
  have hsmall : uint.Z (head &&& W8 31) < 24 := by rw [hh, hbits]; omega
  obtain ⟨hpay, harg⟩ := headArgument_inline_inv head payload argument enc hsmall
  have hvalue : uint.Z argument = name.length := by rw [harg, hh, hbits]; word
  rw [hvalue] at hshort
  simp only [List.length_append, List.length_cons, hpay, List.length_nil] at hbound
  omega

/-- The public field wrapper propagates the actual key rejection with exact
    cursor rollback, one-pair progress and owned diagnostic arguments. -/
theorem wp_nextField_key_tag (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (order : List cursor.StaticField) (names : List GoString) (xs payload rem : List w8)
    (head : w8) (argument : w64) (qf qo dq : DFrac) (kind : GoInterface)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ order.length)
    (hnonempty : ∀ entry ∈ order, entry.Encoded' ≠ []) (hsorted : FieldHeadsOrdered order)
    (hcache : CachedShortTable order) (htable : InlineFieldTable order names)
    (enc : HeadArgument head payload argument)
    (htext : head &&& W8 224 ≠ W8 96)
    (huint : head &&& W8 224 ≠ W8 0) (hneg : head &&& W8 224 ≠ W8 32)
    (htag : head &&& W8 224 = W8 192)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ st ↦ {s with left' := s.left' - W64 1} ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"tag where %s is expected") args ∗
       args ↦* [interface.mkOk go.string #(go!"a text or integer key")] }} := by
  have hmiss := inlineFieldTable_head_miss order names head (payload ++ rem) htable (nontext_head_outside head htext)
  exact wp_nextField_reject c st fields v v s f order xs ((head :: payload) ++ rem) qf qo dq
    iprop(globalAddr cursor.ErrUnsupported ↦□ kind)
    (fun err => iprop(∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"tag where %s is expected") args ∗
       args ↦* [interface.mkOk go.string #(go!"a text or integer key")]))
    hfields hleft hstart hnonempty hsorted hcache (fun j entry _ he => hmiss j entry he) hp hbytes
    (wp_otherField_tag c st v xs payload rem head argument dq kind enc htext huint hneg htag hp hbytes)

/-- The public field wrapper propagates the actual key rejection with exact
    cursor rollback, one-pair progress and owned diagnostic arguments. -/
theorem wp_nextField_key_type (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (order : List cursor.StaticField) (names : List GoString) (xs payload rem : List w8)
    (head : w8) (argument : w64) (qf qo dq : DFrac) (kind : GoInterface)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ order.length)
    (hnonempty : ∀ entry ∈ order, entry.Encoded' ≠ []) (hsorted : FieldHeadsOrdered order)
    (hcache : CachedShortTable order) (htable : InlineFieldTable order names)
    (enc : HeadArgument head payload argument)
    (htext : head &&& W8 224 ≠ W8 96)
    (huint : head &&& W8 224 ≠ W8 0) (hneg : head &&& W8 224 ≠ W8 32)
    (htag : head &&& W8 224 ≠ W8 192)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrType ↦□ kind ∗ c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ st ↦ {s with left' := s.left' - W64 1} ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"expected %s, found %s") args ∗
       args ↦* [interface.mkOk go.string #(go!"a text or integer key"), interface.mkOk go.string #(headDescription head)] }} := by
  have hmiss := inlineFieldTable_head_miss order names head (payload ++ rem) htable (nontext_head_outside head htext)
  exact wp_nextField_reject c st fields v v s f order xs ((head :: payload) ++ rem) qf qo dq
    iprop(globalAddr cursor.ErrType ↦□ kind)
    (fun err => iprop(∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"expected %s, found %s") args ∗
       args ↦* [interface.mkOk go.string #(go!"a text or integer key"), interface.mkOk go.string #(headDescription head)]))
    hfields hleft hstart hnonempty hsorted hcache (fun j entry _ he => hmiss j entry he) hp hbytes
    (wp_otherField_type c st v xs payload rem head argument dq kind enc htext huint hneg htag hp hbytes)

/-- Public text-body truncation rejection for all five supported head widths,
    including oversized unsigned lengths, with original cursor and input restored. -/
theorem wp_nextField_text_truncated (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (order : List cursor.StaticField) (names : List GoString) (xs payload rem : List w8)
    (head : w8) (argument : w64) (qf qo dq : DFrac) (kind : GoInterface)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ order.length)
    (hnonempty : ∀ entry ∈ order, entry.Encoded' ≠ []) (hsorted : FieldHeadsOrdered order)
    (hcache : CachedShortTable order) (htable : InlineFieldTable order names)
    (enc : HeadArgument head payload argument)
    (hm : head &&& W8 224 = W8 96)
    (hshort : rem.length < uint.Z argument)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ st ↦ {s with left' := s.left' - W64 1} ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"key longer than the input") slice.nil }} := by
  have hmiss := inlineFieldTable_text_truncated_miss order names head payload rem argument htable enc hshort
  exact wp_nextField_reject c st fields v v s f order xs ((head :: payload) ++ rem) qf qo dq
    iprop(globalAddr cursor.ErrTruncated ↦□ kind)
    (fun err => iprop(err ↦ cursor.DecodeError.mk kind v.pos' (go!"key longer than the input") slice.nil))
    hfields hleft hstart hnonempty hsorted hcache (fun j entry _ he => hmiss j entry he) hp hbytes
    (wp_otherField_text_truncated c st v xs payload rem head argument dq kind enc hm hshort hp hbytes)

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_key_tag
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_key_type

#print axioms Perennial.github_com.NethermindEth.juno.core.inlineFieldTable_text_truncated_miss
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_text_truncated
