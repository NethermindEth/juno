import ProofCheck.FindDispatch

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

/-- A failed expected-key word comparison preserves the cursor and all table state
    before invoking the proved general search/name path. Duplicate bits cannot hide a mismatch. -/
theorem wp_nextField_cached_miss_ascii (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (names : List GoString) (xs payload key rem : List w8) (head : w8)
    (order : List cursor.StaticField) (input tail : List w8) (encoded : GoString) (index : w64) (qf qo qn dq : DFrac) (kind : GoInterface)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ order.length)
    (hcache : CachedShortTable order)
    (hnext : 0 ≤ sint.Z s.next' ∧ sint.Z s.next' < sint.Z f.order'.len)
    (hlookup : order[(sint.Z s.next').toNat]? = some (cachedField encoded index))
    (hinput : input.length = 24) (hkey : encoded.length ≤ 24)
    (hmatch : input.take encoded.length ≠ encoded)
    (hwindow : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = input ++ tail)
    (hnonempty : ∀ entry ∈ order, entry.Encoded' ≠ [])
    (hsorted : FieldHeadsOrdered order)
    (hmiss : ∀ j entry, (sint.Z s.next').toNat ≤ j → order[j]? = some entry →
      ((head :: payload) ++ key ++ rem).take entry.Encoded'.length ≠ entry.Encoded') (hnames : names.length ≤ 64)
    (enc : HeadArgument head payload (W64 key.length))
    (hm : head &&& W8 224 = W8 96)
    (hshort : uint.Z (head &&& W8 31) < 24 ∨ head &&& W8 31 = W8 24)
    (hascii : ASCIIBytes key)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ key ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗
       c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ (code : w64) (err : GoInterface), RET (PairV #code #err);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + key.length)} ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + key.length)) = sint.Z v.pos' + 1 + payload.length + key.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + key.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + key.length))) (sint.nat v.buf'.len) xs = rem⌝ ∗
       FieldReadOutcome st {s with left' := s.left' - W64 1} names key code err kind (v.pos' + W64 (1 + payload.length + key.length)) }} := by
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
    hnext.1 hlookup $$ Horder with ⟨Hkey, Hclose⟩
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
  have hcmp : cacheCompare input encoded ≠ W64 0 := mt (cached_compare_prefix input encoded (by omega) hkey).mp hmatch
  have hor : cacheCompare input encoded ||| (s.seen' &&& (W64 1 <<< index)) ≠ W64 0 := by
    intro hz
    exact hcmp (BitVec.or_eq_zero_iff.mp hz).1
  simp only [cacheCompare, cacheChunk, List.drop_zero] at hor
  simp only [cachedField, hor, _root_.decide_false]
  wp_auto
  ieval (change (sliceIndexRef cursor.StaticField (sint.Z s.next') f.order' ↦{qo}
    cachedField encoded index : IProp GF) ⊢ _) at Hkey
  ihave Horder := Hclose $$ %(cachedField encoded index) Hkey
  have hidx := (List.getElem?_eq_some_iff.mp hlookup).choose
  have hval : order[(sint.Z s.next').toNat]'hidx = cachedField encoded index :=
    Option.some.inj ((List.getElem?_eq_getElem hidx).symm.trans hlookup)
  have hset : order.set (sint.Z s.next').toNat (cachedField encoded index) = order := by
    rw [← hval]
    exact List.set_getElem_self hidx
  isimp only [hset] at Horder
  ihave Hfields := Hrestore $$ Hfo
  ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
  · iapply (ownSlice_slice v.pos' (v.pos' + W64 24) v.buf' dq xs ⟨hp.1, by word, hroom⟩).2
    rw [hbufbytes]
    iframe Hpre Hbuf Hpost
  wp_apply wp_nextField_slow_miss_ascii c st fields v s f names xs payload key rem head order qf qo qn dq kind
    hfields hleft hstart hcache hnonempty hsorted hmiss hnames enc hm hshort hascii hp hbytes
    $$ [Hc Hst Hfields Horder Hnames Hs] as %code %err ⟨Hc, Hfields, Horder, Hnames, Hs, %Hadvance, Hout⟩
  · iframe Hkind Hc Hst Hfields Horder Hnames Hs
  isimp only [hfields] at Hout
  iapply HΦ
  iframe Hc Hfields Horder Hnames Hs Hout
  ipureintro; exact Hadvance

/-- Public field reading after an absent encoded-table prefix, independent of the
    remaining input length. The actual cache, search, parser and duplicate check compose. -/
theorem wp_nextField_miss_ascii (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (names : List GoString) (xs payload key rem : List w8) (head : w8)
    (order : List cursor.StaticField) (qf qo qn dq : DFrac) (kind : GoInterface)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ order.length)
    (hcache : CachedShortTable order)
    (hnonempty : ∀ entry ∈ order, entry.Encoded' ≠ [])
    (hsorted : FieldHeadsOrdered order)
    (hmiss : ∀ j entry, (sint.Z s.next').toNat ≤ j → order[j]? = some entry →
      ((head :: payload) ++ key ++ rem).take entry.Encoded'.length ≠ entry.Encoded') (hnames : names.length ≤ 64)
    (enc : HeadArgument head payload (W64 key.length))
    (hm : head &&& W8 224 = W8 96)
    (hshort : uint.Z (head &&& W8 31) < 24 ∨ head &&& W8 31 = W8 24)
    (hascii : ASCIIBytes key)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ key ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗
       c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ (code : w64) (err : GoInterface), RET (PairV #code #err);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + key.length)} ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + key.length)) = sint.Z v.pos' + 1 + payload.length + key.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + key.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + key.length))) (sint.nat v.buf'.len) xs = rem⌝ ∗
       FieldReadOutcome st {s with left' := s.left' - W64 1} names key code err kind (v.pos' + W64 (1 + payload.length + key.length)) }} := by
  iintro %Φ ⟨#Hpkg, #Hkind, Hc, Hst, Hfields, Horder, Hnames, Hs⟩ HΦ
  by_cases hwindow : ((head :: payload) ++ key ++ rem).length < 24
  · wp_apply wp_nextField_short_miss_ascii c st fields v s f names xs payload key rem head order qf qo qn dq kind
      hfields hleft hstart hwindow hnonempty hsorted hmiss hnames enc hm hshort hascii hp hbytes
      $$ [Hc Hst Hfields Horder Hnames Hs] as %code %err ⟨Hc, Hfields, Horder, Hnames, Hs, %Hadvance, Hout⟩
    · iframe Hkind Hc Hst Hfields Horder Hnames Hs
    iapply HΦ
    iframe Hc Hfields Horder Hnames Hs Hout
    ipureintro; exact Hadvance
  · by_cases hnext : sint.Z s.next' < sint.Z f.order'.len
    · ihave %Horderlen := ownSlice_len _ _ _ $$ Horder
      have hi : (sint.Z s.next').toNat < order.length := by word
      let entry := order[(sint.Z s.next').toNat]'hi
      have hlookup : order[(sint.Z s.next').toNat]? = some entry := List.getElem?_eq_getElem hi
      obtain ⟨hrep, hkey⟩ := hcache entry (List.getElem_mem hi)
      have hlookup' : order[(sint.Z s.next').toNat]? = some (cachedField entry.Encoded' entry.Index') := by
        exact hlookup.trans (congrArg some hrep)
      have hinput : (((head :: payload) ++ key ++ rem).take 24).length = 24 := by simp only [List.length_take]; omega
      have hmismatch : (((head :: payload) ++ key ++ rem).take 24).take entry.Encoded'.length ≠ entry.Encoded' := by
        rw [List.take_take, Nat.min_eq_left hkey]
        exact hmiss _ entry (Nat.le_refl _) hlookup
      have hwindowbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
          ((head :: payload) ++ key ++ rem).take 24 ++ ((head :: payload) ++ key ++ rem).drop 24 := by
        simpa only [List.take_append_drop] using hbytes
      wp_apply wp_nextField_cached_miss_ascii c st fields v s f names xs payload key rem head order
        (((head :: payload) ++ key ++ rem).take 24) (((head :: payload) ++ key ++ rem).drop 24)
        entry.Encoded' entry.Index' qf qo qn dq kind hfields hleft hstart hcache ⟨hstart.1, hnext⟩
        hlookup' hinput hkey hmismatch hwindowbytes hnonempty hsorted hmiss hnames enc hm hshort hascii hp hbytes
        $$ [Hc Hst Hfields Horder Hnames Hs] as %code %err ⟨Hc, Hfields, Horder, Hnames, Hs, %Hadvance, Hout⟩
      · iframe Hkind Hc Hst Hfields Horder Hnames Hs
      iapply HΦ
      iframe Hc Hfields Horder Hnames Hs Hout
      ipureintro; exact Hadvance
    · wp_method_call
      wp_call
      wp_auto
      have hleft' : sint.Z (W64 0) < sint.Z s.left' := hleft
      simp only [hleft', _root_.decide_true]
      wp_auto
      simp only [hfields]
      icases (cursor.Fields.Fields_access_load_order fields f qf).access_strict $$ Hfields with ⟨Hfo, Hrestore⟩
      wp_auto
      simp only [hnext, _root_.decide_false]
      wp_auto
      ihave Hfields := Hrestore $$ Hfo
      wp_apply wp_nextField_slow_miss_ascii c st fields v s f names xs payload key rem head order qf qo qn dq kind
        hfields hleft hstart hcache hnonempty hsorted hmiss hnames enc hm hshort hascii hp hbytes
        $$ [Hc Hst Hfields Horder Hnames Hs] as %code %err ⟨Hc, Hfields, Horder, Hnames, Hs, %Hadvance, Hout⟩
      · iframe Hkind Hc Hst Hfields Horder Hnames Hs
      isimp only [hfields] at Hout
      iapply HΦ
      iframe Hc Hfields Horder Hnames Hs Hout
      ipureintro; exact Hadvance

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_cached_miss_ascii

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_miss_ascii
