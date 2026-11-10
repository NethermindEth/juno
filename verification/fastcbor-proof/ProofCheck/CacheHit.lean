import ProofCheck.CacheMiss

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

/-- A later table hit or a previously seen expected field falls through the
    actual word cache into the proved search/mark path without changing state first. -/
theorem wp_nextField_cached_fallthrough_hit (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (order : List cursor.StaticField) (names : List GoString) (entry : cursor.StaticField) (name : GoString)
    (xs rem input tail : List w8) (encoded : GoString) (index : w64) (target : Nat) (kind : GoInterface) (qf qo qn dq : DFrac)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ target)
    (hlookup : order[target]? = some entry) (hnonempty : ∀ key ∈ order, key.Encoded' ≠ [])
    (hsorted : FieldHeadsOrdered order)
    (hapart : FieldEncodingsApart order)
    (hcache : CachedShortTable order)
    (hnext : 0 ≤ sint.Z s.next' ∧ sint.Z s.next' < sint.Z f.order'.len)
    (hexpected : order[(sint.Z s.next').toNat]? = some (cachedField encoded index))
    (hinput : input.length = 24)
    (hblocked : cacheCompare input encoded ||| (s.seen' &&& (W64 1 <<< index)) ≠ W64 0)
    (hwindow : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = input ++ tail)
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
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have Hnext := input_consumption xs input tail v.pos' v.buf'.len hp Hlen.1 hwindow
  have hspace : sint.Z v.pos' + 24 ≤ sint.Z v.buf'.len := by rw [hinput] at Hnext; omega
  wp_auto
  have hleft' : sint.Z (W64 0) < sint.Z s.left' := hleft
  simp only [hleft', _root_.decide_true]
  wp_auto
  simp only [hfields]
  icases (cursor.Fields.Fields_access_load_order fields f qf).access_strict $$ Hfields with ⟨Hfo, Hrestore⟩
  wp_auto
  simp only [hnext.2, _root_.decide_true]
  wp_auto
  have hroom : sint.Z (v.pos' + W64 24) ≤ sint.Z v.buf'.len := by word
  have hguard : sint.Z (W64 24) ≤ sint.Z (v.buf'.len - v.pos') := by word
  simp only [hguard, _root_.decide_true]
  wp_auto
  simp only [hfields]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true hnext)]
  icases ownSlice_elem_acc (sint.Z s.next') (cachedField encoded index) f.order' qo order
    hnext.1 hexpected $$ Horder with ⟨Hkey, Hclose⟩
  wp_auto
  have hslice : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z (v.pos' + W64 24) ∧
      sint.Z (v.pos' + W64 24) ≤ sint.Z v.buf'.cap := by word
  rw [ite_eq_left_of_eq_true _ _ (eq_true hslice)]
  wp_auto
  have hbuflen : (slice.slice v.buf' w8 v.pos' (v.pos' + W64 24)).len = W64 24 := by
    simp only [slice.slice]; word
  have hbufbytes : subslice (sint.nat v.pos') (sint.nat (v.pos' + W64 24)) xs = input := by
    have hsum : sint.nat (v.pos' + W64 24) = sint.nat v.pos' + 24 := by word
    rw [hsum, ← subslice_take_drop']
    have hdrop : xs.drop (sint.nat v.pos') = input ++ tail := by
      rw [← Hlen.1, subslice_to_end _ _ _ (Nat.le_refl _)] at hwindow
      exact hwindow
    rw [hdrop, ← hinput, List.take_left]
  icases (ownSlice_slice v.pos' (v.pos' + W64 24) v.buf' dq xs ⟨hp.1, by word, hroom⟩).1
    $$ Hs with ⟨Hpre, Hbuf, Hpost⟩
  isimp only [hbufbytes] at Hbuf
  wp_apply encoding.binary.wp_LittleEndian_Uint64
    (slice.slice v.buf' w8 v.pos' (v.pos' + W64 24)) (input.take 8) dq (input.drop 8)
    (by simp [hinput]) $$ [Hbuf] with Hbuf
  · simp only [List.take_append_drop]; iframe Hbuf
  isimp only [List.take_append_drop] at Hbuf
  have htwentyfour : v.pos' + W64 24 - v.pos' = W64 24 := by word
  have hcap24 : 24 ≤ sint.Z (v.buf'.cap - v.pos') := by word
  icases (cursor.StaticField.StaticField_access_load_Words
      (sliceIndexRef cursor.StaticField (sint.Z s.next') f.order') (cachedField encoded index) qo).access_strict
      $$ Hkey with ⟨Harray, HrestoreArray⟩
  icases array_acc (structFieldRef cursor.StaticField (go!"Words")
      (sliceIndexRef cursor.StaticField (sint.Z s.next') f.order')) 0 qo 3
      (cachedField encoded index).Words' (leToU64 (encoded.take 8)) (by decide) rfl $$ Harray with ⟨Hword, HrestoreWord⟩
  simp only [show sint.Z (W64 0) = 0 from rfl]
  wp_load
  wp_auto
  ihave Harray := HrestoreWord $$ %(leToU64 (encoded.take 8)) Hword
  ieval (change (structFieldRef cursor.StaticField (go!"Words")
    (sliceIndexRef cursor.StaticField (sint.Z s.next') f.order') ↦{qo} (cachedField encoded index).Words' : IProp GF) ⊢ _) at Harray
  ihave Hkey := HrestoreArray $$ Harray
  icases (cursor.StaticField.StaticField_access_load_Masks
      (sliceIndexRef cursor.StaticField (sint.Z s.next') f.order') (cachedField encoded index) qo).access_strict
      $$ Hkey with ⟨Harray, HrestoreArray⟩
  icases array_acc (structFieldRef cursor.StaticField (go!"Masks")
      (sliceIndexRef cursor.StaticField (sint.Z s.next') f.order')) 0 qo 3
      (cachedField encoded index).Masks' (cacheMask (encoded.take 8).length) (by decide) rfl $$ Harray with ⟨Hword, HrestoreWord⟩
  simp only [show sint.Z (W64 0) = 0 from rfl]
  wp_load
  wp_auto
  ihave Harray := HrestoreWord $$ %(cacheMask (encoded.take 8).length) Hword
  ieval (change (structFieldRef cursor.StaticField (go!"Masks")
    (sliceIndexRef cursor.StaticField (sint.Z s.next') f.order') ↦{qo} (cachedField encoded index).Masks' : IProp GF) ⊢ _) at Harray
  ihave Hkey := HrestoreArray $$ Harray
  simp only [htwentyfour]
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by decide, hcap24⟩)]
  wp_auto
  have hread8 : subslice (sint.nat (W64 8))
      (sint.nat (slice.slice v.buf' w8 v.pos' (v.pos' + W64 24)).len) input =
      (input.drop 8).take 8 ++ (input.drop 8).drop 8 := by
    rw [hbuflen, List.take_append_drop]
    simp only [show sint.nat (W64 8) = 8 by rfl, show sint.nat (W64 24) = 24 by rfl]
    rw [subslice, List.take_of_length_le (by omega)]
  wp_apply wp_littleEndian_offset (slice.slice v.buf' w8 v.pos' (v.pos' + W64 24)) input
    ((input.drop 8).take 8) ((input.drop 8).drop 8) (W64 8) (W64 24) dq hbuflen.symm
    ⟨by decide, by rw [hbuflen]; decide⟩ hread8 (by simp [hinput]) $$ [Hbuf] with Hbuf
  · iframe Hbuf
  icases (cursor.StaticField.StaticField_access_load_Words
      (sliceIndexRef cursor.StaticField (sint.Z s.next') f.order') (cachedField encoded index) qo).access_strict
      $$ Hkey with ⟨Harray, HrestoreArray⟩
  icases array_acc (structFieldRef cursor.StaticField (go!"Words")
      (sliceIndexRef cursor.StaticField (sint.Z s.next') f.order')) 1 qo 3
      (cachedField encoded index).Words' (leToU64 ((encoded.drop 8).take 8)) (by decide) rfl $$ Harray with ⟨Hword, HrestoreWord⟩
  simp only [show sint.Z (W64 1) = 1 from rfl]
  wp_load
  wp_auto
  ihave Harray := HrestoreWord $$ %(leToU64 ((encoded.drop 8).take 8)) Hword
  ieval (change (structFieldRef cursor.StaticField (go!"Words")
    (sliceIndexRef cursor.StaticField (sint.Z s.next') f.order') ↦{qo} (cachedField encoded index).Words' : IProp GF) ⊢ _) at Harray
  ihave Hkey := HrestoreArray $$ Harray
  icases (cursor.StaticField.StaticField_access_load_Masks
      (sliceIndexRef cursor.StaticField (sint.Z s.next') f.order') (cachedField encoded index) qo).access_strict
      $$ Hkey with ⟨Harray, HrestoreArray⟩
  icases array_acc (structFieldRef cursor.StaticField (go!"Masks")
      (sliceIndexRef cursor.StaticField (sint.Z s.next') f.order')) 1 qo 3
      (cachedField encoded index).Masks' (cacheMask ((encoded.drop 8).take 8).length) (by decide) rfl $$ Harray with ⟨Hword, HrestoreWord⟩
  simp only [show sint.Z (W64 1) = 1 from rfl]
  wp_load
  wp_auto
  ihave Harray := HrestoreWord $$ %(cacheMask ((encoded.drop 8).take 8).length) Hword
  ieval (change (structFieldRef cursor.StaticField (go!"Masks")
    (sliceIndexRef cursor.StaticField (sint.Z s.next') f.order') ↦{qo} (cachedField encoded index).Masks' : IProp GF) ⊢ _) at Harray
  ihave Hkey := HrestoreArray $$ Harray
  simp only [htwentyfour]
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by decide, hcap24⟩)]
  wp_auto
  have hread16 : subslice (sint.nat (W64 16))
      (sint.nat (slice.slice v.buf' w8 v.pos' (v.pos' + W64 24)).len) input =
      (input.drop 16).take 8 ++ (input.drop 16).drop 8 := by
    rw [hbuflen, List.take_append_drop]
    simp only [show sint.nat (W64 16) = 16 by rfl, show sint.nat (W64 24) = 24 by rfl]
    rw [subslice, List.take_of_length_le (by omega)]
  wp_apply wp_littleEndian_offset (slice.slice v.buf' w8 v.pos' (v.pos' + W64 24)) input
    ((input.drop 16).take 8) ((input.drop 16).drop 8) (W64 16) (W64 24) dq hbuflen.symm
    ⟨by decide, by rw [hbuflen]; decide⟩ hread16 (by simp [hinput]) $$ [Hbuf] with Hbuf
  · iframe Hbuf
  icases (cursor.StaticField.StaticField_access_load_Words
      (sliceIndexRef cursor.StaticField (sint.Z s.next') f.order') (cachedField encoded index) qo).access_strict
      $$ Hkey with ⟨Harray, HrestoreArray⟩
  icases array_acc (structFieldRef cursor.StaticField (go!"Words")
      (sliceIndexRef cursor.StaticField (sint.Z s.next') f.order')) 2 qo 3
      (cachedField encoded index).Words' (leToU64 ((encoded.drop 16).take 8)) (by decide) rfl $$ Harray with ⟨Hword, HrestoreWord⟩
  simp only [show sint.Z (W64 2) = 2 from rfl]
  wp_load
  wp_auto
  ihave Harray := HrestoreWord $$ %(leToU64 ((encoded.drop 16).take 8)) Hword
  ieval (change (structFieldRef cursor.StaticField (go!"Words")
    (sliceIndexRef cursor.StaticField (sint.Z s.next') f.order') ↦{qo} (cachedField encoded index).Words' : IProp GF) ⊢ _) at Harray
  ihave Hkey := HrestoreArray $$ Harray
  icases (cursor.StaticField.StaticField_access_load_Masks
      (sliceIndexRef cursor.StaticField (sint.Z s.next') f.order') (cachedField encoded index) qo).access_strict
      $$ Hkey with ⟨Harray, HrestoreArray⟩
  icases array_acc (structFieldRef cursor.StaticField (go!"Masks")
      (sliceIndexRef cursor.StaticField (sint.Z s.next') f.order')) 2 qo 3
      (cachedField encoded index).Masks' (cacheMask ((encoded.drop 16).take 8).length) (by decide) rfl $$ Harray with ⟨Hword, HrestoreWord⟩
  simp only [show sint.Z (W64 2) = 2 from rfl]
  wp_load
  wp_auto
  ihave Harray := HrestoreWord $$ %(cacheMask ((encoded.drop 16).take 8).length) Hword
  ieval (change (structFieldRef cursor.StaticField (go!"Masks")
    (sliceIndexRef cursor.StaticField (sint.Z s.next') f.order') ↦{qo} (cachedField encoded index).Masks' : IProp GF) ⊢ _) at Harray
  ihave Hkey := HrestoreArray $$ Harray
  icases (cursor.StaticField.StaticField_access_load_Bit
      (sliceIndexRef cursor.StaticField (sint.Z s.next') f.order') (cachedField encoded index) qo).access_strict
      $$ Hkey with ⟨Hbit, HrestoreBit⟩
  wp_auto
  ihave Hkey := HrestoreBit $$ Hbit
  have hor := hblocked
  simp only [cacheCompare, cacheChunk, List.drop_zero] at hor
  simp only [cachedField, hor, _root_.decide_false]
  wp_auto
  ieval (change (sliceIndexRef cursor.StaticField (sint.Z s.next') f.order' ↦{qo}
    cachedField encoded index : IProp GF) ⊢ _) at Hkey
  ihave Horder := Hclose $$ %(cachedField encoded index) Hkey
  have hidx := (List.getElem?_eq_some_iff.mp hexpected).choose
  have hval : order[(sint.Z s.next').toNat]'hidx = cachedField encoded index :=
    Option.some.inj ((List.getElem?_eq_getElem hidx).symm.trans hexpected)
  have hset : order.set (sint.Z s.next').toNat (cachedField encoded index) = order := by
    rw [← hval]
    exact List.set_getElem_self hidx
  isimp only [hset] at Horder
  ihave Hfields := Hrestore $$ Hfo
  ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
  · iapply (ownSlice_slice v.pos' (v.pos' + W64 24) v.buf' dq xs ⟨hp.1, by word, hroom⟩).2
    rw [hbufbytes]
    iframe Hpre Hbuf Hpost
  wp_apply wp_nextField_slow_hit c st fields v s f order names entry name xs rem target kind qf qo qn dq
    hfields hleft hstart hlookup hnonempty hsorted hapart hcache hindex hname hp hbytes
    $$ [Hc Hst Hfields Horder Hnames Hs] as %code %err ⟨Hc, Hfields, Horder, Hnames, Hs, %Hadvance, Hout⟩
  · iframe Hkind Hc Hst Hfields Horder Hnames Hs
  isimp only [hfields] at Hout
  iapply HΦ
  iframe Hc Hfields Horder Hnames Hs Hout
  ipureintro; exact Hadvance

/-- Public NextField handles every encoded-table hit at or after the search index,
    for arbitrary remaining input lengths, including new fields and duplicates. -/
theorem wp_nextField_hit (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (order : List cursor.StaticField) (names : List GoString) (entry : cursor.StaticField) (name : GoString)
    (xs rem : List w8) (target : Nat) (kind : GoInterface) (qf qo qn dq : DFrac)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ target)
    (hlookup : order[target]? = some entry) (hnonempty : ∀ key ∈ order, key.Encoded' ≠ [])
    (hsorted : FieldHeadsOrdered order)
    (hapart : FieldEncodingsApart order)
    (hcache : CachedShortTable order)
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
  iintro %Φ ⟨#Hpkg, #Hkind, Hc, Hst, Hfields, Horder, Hnames, Hs⟩ HΦ
  by_cases hwindow : (entry.Encoded' ++ rem).length < 24
  · wp_apply wp_nextField_short_hit c st fields v s f order names entry name xs rem target kind qf qo qn dq
      hfields hleft hstart hlookup hnonempty hsorted hapart hwindow hindex hname hp hbytes
      $$ [Hc Hst Hfields Horder Hnames Hs] as %code %err ⟨Hc, Hfields, Horder, Hnames, Hs, %Hadvance, Hout⟩
    · iframe Hkind Hc Hst Hfields Horder Hnames Hs
    iapply HΦ
    iframe Hc Hfields Horder Hnames Hs Hout
    ipureintro; exact Hadvance
  · ihave %Horderlen := ownSlice_len _ _ _ $$ Horder
    ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    have ht := (List.getElem?_eq_some_iff.mp hlookup).choose
    have hnext : 0 ≤ sint.Z s.next' ∧ sint.Z s.next' < sint.Z f.order'.len := by word
    have hi : (sint.Z s.next').toNat < order.length := by word
    let expected := order[(sint.Z s.next').toNat]'hi
    have hexpected : order[(sint.Z s.next').toNat]? = some expected := List.getElem?_eq_getElem hi
    obtain ⟨hrep, hkey⟩ := hcache expected (List.getElem_mem hi)
    obtain ⟨hentryrep, hentrylen⟩ := hcache entry (List.mem_of_getElem? hlookup)
    have hexpectedcache : order[(sint.Z s.next').toNat]? = some (cachedField expected.Encoded' expected.Index') :=
      hexpected.trans (congrArg some hrep)
    have hinput : ((entry.Encoded' ++ rem).take 24).length = 24 := by simp only [List.length_take]; omega
    have hwindowbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
        (entry.Encoded' ++ rem).take 24 ++ (entry.Encoded' ++ rem).drop 24 := by
      simpa only [List.take_append_drop] using hbytes
    by_cases hfast : (sint.Z s.next').toNat = target ∧ s.seen' &&& (W64 1 <<< entry.Index') = W64 0
    · have hfastlookup : order[(sint.Z s.next').toNat]? = some (cachedField entry.Encoded' entry.Index') := by
        rw [hfast.1]; exact hlookup.trans (congrArg some hentryrep)
      have hprefix : ((entry.Encoded' ++ rem).take 24).take entry.Encoded'.length = entry.Encoded' := by
        rw [List.take_take, Nat.min_eq_left hentrylen, List.take_left]
      wp_apply wp_nextField_cached c st fields v s f order xs ((entry.Encoded' ++ rem).take 24)
        ((entry.Encoded' ++ rem).drop 24) entry.Encoded' entry.Index' qf qo dq hfields hleft hnext
        hfastlookup hfast.2 hinput hentrylen hprefix hp hwindowbytes
        $$ [Hc Hst Hfields Horder Hs] with ⟨Hc, Hst, Hfields, Horder, Hs, %HfastAdvance⟩
      · iframe Hc Hst Hfields Horder Hs
      have hinc : s.next' + W64 1 = W64 (target + 1) := by word
      isimp only [hinc] at Hst
      iapply HΦ
      iframe Hc Hfields Horder Hnames Hs
      isplitr [Hst]
      · ipureintro; exact input_consumption xs entry.Encoded' rem v.pos' v.buf'.len hp Hlen.1 hbytes
      unfold KnownFieldOutcome
      ileft
      iframe Hst
      ipureintro; exact ⟨rfl, rfl, hfast.2⟩
    · have hblocked : cacheCompare ((entry.Encoded' ++ rem).take 24) expected.Encoded' |||
          (s.seen' &&& (W64 1 <<< expected.Index')) ≠ W64 0 := by
        intro hz
        obtain ⟨hcmp, hseen⟩ := BitVec.or_eq_zero_iff.mp hz
        have hprefix := (cached_compare_prefix ((entry.Encoded' ++ rem).take 24) expected.Encoded'
          (by omega) hkey).mp hcmp
        rw [List.take_take, Nat.min_eq_left hkey] at hprefix
        have hsame := fieldPrefix_unique order (entry.Encoded' ++ rem) hapart (sint.Z s.next').toNat target
          expected entry hexpected hlookup hprefix List.take_left
        have hent : expected = entry := by
          have hh := hexpected
          rw [hsame] at hh
          exact Option.some.inj (hh.symm.trans hlookup)
        apply hfast
        refine ⟨hsame, ?_⟩
        rw [hent] at hseen
        exact hseen.trans (by decide)
      wp_apply wp_nextField_cached_fallthrough_hit c st fields v s f order names entry name xs rem
        ((entry.Encoded' ++ rem).take 24) ((entry.Encoded' ++ rem).drop 24) expected.Encoded' expected.Index'
        target kind qf qo qn dq hfields hleft hstart hlookup hnonempty hsorted hapart hcache hnext
        hexpectedcache hinput hblocked hwindowbytes hindex hname hp hbytes
        $$ [Hc Hst Hfields Horder Hnames Hs] as %code %err ⟨Hc, Hfields, Horder, Hnames, Hs, %Hadvance, Hout⟩
      · iframe Hkind Hc Hst Hfields Horder Hnames Hs
      iapply HΦ
      iframe Hc Hfields Horder Hnames Hs Hout
      ipureintro; exact Hadvance

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_cached_fallthrough_hit

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_hit
