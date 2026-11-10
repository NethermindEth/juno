import ProofCheck.CacheEOF

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 3200000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- The search's early exit uses only the ordering of the first encoded byte. -/
def FieldHeadsOrdered (order : List cursor.StaticField) : Prop :=
  ∀ (i j : Nat) (ki kj : cursor.StaticField), i ≤ j → order[i]? = some ki → order[j]? = some kj →
    uint.Z (ki.Encoded'.headD (W8 0)) ≤ uint.Z (kj.Encoded'.headD (W8 0))

/-- Search success identifies an encoded prefix at or after start. Failure excludes
    every remaining entry, independently of the cached-word implementation. -/
def FieldFindResult (order : List cursor.StaticField) (xs : List w8) (start : Nat) (result : w64) : Prop :=
  if result = W64 (-1) then
    ∀ j key, start ≤ j → order[j]? = some key → xs.take key.Encoded'.length ≠ key.Encoded'
  else start ≤ (sint.Z result).toNat ∧ 0 ≤ sint.Z result ∧ ∃ key,
    order[(sint.Z result).toNat]? = some key ∧ xs.take key.Encoded'.length = key.Encoded'

theorem prefix_head_eq (xs key : List w8) (hne : key ≠ [])
    (hprefix : xs.take key.length = key) : xs.headD (W8 0) = key.headD (W8 0) := by
  cases key with
  | nil => contradiction
  | cons b bs =>
    cases xs with
    | nil => simp at hprefix
    | cons a rest => simpa using congrArg (fun l : List w8 => l.headD (W8 0)) hprefix

theorem prefix_length_le (xs key : List w8) (hprefix : xs.take key.length = key) : key.length ≤ xs.length := by
  have h := congrArg List.length hprefix
  simp only [List.length_take] at h
  omega

/-- A finite pairwise table check discharges the index-based search invariant. -/
theorem fieldHeadsOrdered_of_pairwise (order : List cursor.StaticField)
    (hs : order.Pairwise (fun a b => uint.Z (a.Encoded'.headD (W8 0)) ≤ uint.Z (b.Encoded'.headD (W8 0)))) :
    FieldHeadsOrdered order := by
  intro i j ki kj hij hki hkj
  obtain ⟨hi, hival⟩ := List.getElem?_eq_some_iff.mp hki
  obtain ⟨hj, hjval⟩ := List.getElem?_eq_some_iff.mp hkj
  by_cases heq : i = j
  · subst j
    have heq : ki = kj := Option.some.inj (hki.symm.trans hkj)
    rw [heq]; exact Int.le_refl _
  · have h := hs.rel_getElem_of_lt hi hj (by omega)
    simpa only [hival, hjval] using h

