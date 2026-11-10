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

/-- Hoare composition for a failed fast-cache condition. The slow-path triple
    is an explicit proof argument, discharged by callers using the actual decoder proofs. -/
theorem wp_nextField_cached_else (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (order : List cursor.StaticField) (xs input tail : List w8) (encoded : GoString) (index : w64)
    (qf qo dq : DFrac) (R : IProp GF) (Q : w64 → GoInterface → IProp GF)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hnext : 0 ≤ sint.Z s.next' ∧ sint.Z s.next' < sint.Z f.order'.len)
    (hexpected : order[(sint.Z s.next').toNat]? = some (cachedField encoded index))
    (hinput : input.length = 24)
    (hblocked : cacheCompare input encoded ||| (s.seen' &&& (W64 1 <<< index)) ≠ W64 0)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hwindow : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = input ++ tail)
    (hslow :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         R ∗ c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"nextField")) (Val #st))
      {{ (code : w64) (err : GoInterface), RET (PairV #code #err); Q code err }}) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       R ∗ c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ (code : w64) (err : GoInterface), RET (PairV #code #err); Q code err }} := by
  wp_start as ⟨HR, Hc, Hst, Hfields, Horder, Hs⟩
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
  wp_apply hslow $$ [HR Hc Hst Hfields Horder Hs] as %code %err HQ
  · iframe HR Hc Hst Hfields Horder Hs
  iapply HΦ
  iexact HQ

/-- Search misses compose with a supplied proof of the actual alternate-field reader.
    The search itself and the one-pair decrement are proved here. -/
theorem wp_nextField_slow_other (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (order : List cursor.StaticField) (xs wire : List w8) (qf qo dq : DFrac)
    (R : IProp GF) (Q : w64 → GoInterface → IProp GF)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ order.length)
    (hnonempty : ∀ entry ∈ order, entry.Encoded' ≠ []) (hsorted : FieldHeadsOrdered order)
    (hcache : CachedShortTable order)
    (hmiss : ∀ j entry, (sint.Z s.next').toNat ≤ j → order[j]? = some entry →
      wire.take entry.Encoded'.length ≠ entry.Encoded')
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire)
    (hother :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         R ∗ c ↦ v ∗ st ↦ {s with left' := s.left' - W64 1} ∗ fields ↦{qf} f ∗
         f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"otherField")) (Val #st))
      {{ (code : w64) (err : GoInterface), RET (PairV #code #err); Q code err }}) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       R ∗ c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"nextField")) (Val #st))
    {{ (code : w64) (err : GoInterface), RET (PairV #code #err); Q code err }} := by
  wp_start as ⟨HR, Hc, Hst, Hfields, Horder, Hs⟩
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  wp_auto
  have hn : s.left' ≠ W64 0 := by word
  simp only [hn, _root_.decide_false]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp.1, hp.2, Hwf.2⟩)]
  wp_auto
  icases (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).1
    $$ Hs with ⟨Hpre, Hbuf, Hpost⟩
  isimp only [hbytes] at Hbuf
  wp_apply wp_find_cached st fields {s with left' := s.left' - W64 1} f order
    (slice.slice v.buf' w8 v.pos' v.buf'.len) wire qf qo dq hfields hstart hnonempty hsorted hcache
    $$ [Hst Hfields Horder Hbuf] as %result ⟨Hst, Hfields, Horder, Hbuf, %Hfind⟩
  · iframe Hst Hfields Horder Hbuf
  have hresult := fieldFindResult_miss order wire (sint.Z s.next').toNat result Hfind hmiss
  subst result
  wp_auto
  ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
  · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
    rw [hbytes]; iframe Hpre Hbuf Hpost
  wp_apply hother $$ [HR Hc Hst Hfields Horder Hs] as %code %err HQ
  · iframe HR Hc Hst Hfields Horder Hs
  iapply HΦ
  iexact HQ

/-- Public NextField composition for an absent encoded prefix, covering every input
    length and search-index boundary without assuming cache or search correctness. -/
theorem wp_nextField_other (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (order : List cursor.StaticField) (xs wire : List w8) (qf qo dq : DFrac)
    (R : IProp GF) (Q : w64 → GoInterface → IProp GF)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ order.length)
    (hnonempty : ∀ entry ∈ order, entry.Encoded' ≠ []) (hsorted : FieldHeadsOrdered order)
    (hcache : CachedShortTable order)
    (hmiss : ∀ j entry, (sint.Z s.next').toNat ≤ j → order[j]? = some entry →
      wire.take entry.Encoded'.length ≠ entry.Encoded')
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire)
    (hother :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         R ∗ c ↦ v ∗ st ↦ {s with left' := s.left' - W64 1} ∗ fields ↦{qf} f ∗
         f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"otherField")) (Val #st))
      {{ (code : w64) (err : GoInterface), RET (PairV #code #err); Q code err }}) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       R ∗ c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ (code : w64) (err : GoInterface), RET (PairV #code #err); Q code err }} := by
  iintro %Φ ⟨#Hpkg, HR, Hc, Hst, Hfields, Horder, Hs⟩ HΦ
  have hslow := wp_nextField_slow_other c st fields v s f order xs wire qf qo dq R Q
    hfields hleft hstart hnonempty hsorted hcache hmiss hp hbytes hother
  by_cases henter : sint.Z s.next' < sint.Z f.order'.len ∧ 24 ≤ wire.length
  · ihave %Horderlen := ownSlice_len _ _ _ $$ Horder
    have hi : (sint.Z s.next').toNat < order.length := by word
    let entry := order[(sint.Z s.next').toNat]'hi
    have hlookup : order[(sint.Z s.next').toNat]? = some entry := List.getElem?_eq_getElem hi
    obtain ⟨hrep, hkey⟩ := hcache entry (List.getElem_mem hi)
    have hlookup' : order[(sint.Z s.next').toNat]? = some (cachedField entry.Encoded' entry.Index') :=
      hlookup.trans (congrArg some hrep)
    have hinput : (wire.take 24).length = 24 := by simp only [List.length_take]; omega
    have hmismatch : (wire.take 24).take entry.Encoded'.length ≠ entry.Encoded' := by
      rw [List.take_take, Nat.min_eq_left hkey]
      exact hmiss _ entry (Nat.le_refl _) hlookup
    have hblocked : cacheCompare (wire.take 24) entry.Encoded' |||
        (s.seen' &&& (W64 1 <<< entry.Index')) ≠ W64 0 := by
      intro hz
      exact hmismatch ((cached_compare_prefix (wire.take 24) entry.Encoded' (by omega) hkey).mp
        (BitVec.or_eq_zero_iff.mp hz).1)
    have hwindow : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire.take 24 ++ wire.drop 24 := by
      simpa only [List.take_append_drop] using hbytes
    wp_apply wp_nextField_cached_else c st fields v s f order xs (wire.take 24) (wire.drop 24)
      entry.Encoded' entry.Index' qf qo dq R Q hfields hleft ⟨hstart.1, henter.1⟩ hlookup'
      hinput hblocked hp hwindow hslow $$ [HR Hc Hst Hfields Horder Hs] as %code %err HQ
    · iframe HR Hc Hst Hfields Horder Hs
    iapply HΦ; iexact HQ
  · ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    have hremaining : sint.Z v.buf'.len - sint.Z v.pos' = wire.length := by
      have hl := congrArg List.length hbytes
      simp only [subslice, List.length_drop, List.length_take] at hl
      have hplen : sint.nat v.pos' ≤ xs.length := by word
      rw [← Hlen.1, Nat.min_self] at hl
      word
    wp_method_call
    wp_call
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
      try have hsmall : wire.length < 24 := by omega
      try have hguard : ¬ sint.Z (W64 24) ≤ sint.Z (v.buf'.len - v.pos') := by word
      try simp only [hguard, _root_.decide_false]
      try wp_auto
      ihave Hfields := Hrestore $$ Hfo
      wp_apply hslow $$ [HR Hc Hst Hfields Horder Hs] as %code %err HQ
      · iframe HR Hc Hst Hfields Horder Hs
      iapply HΦ; iexact HQ

/-- Lift a proved key rejection through the public field reader. The leaf proof
    supplies the exact cursor/error state; the wrapper accounts for pair progress
    and preserves the read-only table and complete input. -/
theorem wp_nextField_reject (c st fields : Loc) (v vout : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (order : List cursor.StaticField) (xs wire : List w8) (qf qo dq : DFrac)
    (R : IProp GF) (E : Loc → IProp GF)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ order.length)
    (hnonempty : ∀ entry ∈ order, entry.Encoded' ≠ []) (hsorted : FieldHeadsOrdered order)
    (hcache : CachedShortTable order)
    (hmiss : ∀ j entry, (sint.Z s.next').toNat ≤ j → order[j]? = some entry →
      wire.take entry.Encoded'.length ≠ entry.Encoded')
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire)
    (hreject :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"otherField")) (Val #st))
      {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
         c ↦ vout ∗ v.buf' ↦*{dq} xs ∗ E err }}) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       R ∗ c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ vout ∗ st ↦ {s with left' := s.left' - W64 1} ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs ∗ E err }} := by
  let Q (code : w64) (err : GoInterface) : IProp GF := iprop(∃ errorLoc : Loc,
    ⌜code = W64 (-1) ∧ err = interface.mkOk (go.PointerType cursor.DecodeError.ty) #errorLoc⌝ ∗
    c ↦ vout ∗ st ↦ {s with left' := s.left' - W64 1} ∗ fields ↦{qf} f ∗
    f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs ∗ E errorLoc)
  have hother :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         R ∗ c ↦ v ∗ st ↦ {s with left' := s.left' - W64 1} ∗ fields ↦{qf} f ∗
         f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"otherField")) (Val #st))
      {{ (code : w64) (err : GoInterface), RET (PairV #code #err); Q code err }} := by
    wp_start_folded as ⟨HR, Hc, Hst, Hfields, Horder, Hs⟩
    wp_apply hreject $$ [HR Hc Hs] as %errorLoc ⟨Hc, Hs, Herr⟩
    · iframe HR Hc Hs
    iapply HΦ
    unfold Q
    iexists errorLoc
    iframe Hc Hst Hfields Horder Hs Herr
    ipureintro; exact ⟨rfl, rfl⟩
  iintro %Φ ⟨#Hpkg, HR, Hc, Hst, Hfields, Horder, Hs⟩ HΦ
  wp_apply wp_nextField_other c st fields v s f order xs wire qf qo dq R Q hfields hleft hstart
    hnonempty hsorted hcache hmiss hp hbytes hother $$ [HR Hc Hst Hfields Horder Hs] as %code %err HQ
  · iframe HR Hc Hst Hfields Horder Hs
  isimp only [Q] at HQ
  icases HQ with ⟨%errorLoc, %Heq, Hc, Hst, Hfields, Horder, Hs, Herr⟩
  rcases Heq with ⟨rfl, rfl⟩
  iapply HΦ
  iframe Hc Hst Hfields Horder Hs Herr

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_cached_else

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_slow_other
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_other

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_reject
