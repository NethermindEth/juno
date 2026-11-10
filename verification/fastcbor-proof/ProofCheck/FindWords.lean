import ProofCheck.FindHit

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 3200000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

/-- The word-search prefilter reads exactly the first input byte. -/
theorem leToU64_first_byte (xs : List w8) :
    W8 (uint.Z (leToU64 (xs.take 8))) = xs.headD (W8 0) := by
  cases xs with
  | nil => rfl
  | cons b bs =>
    simp only [List.take_succ_cons, List.headD_cons, leToU64, leToU64Def, LittleEndian.combine]
    word

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- The generated Header/GasPrice field names all fit in the three cached words. -/
def CachedShortTable (order : List cursor.StaticField) : Prop :=
  ∀ key ∈ order, key = cachedField key.Encoded' key.Index' ∧ key.Encoded'.length ≤ 24

theorem headerCachedShortTable : CachedShortTable headerFieldKeys := by
  intro key hmem
  refine ⟨headerCachedFields_valid key hmem, ?_⟩
  have h : ∀ key ∈ headerFieldKeys, key.Encoded'.length ≤ 24 := by
    simp only [headerFieldKeys, List.mem_cons, List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq]
    decide
  exact h key hmem

theorem gasCachedShortTable : CachedShortTable gasFieldKeys := by
  intro key hmem
  refine ⟨gasCachedFields_valid key hmem, ?_⟩
  have h : ∀ key ∈ gasFieldKeys, key.Encoded'.length ≤ 24 := by
    simp only [gasFieldKeys, List.mem_cons, List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq]
    decide
  exact h key hmem

variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Word-based search has the same independent prefix contract as the byte-based
    search, for every input length and table entry covered by the Header cache. -/