/-- The complete generated Header table has the first-byte ordering used by find. -/
theorem headerFieldHeads_ordered : FieldHeadsOrdered headerFieldKeys := by
  apply fieldHeadsOrdered_of_pairwise
  apply (List.pairwise_map (f := fun k : cursor.StaticField => uint.Z (k.Encoded'.headD (W8 0)))).mp
  change ([100, 102, 104, 104, 105, 106, 106, 106, 106, 108, 110, 111, 111, 112, 112] : List Int).Pairwise (· ≤ ·)
  decide

/-- The nested GasPrice table has the same ordering property. -/
theorem gasFieldHeads_ordered : FieldHeadsOrdered gasFieldKeys := by
  apply fieldHeadsOrdered_of_pairwise
  apply (List.pairwise_map (f := fun k : cursor.StaticField => uint.Z (k.Encoded'.headD (W8 0)))).mp
  change ([106, 106] : List Int).Pairwise (· ≤ ·)
  decide

theorem headerFieldEncodings_nonempty : ∀ key ∈ headerFieldKeys, key.Encoded' ≠ [] := by
  simp only [headerFieldKeys, List.mem_cons, List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq,
    false_implies, implies_true, _root_.and_true, forall_const]
  decide

theorem gasFieldEncodings_nonempty : ∀ key ∈ gasFieldKeys, key.Encoded' ≠ [] := by
  simp only [gasFieldKeys, List.mem_cons, List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq,
    false_implies, implies_true, _root_.and_true, forall_const]
  decide

/-- An absent encoded prefix forces the distinguished miss result. -/
theorem fieldFindResult_miss (order : List cursor.StaticField) (xs : List w8) (start : Nat) (result : w64)
    (hresult : FieldFindResult order xs start result)
    (hmiss : ∀ j key, start ≤ j → order[j]? = some key → xs.take key.Encoded'.length ≠ key.Encoded') :
    result = W64 (-1) := by
  by_cases hne : result = W64 (-1)
  · exact hne
  exfalso
  simp only [FieldFindResult, hne, ite_false] at hresult
  obtain ⟨hstart, _, key, hlookup, hprefix⟩ := hresult
  exact hmiss _ key hstart hlookup hprefix

/-- A unique remaining encoded prefix determines the exact search index. -/
theorem fieldFindResult_exact (order : List cursor.StaticField) (xs : List w8) (start target : Nat)
    (entry : cursor.StaticField) (result : w64) (hresult : FieldFindResult order xs start result)
    (hstart : start ≤ target) (hlookup : order[target]? = some entry)
    (hprefix : xs.take entry.Encoded'.length = entry.Encoded')
    (hunique : ∀ j other, start ≤ j → order[j]? = some other → xs.take other.Encoded'.length = other.Encoded' → j = target) :
    result = W64 target := by
  by_cases hneg : result = W64 (-1)
  · simp only [FieldFindResult, hneg, ite_true] at hresult
    exact False.elim (hresult target entry hstart hlookup hprefix)
  · simp only [FieldFindResult, hneg, ite_false] at hresult
    obtain ⟨hstart', hnonneg, other, hother, hmatch⟩ := hresult
    have hidx := hunique _ other hstart' hother hmatch
    word

/-- No two encoded field names may be prefixes of each other. This is checked
    directly on the generated tables, without assuming CBOR decoder correctness. -/
def FieldEncodingsApart (order : List cursor.StaticField) : Prop :=
  order.Pairwise (fun a b => a.Encoded'.take b.Encoded'.length ≠ b.Encoded' ∧
    b.Encoded'.take a.Encoded'.length ≠ a.Encoded')

theorem common_prefixes_comparable (xs a b : List w8)
    (ha : xs.take a.length = a) (hb : xs.take b.length = b) :
    a.take b.length = b ∨ b.take a.length = a := by
  by_cases hab : a.length ≤ b.length
  · right
    have h := congrArg (List.take a.length) hb
    rw [List.take_take, Nat.min_eq_left hab, ha] at h
    exact h.symm
  · left
    have h := congrArg (List.take b.length) ha
    rw [List.take_take, Nat.min_eq_left (by omega), hb] at h
    exact h.symm

/-- Prefix-free tables make a successful match's index unique for every input. -/
theorem fieldPrefix_unique (order : List cursor.StaticField) (xs : List w8)
    (hs : FieldEncodingsApart order) (i j : Nat) (ki kj : cursor.StaticField)
    (hki : order[i]? = some ki) (hkj : order[j]? = some kj)
    (hpi : xs.take ki.Encoded'.length = ki.Encoded')
    (hpj : xs.take kj.Encoded'.length = kj.Encoded') : i = j := by
  obtain ⟨hi, hival⟩ := List.getElem?_eq_some_iff.mp hki
  obtain ⟨hj, hjval⟩ := List.getElem?_eq_some_iff.mp hkj
  by_cases heq : i = j
  · exact heq
  exfalso
  by_cases hij : i < j
  · have h := hs.rel_getElem_of_lt hi hj hij
    simp only [hival, hjval] at h
    rcases common_prefixes_comparable xs ki.Encoded' kj.Encoded' hpi hpj with hp | hp
    · exact h.1 hp
    · exact h.2 hp
  · have h := hs.rel_getElem_of_lt hj hi (by omega)
    simp only [hival, hjval] at h
    rcases common_prefixes_comparable xs ki.Encoded' kj.Encoded' hpi hpj with hp | hp
    · exact h.2 hp
    · exact h.1 hp

theorem headerFieldEncodings_apart : FieldEncodingsApart headerFieldKeys := by
  simp only [FieldEncodingsApart, headerFieldKeys, List.pairwise_cons, List.Pairwise.nil,
    List.mem_cons, List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq,
    false_implies, implies_true, _root_.and_true, forall_const]
  repeat' apply And.intro
  all_goals decide

theorem gasFieldEncodings_apart : FieldEncodingsApart gasFieldKeys := by
  simp only [FieldEncodingsApart, gasFieldKeys, List.pairwise_cons, List.Pairwise.nil,
    List.mem_cons, List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq,
    false_implies, implies_true, _root_.and_true, forall_const]
  decide

variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Complete short-buffer search, including continue, ordered early exit, exhaustion,
    and successful matching. All table and input ownership may be fractional. -/
theorem wp_find_short (st fields : Loc) (s : cursor.Struct) (f : cursor.Fields)
    (order : List cursor.StaticField) (buf : GoSlice) (xs : List w8) (qf qo dq : DFrac)
    (hfields : s.fields' = fields) (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ order.length)
    (hshort : xs.length < 24) (hnonempty : ∀ key ∈ order, key.Encoded' ≠ [])
    (hsorted : FieldHeadsOrdered order) :
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
  have hsmall : ¬ sint.Z (W64 24) ≤ sint.Z buf.len := by word
  simp only [hsmall, _root_.decide_false]
  wp_auto
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
    simp only [hcond, _root_.decide_true]
    wp_auto
    by_cases hx : 0 < xs.length
    · have hxcond : sint.Z (W64 0) < sint.Z buf.len := by word
      simp only [hxcond, _root_.decide_true, ite_true]
      wp_auto
      have hi : sint.Z (W64 n) = n := by word
      rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by word, hcond⟩)]
      let key := order[n]'hmore
      have hlookup : order[n]? = some key := List.getElem?_eq_getElem hmore
      have hkeyne : key.Encoded' ≠ [] := hnonempty key (List.getElem_mem hmore)
      have hkeypos : 0 < key.Encoded'.length := List.length_pos_iff.mpr hkeyne
      icases ownSlice_elem_acc (sint.Z (W64 n)) key f.order' qo order (by word)
        (by simpa only [hi, Int.toNat_natCast] using hlookup) $$ Horder with ⟨Hkey, Hclose⟩
      wp_auto
      icases (cursor.StaticField.StaticField_access_load_Encoded
          (sliceIndexRef cursor.StaticField (sint.Z (W64 n)) f.order') key qo).access_strict $$ Hkey with ⟨Hencoded, HrestoreKey⟩
      wp_auto
      ihave Hkey := HrestoreKey $$ Hencoded
      ihave Horder := Hclose $$ Hkey
      have hset : order.set (sint.Z (W64 n)).toNat key = order := by
        simp only [hi, Int.toNat_natCast]; exact List.set_getElem_self hmore
      isimp only [hset] at Horder
      have hkhead : key.Encoded'[0]? = some (key.Encoded'.headD (W8 0)) := by
        cases h : key.Encoded' with
        | nil => contradiction
        | cons b bs => rfl
      rw [hkhead]
      wp_auto
      rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, hxcond⟩)]
      have hxhead : xs[(sint.Z (W64 0)).toNat]? = some (xs.headD (W8 0)) := by
        cases xs with
        | nil => simp at hx
        | cons b bs => rfl
      wp_apply wp_load_slice_index buf (sint.Z (W64 0)) xs dq (xs.headD (W8 0)) (by decide) $$ [Hbuf] with Hbuf
      · iframe Hbuf; ipureintro; exact hxhead
      by_cases heq : key.Encoded'.headD (W8 0) = xs.headD (W8 0)
      · simp only [heq, ne_eq, not_true_eq_false, _root_.decide_false]
        wp_auto
        wp_apply github_com.mit_pdos.perennial.goose.model.strings.wp_string_len with %Hkeylen
        by_cases hfits : key.Encoded'.length ≤ xs.length
        · have hfitscond : sint.Z (W64 key.Encoded'.length) ≤ sint.Z buf.len := by word
          simp only [hfitscond, _root_.decide_true]
          wp_auto
          wp_apply github_com.mit_pdos.perennial.goose.model.strings.wp_string_len with %-
          have hb : 0 ≤ sint.Z (W64 0) ∧ sint.Z (W64 0) ≤ sint.Z (W64 key.Encoded'.length) ∧
              sint.Z (W64 key.Encoded'.length) ≤ sint.Z buf.len := by word
          rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hb.1, hb.2.1, by word⟩)]
          icases (ownSlice_slice (W64 0) (W64 key.Encoded'.length) buf dq xs hb).1 $$ Hbuf with ⟨Hpre, Hbytes, Hpost⟩
          have hs : subslice (sint.nat (W64 0)) (sint.nat (W64 key.Encoded'.length)) xs = xs.take key.Encoded'.length := by
            have hz : sint.nat (W64 key.Encoded'.length) = key.Encoded'.length := by word
            rw [show sint.nat (W64 0) = 0 from rfl, hz]; rfl
          isimp only [hs] at Hbytes
          wp_apply wp_bytes_to_string (slice.slice buf w8 (W64 0) (W64 key.Encoded'.length))
              (xs.take key.Encoded'.length) dq $$ [Hbytes] with Hbytes
          · iframe Hbytes
          ihave Hbuf : (buf ↦*{dq} xs : IProp GF) $$ [Hpre Hbytes Hpost]
          · iapply (ownSlice_slice (W64 0) (W64 key.Encoded'.length) buf dq xs hb).2
            rw [hs]; iframe Hpre Hbytes Hpost
          by_cases hmatch : xs.take key.Encoded'.length = key.Encoded'
          · simp only [hmatch, _root_.decide_true, ite_true]
            wp_auto
            wp_for_post
            ihave Hfields := Hrestore $$ Hfo
            iapply HΦ
            iframe Hst Hfields Horder Hbuf
            ipureintro
            have hnot : W64 n ≠ W64 (-1) := by word
            simp only [FieldFindResult, hnot, ite_false, hi, Int.toNat_natCast]
            exact ⟨Hbound.1, by omega, key, hlookup, hmatch⟩
          · simp only [hmatch, _root_.decide_false, ite_false]
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
              subst other; exact hmatch
        · have hfitscond : ¬ sint.Z (W64 key.Encoded'.length) ≤ sint.Z buf.len := by word
          simp only [hfitscond, _root_.decide_false]
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
            exact fun h => hfits (prefix_length_le xs key.Encoded' h)
      · simp only [heq, ne_eq, not_false_eq_true, _root_.decide_true]
        wp_auto
        rw [hkhead]
        wp_auto
        rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, hxcond⟩)]
        wp_apply wp_load_slice_index buf (sint.Z (W64 0)) xs dq (xs.headD (W8 0)) (by decide) $$ [Hbuf] with Hbuf
        · iframe Hbuf; ipureintro; exact hxhead
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
    · have hxcond : ¬ sint.Z (W64 0) < sint.Z buf.len := by word
      simp only [hxcond, _root_.decide_false, hf, Bool.false_eq_true, ite_false, _root_.decide_true, ite_true]
      wp_auto
      ihave Hfields := Hrestore $$ Hfo
      iapply HΦ
      iframe Hst Hfields Horder Hbuf
      ipureintro
      simp only [FieldFindResult, ite_true]
      intro j key hj hlookup hp
      have hk := hnonempty key (List.mem_of_getElem? hlookup)
      have hl := prefix_length_le xs key.Encoded' hp
      have hkpos := List.length_pos_iff.mpr hk
      omega
  · have hcond : ¬ sint.Z (W64 n) < sint.Z f.order'.len := by word
    simp only [hcond, _root_.decide_false, hf, Bool.false_eq_true, ite_false, _root_.decide_true, ite_true]
    wp_auto
    simp only [hf, _root_.decide_false, _root_.decide_true, Bool.false_eq_true, ite_false, ite_true]
    wp_auto
    ihave Hfields := Hrestore $$ Hfo
    iapply HΦ
    iframe Hst Hfields Horder Hbuf
    ipureintro
    simp only [FieldFindResult, ite_true]
    intro j key hj hlookup
    have hjbound := (List.getElem?_eq_some_iff.mp hlookup).choose
    exact Hmiss j key hj (by omega) hlookup

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.prefix_head_eq
#print axioms Perennial.github_com.NethermindEth.juno.core.prefix_length_le
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_find_short

#print axioms Perennial.github_com.NethermindEth.juno.core.fieldHeadsOrdered_of_pairwise

#print axioms Perennial.github_com.NethermindEth.juno.core.headerFieldHeads_ordered

#print axioms Perennial.github_com.NethermindEth.juno.core.gasFieldHeads_ordered

#print axioms Perennial.github_com.NethermindEth.juno.core.headerFieldEncodings_nonempty

#print axioms Perennial.github_com.NethermindEth.juno.core.gasFieldEncodings_nonempty

#print axioms Perennial.github_com.NethermindEth.juno.core.fieldFindResult_miss

#print axioms Perennial.github_com.NethermindEth.juno.core.fieldFindResult_exact

#print axioms Perennial.github_com.NethermindEth.juno.core.common_prefixes_comparable

#print axioms Perennial.github_com.NethermindEth.juno.core.fieldPrefix_unique

#print axioms Perennial.github_com.NethermindEth.juno.core.headerFieldEncodings_apart

#print axioms Perennial.github_com.NethermindEth.juno.core.gasFieldEncodings_apart
