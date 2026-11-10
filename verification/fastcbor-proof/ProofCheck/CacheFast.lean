import ProofCheck.CacheWords

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

/-- Cache representation for an encoded key covered by the three-word fast path. -/
def cachedField (encoded : GoString) (index : w64) : cursor.StaticField :=
  cursor.StaticField.mk
    (array.mk 3 [leToU64 (encoded.take 8), leToU64 ((encoded.drop 8).take 8), leToU64 ((encoded.drop 16).take 8)])
    (array.mk 3 [cacheMask (encoded.take 8).length, cacheMask ((encoded.drop 8).take 8).length,
      cacheMask ((encoded.drop 16).take 8).length]) encoded index (W64 1 <<< index) true

/-- Every concrete generated Header entry has the cache representation used by the fast-path proof. -/
theorem headerCachedFields_valid : ∀ key ∈ headerFieldKeys, key = cachedField key.Encoded' key.Index' := by
  simp only [headerFieldKeys, List.mem_cons, List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq]
  exact ⟨rfl, rfl, rfl, rfl, rfl, rfl, rfl, rfl, rfl, rfl, rfl, rfl, rfl, rfl, rfl⟩

/-- Both concrete GasPrice entries have the same representation. -/
theorem gasCachedFields_valid : ∀ key ∈ gasFieldKeys, key = cachedField key.Encoded' key.Index' := by
  simp only [gasFieldKeys, List.mem_cons, List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq]
  exact ⟨rfl, rfl⟩

/-- A matching unseen short key advances the actual NextField fast path exactly once. -/
theorem wp_nextField_cached (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (order : List cursor.StaticField) (xs input tail : List w8) (encoded : GoString) (index : w64)
    (qf qo dq : DFrac)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hnext : 0 ≤ sint.Z s.next' ∧ sint.Z s.next' < sint.Z f.order'.len)
    (hlookup : order[(sint.Z s.next').toNat]? = some (cachedField encoded index))
    (hnew : s.seen' &&& (W64 1 <<< index) = W64 0)
    (hinput : input.length = 24) (hkey : encoded.length ≤ 24)
    (hmatch : input.take encoded.length = encoded)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = input ++ tail) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ RET (PairV #index #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 encoded.length} ∗
       st ↦ {s with left' := s.left' - W64 1, next' := s.next' + W64 1, seen' := s.seen' ||| (W64 1 <<< index)} ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 encoded.length) = sint.Z v.pos' + encoded.length ∧
         sint.Z (v.pos' + W64 encoded.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 encoded.length)) (sint.nat v.buf'.len) xs = input.drop encoded.length ++ tail⌝ }} := by
  wp_start as ⟨Hc, Hst, Hfields, Horder, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have Hnext := input_consumption xs input tail v.pos' v.buf'.len hp Hlen.1 hbytes
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
      rw [← Hlen.1, subslice_to_end _ _ _ (Nat.le_refl _)] at hbytes
      exact hbytes
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
  have hcmp := (cached_compare_prefix input encoded (by omega) hkey).mpr hmatch
  simp only [cacheCompare, cacheChunk, List.drop_zero] at hcmp
  simp only [cachedField, hnew, hcmp]
  wp_auto
  ieval (change (sliceIndexRef cursor.StaticField (sint.Z s.next') f.order' ↦{qo}
    cachedField encoded index : IProp GF) ⊢ _) at Hkey
  icases (cursor.StaticField.StaticField_access_load_Short
      (sliceIndexRef cursor.StaticField (sint.Z s.next') f.order') (cachedField encoded index) qo).access_strict
      $$ Hkey with ⟨Hvalue, HrestoreValue⟩
  wp_auto
  ihave Hkey := HrestoreValue $$ Hvalue
  icases (cursor.StaticField.StaticField_access_load_Encoded
      (sliceIndexRef cursor.StaticField (sint.Z s.next') f.order') (cachedField encoded index) qo).access_strict
      $$ Hkey with ⟨Hvalue, HrestoreValue⟩
  wp_auto
  ihave Hkey := HrestoreValue $$ Hvalue
  wp_apply github_com.mit_pdos.perennial.goose.model.strings.wp_string_len with %-
  icases (cursor.StaticField.StaticField_access_load_Bit
      (sliceIndexRef cursor.StaticField (sint.Z s.next') f.order') (cachedField encoded index) qo).access_strict
      $$ Hkey with ⟨Hvalue, HrestoreValue⟩
  wp_auto
  ihave Hkey := HrestoreValue $$ Hvalue
  icases (cursor.StaticField.StaticField_access_load_Index
      (sliceIndexRef cursor.StaticField (sint.Z s.next') f.order') (cachedField encoded index) qo).access_strict
      $$ Hkey with ⟨Hvalue, HrestoreValue⟩
  wp_auto
  ihave Hkey := HrestoreValue $$ Hvalue
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
  isimp only [hfields] at Hst
  iapply HΦ
  iframe Hc Hst Hfields Horder Hs
  ipureintro
  have hsplit : encoded ++ input.drop encoded.length = input := by
    have h := List.take_append_drop encoded.length input
    rwa [hmatch] at h
  have hfull : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      encoded ++ (input.drop encoded.length ++ tail) := by
    rw [hbytes, ← List.append_assoc, hsplit]
  exact input_consumption xs encoded (input.drop encoded.length ++ tail) v.pos' v.buf'.len hp Hlen.1 hfull

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_cached

#print axioms Perennial.github_com.NethermindEth.juno.core.headerCachedFields_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.gasCachedFields_valid