theorem wp_find_words (st fields : Loc) (s : cursor.Struct) (f : cursor.Fields)
    (order : List cursor.StaticField) (buf : GoSlice) (xs : List w8) (qf qo dq : DFrac)
    (hfields : s.fields' = fields) (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ order.length)
    (hroom : 24 ≤ xs.length) (hnonempty : ∀ key ∈ order, key.Encoded' ≠ [])
    (hsorted : FieldHeadsOrdered order) (hcache : CachedShortTable order) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       st ↦ s ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ buf ↦*{dq} xs }}
      (App (Val (st @!! go.PointerType cursor.Struct.ty @!! go!"find")) (Val #buf))
    {{ (result : w64), RET #result;
       st ↦ s ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ buf ↦*{dq} xs ∗
       ⌜FieldFindResult order xs (sint.Z s.next').toNat result⌝ }} := by
  wp_start as ⟨Hst, Hfields, Horder, Hbuf⟩
  ihave %Horderlen := ownSlice_len _ _ _ $$ Horder
  ihave %Hlen := ownSlice_len _ _ _ $$ Hbuf
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hbuf
  wp_auto
  simp only [hfields]
  icases (cursor.Fields.Fields_access_load_order fields f qf).access_strict $$ Hfields with ⟨Hfo, Hrestore⟩
  wp_auto
  have hbig : sint.Z (W64 24) ≤ sint.Z buf.len := by word
  simp only [hbig, _root_.decide_true]
  wp_auto
  wp_apply encoding.binary.wp_LittleEndian_Uint64 buf (xs.take 8) dq (xs.drop 8)
    (by simp; omega) $$ [Hbuf] with Hbuf
  · simp only [List.take_append_drop]; iframe Hbuf
  isimp only [List.take_append_drop] at Hbuf
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by word, Hwf.2⟩)]
  wp_auto
  have hread8 : subslice (sint.nat (W64 8)) (sint.nat buf.len) xs =
      (xs.drop 8).take 8 ++ (xs.drop 8).drop 8 := by
    rw [List.take_append_drop, ← Hlen.1]
    simp only [show sint.nat (W64 8) = 8 from rfl, subslice, List.take_length]
  wp_apply wp_littleEndian_offset buf xs ((xs.drop 8).take 8) ((xs.drop 8).drop 8)
    (W64 8) buf.len dq rfl ⟨by decide, by word⟩ hread8 (by simp; omega) $$ [Hbuf] with Hbuf
  · iframe Hbuf
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by word, Hwf.2⟩)]
  wp_auto
  have hread16 : subslice (sint.nat (W64 16)) (sint.nat buf.len) xs =
      (xs.drop 16).take 8 ++ (xs.drop 16).drop 8 := by
    rw [List.take_append_drop, ← Hlen.1]
    simp only [show sint.nat (W64 16) = 16 from rfl, subslice, List.take_length]
  wp_apply wp_littleEndian_offset buf xs ((xs.drop 16).take 8) ((xs.drop 16).drop 8)
    (W64 16) buf.len dq rfl ⟨by decide, by word⟩ hread16 (by simp; omega) $$ [Hbuf] with Hbuf
  · iframe Hbuf
  isimp only [leToU64_first_byte] at head
  have hnext : s.next' = W64 (sint.Z s.next').toNat := by word
  ihave HI : (∃ n : Nat, "Hi" ∷ i_ptr ↦ W64 n ∗
      "%Hbound" ∷ ⌜(sint.Z s.next').toNat ≤ n ∧ n ≤ order.length⌝ ∗
      "%Hmiss" ∷ ⌜∀ j key, (sint.Z s.next').toNat ≤ j → j < n →
        order[j]? = some key → xs.take key.Encoded'.length ≠ key.Encoded'⌝ : IProp GF) $$ [i]
  · iexists (sint.Z s.next').toNat
    isimp only [← hnext]
    iframe
    ipureintro
    exact ⟨⟨Nat.le_refl _, hstart.2⟩, fun j key hj hjn => False.elim (by omega)⟩
  have hf : (#false : val) ≠ #true := fun h => Bool.noConfusion (go.intoVal_inj h)
  wp_for HI
  by_cases hmore : n < order.length
  · have hcond : sint.Z (W64 n) < sint.Z f.order'.len := by word
    simp only [hcond, _root_.decide_true, ite_true]
    wp_auto
    have hi : sint.Z (W64 n) = n := by word
    rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by word, hcond⟩)]
    let key := order[n]'hmore
    have hlookup : order[n]? = some key := List.getElem?_eq_getElem hmore
    have hkeyne : key.Encoded' ≠ [] := hnonempty key (List.getElem_mem hmore)
    obtain ⟨hrep, hkeylen⟩ := hcache key (List.getElem_mem hmore)
    have hwords := congrArg cursor.StaticField.Words' hrep
    have hmasks := congrArg cursor.StaticField.Masks' hrep
    have hshort : key.Short' = true := congrArg cursor.StaticField.Short' hrep
    icases ownSlice_elem_acc (sint.Z (W64 n)) key f.order' qo order (by word)
      (by simpa only [hi, Int.toNat_natCast] using hlookup) $$ Horder with ⟨Hkey, Hclose⟩
    wp_auto
    have hset : order.set (sint.Z (W64 n)).toNat key = order := by
      simp only [hi, Int.toNat_natCast]; exact List.set_getElem_self hmore
    icases (cursor.StaticField.StaticField_access_load_Words
        (sliceIndexRef cursor.StaticField (sint.Z (W64 n)) f.order') key qo).access_strict
        $$ Hkey with ⟨Harray, HrestoreArray⟩
    icases array_acc (structFieldRef cursor.StaticField (go!"Words")
        (sliceIndexRef cursor.StaticField (sint.Z (W64 n)) f.order')) 0 qo 3
        key.Words' (leToU64 (key.Encoded'.take 8)) (by decide) (by rw [hwords]; rfl) $$ Harray with ⟨Hword, HrestoreWord⟩
    simp only [show sint.Z (W64 0) = 0 from rfl]
    wp_load
    wp_auto
    ihave Harray := HrestoreWord $$ %(leToU64 (key.Encoded'.take 8)) Hword
    have harray : array.mk 3 (key.Words'.arr.set (Int.toNat 0) (leToU64 (key.Encoded'.take 8))) = key.Words' := by
      simp only [hwords]; rfl
    isimp only [harray] at Harray
    ihave Hkey := HrestoreArray $$ Harray
    simp only [leToU64_first_byte]
    by_cases heq : key.Encoded'.headD (W8 0) = xs.headD (W8 0)
    · simp only [heq, ne_eq, not_true_eq_false, _root_.decide_false]
      wp_auto
      icases (cursor.StaticField.StaticField_access_load_Words
          (sliceIndexRef cursor.StaticField (sint.Z (W64 n)) f.order') key qo).access_strict
          $$ Hkey with ⟨Harray, HrestoreArray⟩
      icases array_acc (structFieldRef cursor.StaticField (go!"Words")
          (sliceIndexRef cursor.StaticField (sint.Z (W64 n)) f.order')) 0 qo 3
          key.Words' (leToU64 (key.Encoded'.take 8)) (by decide) (by rw [hwords]; rfl) $$ Harray with ⟨Hword, HrestoreWord⟩
      simp only [show sint.Z (W64 0) = 0 from rfl]
      wp_load
      wp_auto
      ihave Harray := HrestoreWord $$ %(leToU64 (key.Encoded'.take 8)) Hword
      have harray : array.mk 3 (key.Words'.arr.set (Int.toNat 0) (leToU64 (key.Encoded'.take 8))) = key.Words' := by
        simp only [hwords]; rfl
      isimp only [harray] at Harray
      ihave Hkey := HrestoreArray $$ Harray
      icases (cursor.StaticField.StaticField_access_load_Masks
          (sliceIndexRef cursor.StaticField (sint.Z (W64 n)) f.order') key qo).access_strict
          $$ Hkey with ⟨Harray, HrestoreArray⟩
      icases array_acc (structFieldRef cursor.StaticField (go!"Masks")
          (sliceIndexRef cursor.StaticField (sint.Z (W64 n)) f.order')) 0 qo 3
          key.Masks' (cacheMask (key.Encoded'.take 8).length) (by decide) (by rw [hmasks]; rfl) $$ Harray with ⟨Hword, HrestoreWord⟩
      simp only [show sint.Z (W64 0) = 0 from rfl]
      wp_load
      wp_auto
      ihave Harray := HrestoreWord $$ %(cacheMask (key.Encoded'.take 8).length) Hword
      have harray : array.mk 3 (key.Masks'.arr.set (Int.toNat 0) (cacheMask (key.Encoded'.take 8).length)) = key.Masks' := by
        simp only [hmasks]; rfl
      isimp only [harray] at Harray
      ihave Hkey := HrestoreArray $$ Harray
      icases (cursor.StaticField.StaticField_access_load_Words
          (sliceIndexRef cursor.StaticField (sint.Z (W64 n)) f.order') key qo).access_strict
          $$ Hkey with ⟨Harray, HrestoreArray⟩
      icases array_acc (structFieldRef cursor.StaticField (go!"Words")
          (sliceIndexRef cursor.StaticField (sint.Z (W64 n)) f.order')) 1 qo 3
          key.Words' (leToU64 ((key.Encoded'.drop 8).take 8)) (by decide) (by rw [hwords]; rfl) $$ Harray with ⟨Hword, HrestoreWord⟩
      simp only [show sint.Z (W64 1) = 1 from rfl]
      wp_load
      wp_auto
      ihave Harray := HrestoreWord $$ %(leToU64 ((key.Encoded'.drop 8).take 8)) Hword
      have harray : array.mk 3 (key.Words'.arr.set (Int.toNat 1) (leToU64 ((key.Encoded'.drop 8).take 8))) = key.Words' := by
        simp only [hwords]; rfl
      isimp only [harray] at Harray
      ihave Hkey := HrestoreArray $$ Harray
      icases (cursor.StaticField.StaticField_access_load_Masks
          (sliceIndexRef cursor.StaticField (sint.Z (W64 n)) f.order') key qo).access_strict
          $$ Hkey with ⟨Harray, HrestoreArray⟩
      icases array_acc (structFieldRef cursor.StaticField (go!"Masks")
          (sliceIndexRef cursor.StaticField (sint.Z (W64 n)) f.order')) 1 qo 3
          key.Masks' (cacheMask ((key.Encoded'.drop 8).take 8).length) (by decide) (by rw [hmasks]; rfl) $$ Harray with ⟨Hword, HrestoreWord⟩
      simp only [show sint.Z (W64 1) = 1 from rfl]
      wp_load
      wp_auto
      ihave Harray := HrestoreWord $$ %(cacheMask ((key.Encoded'.drop 8).take 8).length) Hword
      have harray : array.mk 3 (key.Masks'.arr.set (Int.toNat 1) (cacheMask ((key.Encoded'.drop 8).take 8).length)) = key.Masks' := by
        simp only [hmasks]; rfl
      isimp only [harray] at Harray
      ihave Hkey := HrestoreArray $$ Harray
      icases (cursor.StaticField.StaticField_access_load_Words
          (sliceIndexRef cursor.StaticField (sint.Z (W64 n)) f.order') key qo).access_strict
          $$ Hkey with ⟨Harray, HrestoreArray⟩
      icases array_acc (structFieldRef cursor.StaticField (go!"Words")
          (sliceIndexRef cursor.StaticField (sint.Z (W64 n)) f.order')) 2 qo 3
          key.Words' (leToU64 ((key.Encoded'.drop 16).take 8)) (by decide) (by rw [hwords]; rfl) $$ Harray with ⟨Hword, HrestoreWord⟩
      simp only [show sint.Z (W64 2) = 2 from rfl]
      wp_load
      wp_auto
      ihave Harray := HrestoreWord $$ %(leToU64 ((key.Encoded'.drop 16).take 8)) Hword
      have harray : array.mk 3 (key.Words'.arr.set (Int.toNat 2) (leToU64 ((key.Encoded'.drop 16).take 8))) = key.Words' := by
        simp only [hwords]; rfl
      isimp only [harray] at Harray
      ihave Hkey := HrestoreArray $$ Harray
      icases (cursor.StaticField.StaticField_access_load_Masks
          (sliceIndexRef cursor.StaticField (sint.Z (W64 n)) f.order') key qo).access_strict
          $$ Hkey with ⟨Harray, HrestoreArray⟩
      icases array_acc (structFieldRef cursor.StaticField (go!"Masks")
          (sliceIndexRef cursor.StaticField (sint.Z (W64 n)) f.order')) 2 qo 3
          key.Masks' (cacheMask ((key.Encoded'.drop 16).take 8).length) (by decide) (by rw [hmasks]; rfl) $$ Harray with ⟨Hword, HrestoreWord⟩
      simp only [show sint.Z (W64 2) = 2 from rfl]
      wp_load
      wp_auto
      ihave Harray := HrestoreWord $$ %(cacheMask ((key.Encoded'.drop 16).take 8).length) Hword
      have harray : array.mk 3 (key.Masks'.arr.set (Int.toNat 2) (cacheMask ((key.Encoded'.drop 16).take 8).length)) = key.Masks' := by
        simp only [hmasks]; rfl
      isimp only [harray] at Harray
      ihave Hkey := HrestoreArray $$ Harray
      have hcmp : ((leToU64 (xs.take 8) ^^^ leToU64 (key.Encoded'.take 8)) &&& cacheMask (key.Encoded'.take 8).length |||
          (leToU64 ((xs.drop 8).take 8) ^^^ leToU64 ((key.Encoded'.drop 8).take 8)) &&& cacheMask ((key.Encoded'.drop 8).take 8).length |||
          (leToU64 ((xs.drop 16).take 8) ^^^ leToU64 ((key.Encoded'.drop 16).take 8)) &&& cacheMask ((key.Encoded'.drop 16).take 8).length) = W64 0 ↔
          xs.take key.Encoded'.length = key.Encoded' := by
        simpa only [cacheCompare, cacheChunk, List.drop_zero] using cached_compare_prefix xs key.Encoded' hroom hkeylen
      by_cases hmatch : xs.take key.Encoded'.length = key.Encoded'
      · have hzero := hcmp.mpr hmatch
        simp only [hzero, _root_.decide_true]
        wp_auto
        icases (cursor.StaticField.StaticField_access_load_Short
            (sliceIndexRef cursor.StaticField (sint.Z (W64 n)) f.order') key qo).access_strict $$ Hkey with ⟨Hshort, HrestoreKey⟩
        wp_auto
        simp only [hshort]
        wp_auto
        ihave Hkey := HrestoreKey $$ Hshort
        ihave Horder := Hclose $$ Hkey
        isimp only [hset] at Horder
        wp_for_post
        ihave Hfields := Hrestore $$ Hfo
        iapply HΦ
        iframe Hst Hfields Horder Hbuf
        ipureintro
        have hnot : W64 n ≠ W64 (-1) := by word
        simp only [FieldFindResult, hnot, ite_false, hi, Int.toNat_natCast]
        exact ⟨Hbound.1, by omega, key, hlookup, hmatch⟩
      · have hnonzero := mt hcmp.mp hmatch
        simp only [hnonzero, _root_.decide_false]
        wp_auto
        ihave Horder := Hclose $$ Hkey
        isimp only [hset] at Horder
        wp_for_post
        have hinc : W64 n + W64 1 = W64 (n + 1) := by word
        isimp only [hinc] at Hi
        iframe
        iexists (n + 1); iframe
        ipureintro
        refine ⟨⟨by omega, by omega⟩, ?_⟩
        intro j other hj hjn hother
        by_cases hjlt : j < n
        · exact Hmiss j other hj hjlt hother
        · have hjn : j = n := by omega
          subst j
          have ho : other = key := Option.some.inj (hother.symm.trans hlookup)
          subst other; exact hmatch
    · simp only [heq, ne_eq, not_false_eq_true, _root_.decide_true]
      wp_auto
      ihave Horder := Hclose $$ Hkey
      isimp only [hset] at Horder
      by_cases hafter : uint.Z (xs.headD (W8 0)) < uint.Z (key.Encoded'.headD (W8 0))
      · simp only [hafter, _root_.decide_true]
        wp_auto
        wp_for_post
        ihave Hfields := Hrestore $$ Hfo
        iapply HΦ
        iframe Hst Hfields Horder Hbuf
        ipureintro
        simp only [FieldFindResult, ite_true]
        intro j other hj hother hprefix
        by_cases hjlt : j < n
        · exact Hmiss j other hj hjlt hother hprefix
        · have hord := hsorted n j key other (by omega) hlookup hother
          have hhead := prefix_head_eq xs other.Encoded' (hnonempty other (List.mem_of_getElem? hother)) hprefix
          rw [← hhead] at hord
          omega
      · simp only [hafter, _root_.decide_false]
        wp_auto
        wp_for_post
        have hinc : W64 n + W64 1 = W64 (n + 1) := by word
        isimp only [hinc] at Hi
        iframe
        iexists (n + 1); iframe
        ipureintro
        refine ⟨⟨by omega, by omega⟩, ?_⟩
        intro j other hj hjn hother
        by_cases hjlt : j < n
        · exact Hmiss j other hj hjlt hother
        · have hjn : j = n := by omega
          subst j
          have ho : other = key := Option.some.inj (hother.symm.trans hlookup)
          subst other
          exact fun hp => heq (prefix_head_eq xs key.Encoded' hkeyne hp).symm
  · have hcond : ¬ sint.Z (W64 n) < sint.Z f.order'.len := by word
    simp only [hcond, _root_.decide_false, hf, Bool.false_eq_true, ite_false, _root_.decide_true, ite_true]
    wp_auto
    ihave Hfields := Hrestore $$ Hfo
    iapply HΦ
    iframe Hst Hfields Horder Hbuf
    ipureintro
    simp only [FieldFindResult, ite_true]
    intro j key hj hlookup
    have hjbound := (List.getElem?_eq_some_iff.mp hlookup).choose
    exact Hmiss j key hj (by omega) hlookup

/-- Unified production search for arbitrary input lengths and cached short-key tables. -/
theorem wp_find_cached (st fields : Loc) (s : cursor.Struct) (f : cursor.Fields)
    (order : List cursor.StaticField) (buf : GoSlice) (xs : List w8) (qf qo dq : DFrac)
    (hfields : s.fields' = fields) (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ order.length)
    (hnonempty : ∀ key ∈ order, key.Encoded' ≠ [])
    (hsorted : FieldHeadsOrdered order) (hcache : CachedShortTable order) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       st ↦ s ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ buf ↦*{dq} xs }}
      (App (Val (st @!! go.PointerType cursor.Struct.ty @!! go!"find")) (Val #buf))
    {{ (result : w64), RET #result;
       st ↦ s ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ buf ↦*{dq} xs ∗
       ⌜FieldFindResult order xs (sint.Z s.next').toNat result⌝ }} := by
  iintro %Φ ⟨#Hpkg, Hst, Hfields, Horder, Hbuf⟩ HΦ
  by_cases hshort : xs.length < 24
  · wp_apply wp_find_short st fields s f order buf xs qf qo dq hfields hstart hshort hnonempty hsorted
      $$ [Hst Hfields Horder Hbuf] as %result ⟨Hst, Hfields, Horder, Hbuf, %Hresult⟩
    · iframe Hst Hfields Horder Hbuf
    iapply HΦ; iframe Hst Hfields Horder Hbuf
    ipureintro; exact Hresult
  · wp_apply wp_find_words st fields s f order buf xs qf qo dq hfields hstart (by omega) hnonempty hsorted hcache
      $$ [Hst Hfields Horder Hbuf] as %result ⟨Hst, Hfields, Horder, Hbuf, %Hresult⟩
    · iframe Hst Hfields Horder Hbuf
    iapply HΦ; iframe Hst Hfields Horder Hbuf
    ipureintro; exact Hresult

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.leToU64_first_byte
#print axioms Perennial.github_com.NethermindEth.juno.core.headerCachedShortTable
#print axioms Perennial.github_com.NethermindEth.juno.core.gasCachedShortTable

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_find_words

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_find_cached
