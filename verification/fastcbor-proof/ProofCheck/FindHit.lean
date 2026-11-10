import ProofCheck.FindMiss

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

/-- A known cached-table field either sets its seen bit or owns a duplicate diagnostic. -/
def KnownFieldOutcome (st : Loc) (s : cursor.Struct) (index : w64) (name : GoString)
    (code : w64) (err kind : GoInterface) (offset : w64) : IProp GF := iprop(
  (⌜code = index ∧ err = interface.nil ∧ s.seen' &&& (W64 1 <<< index) = W64 0⌝ ∗
      st ↦ {s with seen' := s.seen' ||| (W64 1 <<< index)}) ∨
  ∃ (errorLoc : Loc) (args : GoSlice),
    ⌜code = W64 (-1) ∧ err = interface.mkOk (go.PointerType cursor.DecodeError.ty) #errorLoc ∧
      s.seen' &&& (W64 1 <<< index) ≠ W64 0⌝ ∗ st ↦ s ∗
    errorLoc ↦ cursor.DecodeError.mk kind offset (go!"field %s named twice") args ∗
    args ↦* [interface.mkOk go.string #name])

/-- A unique short-buffer table hit consumes its encoded key, updates the search
    index, and performs the real duplicate check at the post-key cursor position. -/
theorem wp_nextField_slow_short_hit (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (order : List cursor.StaticField) (names : List GoString) (entry : cursor.StaticField) (name : GoString)
    (xs rem : List w8) (target : Nat) (kind : GoInterface) (qf qo qn dq : DFrac)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ target)
    (hlookup : order[target]? = some entry) (hnonempty : ∀ key ∈ order, key.Encoded' ≠ [])
    (hsorted : FieldHeadsOrdered order)
    (hapart : FieldEncodingsApart order)
    (hwindow : (entry.Encoded' ++ rem).length < 24)
    (hindex : 0 ≤ sint.Z entry.Index' ∧ sint.Z entry.Index' < 64)
    (hname : names[(sint.Z entry.Index').toNat]? = some name)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = entry.Encoded' ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ c ↦ v ∗ st ↦ s ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"nextField")) (Val #st))
    {{ (code : w64) (err : GoInterface), RET (PairV #code #err);
       c ↦ {v with pos' := v.pos' + W64 entry.Encoded'.length} ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 entry.Encoded'.length) = sint.Z v.pos' + entry.Encoded'.length ∧
         sint.Z (v.pos' + W64 entry.Encoded'.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 entry.Encoded'.length)) (sint.nat v.buf'.len) xs = rem⌝ ∗
       KnownFieldOutcome st {s with left' := s.left' - W64 1, next' := W64 (target + 1)} entry.Index' name
         code err kind (v.pos' + W64 entry.Encoded'.length) }} := by
  wp_start as ⟨#Hkind, Hc, Hst, Hfields, Horder, Hnames, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  ihave %Horderlen := ownSlice_len _ _ _ $$ Horder
  have ht := (List.getElem?_eq_some_iff.mp hlookup).choose
  have Hnext := input_consumption xs entry.Encoded' rem v.pos' v.buf'.len hp Hlen.1 hbytes
  wp_auto
  have hn : s.left' ≠ W64 0 := by word
  simp only [hn, _root_.decide_false]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp.1, hp.2, Hwf.2⟩)]
  wp_auto
  icases (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).1
    $$ Hs with ⟨Hpre, Hbuf, Hpost⟩
  isimp only [hbytes] at Hbuf
  wp_apply wp_find_short st fields {s with left' := s.left' - W64 1} f order
    (slice.slice v.buf' w8 v.pos' v.buf'.len) (entry.Encoded' ++ rem) qf qo dq hfields
    ⟨hstart.1, by change (sint.Z s.next').toNat ≤ order.length; omega⟩ hwindow hnonempty hsorted $$ [Hst Hfields Horder Hbuf] as %result ⟨Hst, Hfields, Horder, Hbuf, %Hfind⟩
  · iframe Hst Hfields Horder Hbuf
  have hresult := fieldFindResult_exact order (entry.Encoded' ++ rem) (sint.Z s.next').toNat target entry result
    Hfind hstart.2 hlookup (List.take_left)
    (fun j other _ hj hp => fieldPrefix_unique order (entry.Encoded' ++ rem) hapart j target other entry
      hj hlookup hp List.take_left)
  subst result
  have htarget : 0 ≤ sint.Z (W64 target) ∧ sint.Z (W64 target) < sint.Z f.order'.len := by word
  have hi : sint.Z (W64 target) = target := by word
  have hzero : sint.Z (W64 0) ≤ sint.Z (W64 target) := htarget.1
  simp only [hzero, _root_.decide_true]
  wp_auto
  simp only [hfields]
  icases (cursor.Fields.Fields_access_load_order fields f qf).access_strict $$ Hfields with ⟨Hfo, Hrestore⟩
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true htarget)]
  icases ownSlice_elem_acc (sint.Z (W64 target)) entry f.order' qo order htarget.1
    (by simpa only [hi, Int.toNat_natCast] using hlookup) $$ Horder with ⟨Hkey, Hclose⟩
  wp_auto
  icases (cursor.StaticField.StaticField_access_load_Encoded
      (sliceIndexRef cursor.StaticField (sint.Z (W64 target)) f.order') entry qo).access_strict $$ Hkey with ⟨Hencoded, HrestoreKey⟩
  wp_auto
  wp_apply github_com.mit_pdos.perennial.goose.model.strings.wp_string_len with %-
  ihave Hkey := HrestoreKey $$ Hencoded
  icases (cursor.StaticField.StaticField_access_load_Index
      (sliceIndexRef cursor.StaticField (sint.Z (W64 target)) f.order') entry qo).access_strict $$ Hkey with ⟨Hindex, HrestoreKey⟩
  wp_auto
  ihave Hkey := HrestoreKey $$ Hindex
  ihave Horder := Hclose $$ Hkey
  have hset : order.set (sint.Z (W64 target)).toNat entry = order := by
    rw [hi, Int.toNat_natCast]
    have he : order[target]'ht = entry := (List.getElem?_eq_some_iff.mp hlookup).choose_spec
    rw [← he]; exact List.set_getElem_self ht
  isimp only [hset] at Horder
  ihave Hfields := Hrestore $$ Hfo
  ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
  · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
    rw [hbytes]; iframe Hpre Hbuf Hpost
  have hinc : W64 target + W64 1 = W64 (target + 1) := by word
  isimp only [hinc, hfields] at Hst
  by_cases hnew : s.seen' &&& (W64 1 <<< entry.Index') = W64 0
  · wp_apply wp_mark_new c st {s with left' := s.left' - W64 1, next' := W64 (target + 1)} entry.Index'
      hindex hnew $$ [Hst] with Hst
    · isimp only [hfields]
      iframe Hst
    isimp only [hfields] at Hst
    iapply HΦ
    iframe Hc Hfields Horder Hnames Hs
    isplitr [Hst]
    · ipureintro; exact Hnext
    unfold KnownFieldOutcome
    ileft
    iframe Hst
    ipureintro; exact ⟨rfl, rfl, hnew⟩
  · wp_apply wp_mark_duplicate c st fields {v with pos' := v.pos' + W64 entry.Encoded'.length}
      {s with left' := s.left' - W64 1, next' := W64 (target + 1)} f names name entry.Index' kind qf qn hfields hindex hname hnew
      $$ [Hc Hst Hfields Hnames] as %errorLoc ⟨Hc, Hst, Hfields, Hnames, Herr⟩
    · isimp only [hfields]
      iframe Hkind Hc Hst Hfields Hnames
    icases Herr with ⟨%args, Herr, Hargs⟩
    isimp only [hfields] at Hst
    iapply HΦ
    iframe Hc Hfields Horder Hnames Hs
    isplitr [Hst Herr Hargs]
    · ipureintro; exact Hnext
    unfold KnownFieldOutcome
    iright
    iexists errorLoc, args
    iframe Hst Herr Hargs
    ipureintro; exact ⟨rfl, rfl, hnew⟩

/-- Public NextField preserves the complete short-hit contract, including duplicate errors. -/
theorem wp_nextField_short_hit (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (order : List cursor.StaticField) (names : List GoString) (entry : cursor.StaticField) (name : GoString)
    (xs rem : List w8) (target : Nat) (kind : GoInterface) (qf qo qn dq : DFrac)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ target)
    (hlookup : order[target]? = some entry) (hnonempty : ∀ key ∈ order, key.Encoded' ≠ [])
    (hsorted : FieldHeadsOrdered order)
    (hapart : FieldEncodingsApart order)
    (hwindow : (entry.Encoded' ++ rem).length < 24)
    (hindex : 0 ≤ sint.Z entry.Index' ∧ sint.Z entry.Index' < 64)
    (hname : names[(sint.Z entry.Index').toNat]? = some name)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = entry.Encoded' ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ c ↦ v ∗ st ↦ s ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ (code : w64) (err : GoInterface), RET (PairV #code #err);
       c ↦ {v with pos' := v.pos' + W64 entry.Encoded'.length} ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 entry.Encoded'.length) = sint.Z v.pos' + entry.Encoded'.length ∧
         sint.Z (v.pos' + W64 entry.Encoded'.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 entry.Encoded'.length)) (sint.nat v.buf'.len) xs = rem⌝ ∗
       KnownFieldOutcome st {s with left' := s.left' - W64 1, next' := W64 (target + 1)} entry.Index' name
         code err kind (v.pos' + W64 entry.Encoded'.length) }} := by
  wp_start as ⟨#Hkind, Hc, Hst, Hfields, Horder, Hnames, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hremaining : sint.Z v.buf'.len - sint.Z v.pos' < 24 := by
    have hl := congrArg List.length hbytes
    simp only [subslice, List.length_drop, List.length_take] at hl
    have hplen : sint.nat v.pos' ≤ xs.length := by word
    have hblen : sint.nat v.buf'.len = xs.length := Hlen.1.symm
    rw [hblen, Nat.min_self] at hl
    word
  wp_auto
  have hleft' : sint.Z (W64 0) < sint.Z s.left' := hleft
  simp only [hleft', _root_.decide_true]
  wp_auto
  simp only [hfields]
  icases (cursor.Fields.Fields_access_load_order fields f qf).access_strict $$ Hfields with ⟨Hfo, Hrestore⟩
  wp_auto
  by_cases hnext : sint.Z s.next' < sint.Z f.order'.len
  all_goals
    simp only [hnext, _root_.decide_true, _root_.decide_false]
    wp_auto
    have hguard : ¬ sint.Z (W64 24) ≤ sint.Z (v.buf'.len - v.pos') := by word
    try simp only [hguard, _root_.decide_false]
    try wp_auto
    ihave Hfields := Hrestore $$ Hfo
    wp_apply wp_nextField_slow_short_hit c st fields v s f order names entry name xs rem target kind qf qo qn dq
      hfields hleft hstart hlookup hnonempty hsorted hapart hwindow hindex hname hp hbytes
      $$ [Hc Hst Hfields Horder Hnames Hs] as %code %err ⟨Hc, Hfields, Horder, Hnames, Hs, %Hnext, Hout⟩
    · iframe Hkind Hc Hst Hfields Horder Hnames Hs
    isimp only [hfields] at Hout
    iapply HΦ
    iframe Hc Hfields Horder Hnames Hs Hout
    ipureintro; exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_slow_short_hit

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_short_hit
