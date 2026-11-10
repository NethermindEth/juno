import ProofCheck.BigIntMagnitudeWord
import ProofCheck.Signatures

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Independently owned, normalized machine-word storage for a mathematical integer. -/
def BigMagnitudeOwned (p : Loc) (value : Int) : IProp GF := iprop(
  ∃ (s : GoSlice) (words : List w64) (negative : Bool),
    p ↦ math.big.Int'.mk negative s ∗ s ↦* words ∗
    ⌜normalizedPrefix words words.length ∧
      value = (if negative then -magnitudeValue words else magnitudeValue words) ∧
      (words = [] → negative = false)⌝)

/-- Normalization leaves a canonical retained slice, including the empty zero. -/
theorem bigNormalizedPrefix_take (words : List w64) (n : Int) (h : normalizedPrefix words n) :
    normalizedPrefix (words.take n.toNat) (words.take n.toNat).length := by
  unfold normalizedPrefix at h ⊢
  have hlen : (words.take n.toNat).length = n.toNat := by
    simp only [List.length_take]; omega
  refine ⟨by omega, by omega, ?_, ?_⟩
  · intro j hj hj'; omega
  · by_cases hz : n = 0
    · left; rw [hlen]; omega
    · have hpos : 0 < n := by omega
      rcases h.2.2.2 with hnzero | ⟨w, hw, hn⟩
      · omega
      · right
        refine ⟨w, ?_, hn⟩
        have hi : ((words.take n.toNat).length - 1 : Int).toNat = (n - 1).toNat := by omega
        rw [hi, List.getElem?_take]
        simp only [show (n - 1).toNat < n.toNat by omega, ite_true]
        exact hw

/-- The production byte-to-magnitude helper, including allocation, packing,
    carry propagation, normalization, and sign. -/
theorem wp_bigMagnitude (data : GoSlice) (xs : List w8) (dq : DFrac)
    (negative increment : Bool) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       data ↦*{dq} xs }}
      (App (App (App (Val (@! cursor.bigIntMagnitude)) (Val #data))
        (Val #negative)) (Val #increment))
    {{ (p : Loc), RET #p; data ↦*{dq} xs ∗
       BigMagnitudeOwned p ((if negative then -1 else 1) *
         (bigEndianMagnitude xs + if increment then 1 else 0)) }} := by
  wp_start as Hs
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  wp_auto
  have hlength : (xs.length : Int) = sint.Z data.len := by
    simp only [sint.nat, sint.Z] at Hlen Hwf ⊢; omega
  have hsmall : xs.length < 2 ^ 63 := by word
  have hdiv : BitVec.sdiv data.len (W64 8) = W64 ((xs.length / 8 : Nat) : Int) := by word
  have hrem : (BitVec.srem data.len (W64 8) = W64 0) ↔ xs.length % 8 = 0 := by
    have h : (BitVec.srem data.len (W64 8)).toInt = (xs.length % 8 : Nat) := by
      rw [BitVec.toInt_srem]
      change (sint.Z data.len).tmod 8 = _
      rw [← hlength, Int.ofNat_tmod]
      rfl
    constructor
    · intro hz; rw [hz] at h; change 0 = _ at h; omega
    · intro hz; apply BitVec.eq_of_toInt_eq; rw [h, hz]; rfl
  isimp only [hdiv] at count
  simp only [hrem]
  wp_join (count_ptr ↦ W64 (packBigEndianWords xs).length : IProp GF) with [count] as count
  · have hc : W64 ((xs.length / 8 : Nat) : Int) = W64 (packBigEndianWords xs).length := by
      rw [bigPackedWords_allocation_count]
      simp only [Hif, ite_true, Nat.add_zero]
    isimp only [hc] at count
    iframe
  · have hc : W64 ((xs.length / 8 : Nat) : Int) + W64 1 = W64 (packBigEndianWords xs).length := by
      rw [bigPackedWords_allocation_count]
      simp only [Hif, ite_false]
      word
    isimp only [hc] at count
    iframe
  wp_join (count_ptr ↦ W64 ((packBigEndianWords xs).length + if increment then 1 else 0) : IProp GF)
    with [count] as count
  · have hc : W64 (packBigEndianWords xs).length + W64 1 =
        W64 ((packBigEndianWords xs).length + 1 : Nat) := by word
    isimp only [hc] at count
    iframe
  let n := (packBigEndianWords xs).length + if increment then 1 else 0
  have hn : n < 2 ^ 63 := by
    have := bigPackedWords_allocation_bound xs hsmall
    dsimp only [n]; split <;> omega
  have hcount : W64 ((packBigEndianWords xs).length + if increment then 1 else 0) = W64 n := by
    dsimp only [n]; split <;> simp
  isimp only [hcount] at count
  wp_join (iprop(∃ words : GoSlice, words_ptr ↦ words ∗ words ↦* (List.replicate n (W64 0)) ∗ count_ptr ↦ W64 n) : IProp GF)
    with [words count] as ⟨%words, Hwords, Hout, count⟩
  · have hnzero : n = 0 := by word
    iexists slice.nil
    iframe
    rw [hnzero, List.replicate_zero]
    iapply ownSlice_nil
  · wp_apply wp_slice_make2 (V := w64) (W64 n) as %words ⟨Hout, -⟩
    · ipureintro; word
    wp_join_done
    iexists words
    have hnat : sint.nat (W64 n) = n := by word
    isimp only [hnat] at Hout
    iframe

  ihave %Houtlen := ownSlice_len _ _ _ $$ Hout
  ihave %Houtwf := ownSlice_wf _ _ _ $$ Hout
  have houtlen : sint.Z words.len = n := by
    simp only [List.length_replicate, sint.nat, sint.Z] at Houtlen Houtwf ⊢; omega
  have hstartslice : slice.slice data w8 (W64 0) (W64 xs.length) = data := by
    have he : W64 xs.length = data.len := by word
    rw [he]
    exact (slice_slice_trivial data).symm
  ihave HI : (∃ (pending : List w8) (done : List w64),
      "Hi" ∷ i_ptr ↦ W64 done.length ∗
      "Hdata" ∷ data_ptr ↦ slice.slice data w8 (W64 0) (W64 pending.length) ∗
      "Hout" ∷ words ↦* (done ++ List.replicate (n - done.length) (W64 0)) ∗
      "Hs" ∷ data ↦*{dq} xs ∗
      "%Hpending" ∷ ⌜pending = xs.take pending.length⌝ ∗
      "%Hplen" ∷ ⌜pending.length ≤ xs.length⌝ ∗
      "%Hpacked" ∷ ⌜packBigEndianWords xs = done ++ packBigEndianWords pending⌝ : IProp GF)
      $$ [i data Hout Hs]
  · iexists xs, []
    isimp only [List.length_nil, Nat.sub_zero, List.nil_append, hstartslice]
    iframe
    ipureintro
    exact ⟨List.take_length.symm, Nat.le_refl _, trivial⟩
  wp_for HI
  have hcountsplit : n = done.length + (packBigEndianWords pending).length + if increment then 1 else 0 := by
    dsimp only [n]; rw [Hpacked, List.length_append]
  have hdone : done.length ≤ n := by omega
  by_cases more : 8 ≤ pending.length
  · have hcond : sint.Z (W64 8) ≤ sint.Z ((W64 pending.length) - W64 0) := by word
    simp only [hcond, _root_.decide_true, ite_true]
    wp_auto
    let k := pending.length - 8
    have hk : k + 8 = pending.length := by dsimp only [k]; omega
    have hhigh : W64 pending.length - W64 0 = W64 pending.length := by word
    have hlow2 : W64 pending.length - W64 8 = W64 k := by dsimp only [k]; word
    simp only [hhigh, hlow2]
    have hb : 0 ≤ sint.Z (W64 k) ∧ sint.Z (W64 k) ≤ sint.Z (W64 pending.length) ∧
        sint.Z (W64 pending.length) ≤ sint.Z (data.cap - W64 0) := by word
    rw [ite_eq_left_of_eq_true _ _ (eq_true hb)]
    rw [slice_slice data (W64 0) (W64 pending.length) (W64 k) (W64 pending.length) (by word)]
    simp only [show W64 0 + W64 k = W64 k by word,
      show W64 0 + W64 pending.length = W64 pending.length by word]
    have hwindow : subslice (sint.nat (W64 k)) (sint.nat (W64 pending.length)) xs = pending.drop k := by
      have hkcast : sint.nat (W64 k) = k := by word
      have hlcast : sint.nat (W64 pending.length) = pending.length := by word
      rw [hkcast, hlcast]
      simp only [subslice, ← Hpending]
    have hbyteslen : (pending.drop k).length = 8 := by simp only [List.length_drop]; omega
    wp_apply wp_bigMagnitude_window_load data xs (pending.drop k) (W64 k) (W64 pending.length) dq
      (by word) hwindow hbyteslen $$ [Hs] with Hs
    · iframe Hs
    have hroom : done.length < n := by
      have hpacklen := bigPackedWords_allocation_count pending
      omega
    have hindex : 0 ≤ sint.Z (W64 done.length) ∧ sint.Z (W64 done.length) < sint.Z words.len := by word
    rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
    wp_apply wp_store_slice_index (t := math.big.Word.ty) words (sint.Z (W64 done.length))
      (done ++ List.replicate (n - done.length) (W64 0)) (leToU64 (pending.drop k).reverse)
      $$ [Hout] with Hout
    · iframe Hout; ipureintro
      simp only [List.length_append, List.length_replicate]; word

    have hbtrim : 0 ≤ sint.Z (W64 0) ∧ sint.Z (W64 0) ≤ sint.Z (W64 k) ∧
        sint.Z (W64 k) ≤ sint.Z (data.cap - W64 0) := by word
    rw [ite_eq_left_of_eq_true _ _ (eq_true hbtrim)]
    rw [slice_slice data (W64 0) (W64 pending.length) (W64 0) (W64 k) (by decide)]
    simp only [show W64 0 + W64 0 = W64 0 from rfl, show W64 0 + W64 k = W64 k by word]
    wp_auto
    let w := leToU64 (pending.drop k).reverse
    have hidx : (sint.Z (W64 done.length)).toNat = done.length := by word
    have hstore : (done ++ List.replicate (n - done.length) (W64 0)).set done.length w =
        (done ++ [w]) ++ List.replicate (n - (done ++ [w]).length) (W64 0) := by
      have hleft : n - done.length = (n - (done ++ [w]).length) + 1 := by simp; omega
      rw [hleft]
      exact initialized_prefix_step done _ (W64 0) w
    have hnextlen : (pending.take k).length = k := by simp only [List.length_take]; omega
    have hinc : W64 done.length + W64 1 = W64 (done ++ [w]).length := by simp only [List.length_append, List.length_singleton]; word
    dsimp only [w] at hstore
    wp_for_post
    isimp only [hidx, hstore] at Hout
    isimp only [hinc] at Hi
    iframe
    iexists (pending.take k), (done ++ [w])
    isimp only [hnextlen]
    iframe
    ipureintro
    refine ⟨?_, by omega, ?_⟩
    · rw [Hpending, List.take_take]
      congr 1
      omega
    · rw [Hpacked]
      have hp := bigPackedWords_suffix (pending.take k) (pending.drop k) hbyteslen
      rw [List.take_append_drop] at hp
      rw [hp]
      simp only [List.append_assoc, List.singleton_append, w]
  · have hcond : ¬ sint.Z (W64 8) ≤ sint.Z (W64 pending.length - W64 0) := by word
    simp only [hcond, _root_.decide_false, Bool.false_eq_true, ite_false]
    wp_auto

    have hnilcond : (W64 pending.length - W64 0 = W64 0) ↔ pending = [] := by
      constructor
      · intro h; apply List.eq_nil_of_length_eq_zero; word
      · intro h; subst pending; rfl
    simp only [hnilcond]
    wp_join (iprop(words_ptr ↦ words ∗
        words ↦* (packBigEndianWords xs ++ List.replicate (if increment then 1 else 0) (W64 0)) ∗
        data ↦*{dq} xs) : IProp GF)
      with [Hwords Hout Hdata Hi Hs] as ⟨Hwords, Hout, Hs⟩
    · have hp0 : packBigEndianWords [] = [] := by
        unfold packBigEndianWords
        rw [packLittleEndianWords]
        rfl
      have hc : n - done.length = if increment then 1 else 0 := by
        rw [hp0] at hcountsplit; simp only [List.length_nil] at hcountsplit; omega
      have hd : packBigEndianWords xs = done := by simpa only [hp0, List.append_nil] using Hpacked
      isimp only [hc] at Hout
      isimp only [← hd] at Hout
      iframe
    · ihave HJ : (∃ (j : Nat) (acc : w64) (oldb : w8),
          "Hj" ∷ i_ptr ↦ W64 j ∗ "Hacc" ∷ word_ptr ↦ acc ∗ "Hb" ∷ b_ptr ↦ oldb ∗
          "Hs" ∷ data ↦*{dq} xs ∗ "%Hjbound" ∷ ⌜j ≤ pending.length⌝ ∗
          "%Haccval" ∷ ⌜uint.Z acc = bigEndianMagnitude (pending.take j)⌝ : IProp GF)
          $$ [i word b Hs]
      · iexists 0, (W64 0), (W8 0); iframe; ipureintro
        exact ⟨Nat.zero_le _, rfl⟩
      wp_for HJ
      by_cases next : j < pending.length
      · have hjcond : sint.Z (W64 j) < sint.Z (W64 pending.length - W64 0) := by word
        simp only [hjcond, _root_.decide_true, ite_true]
        wp_auto
        have hidx : 0 ≤ sint.Z (W64 j) ∧ sint.Z (W64 j) < sint.Z (W64 pending.length - W64 0) := by word
        rw [ite_eq_left_of_eq_true _ _ (eq_true hidx)]
        have href : sliceIndexRef w8 (sint.Z (W64 j)) (slice.slice data w8 (W64 0) (W64 pending.length)) =
            sliceIndexRef w8 (sint.Z (W64 j)) data := by
          simp only [sliceIndexRef, slice.slice, show sint.Z (W64 0) = 0 from rfl, go.arrayIndexRef_0]
        rw [href]
        let byte := pending[j]'next
        have hlookup : xs[(sint.Z (W64 j)).toNat]? = some byte := by
          have hc : (sint.Z (W64 j)).toNat = j := by word
          rw [hc]
          have htake : pending[j]? = xs[j]? := by rw [Hpending, List.getElem?_take]; simp only [next, ite_true]
          rw [← htake]
          exact List.getElem?_eq_getElem next
        wp_apply wp_load_slice_index (t := go.byte) data (sint.Z (W64 j)) xs dq byte (by word)
          $$ [Hs] with Hs
        · iframe Hs; ipureintro; exact hlookup
        have haccbound : uint.Z acc < 2 ^ 56 := by
          rw [Haccval]
          have h := bigEndianMagnitude_bound (pending.take j)
          have hpNat : (2 : Nat) ^ (8 * (pending.take j).length) ≤ 2 ^ 56 :=
            Nat.pow_le_pow_right (by decide) (by simp only [List.length_take]; omega)
          have hp : (2 : Int) ^ (8 * (pending.take j).length) ≤ 2 ^ 56 := by exact_mod_cast hpNat
          omega
        have hnewval : uint.Z ((acc <<< W64 8) ||| W64 (uint.Z byte)) =
            bigEndianMagnitude (pending.take (j + 1)) := by
          rw [bigMagnitude_shift_byte acc byte haccbound, Haccval]
          rw [List.take_succ_eq_append_getElem next, bigEndianMagnitude_append_byte]
        have hjinc : W64 j + W64 1 = W64 ((j + 1 : Nat) : Int) := by word
        wp_for_post
        isimp only [hjinc] at Hj
        iframe
        iexists (j + 1), ((acc <<< W64 8) ||| W64 (uint.Z byte)), byte
        iframe
        ipureintro
        exact ⟨by omega, hnewval⟩
      · have hjcond : ¬ sint.Z (W64 j) < sint.Z (W64 pending.length - W64 0) := by word
        simp only [hjcond, _root_.decide_false, Bool.false_eq_true, ite_false]
        wp_auto
        have hjeq : j = pending.length := by omega
        have hvalue : uint.Z acc = bigEndianMagnitude pending := by
          simpa only [hjeq, List.take_length] using Haccval
        have hword : acc = leToU64 pending.reverse := by
          have := bigWord_bigEndian_value pending (by omega)
          word
        have hpartial : packBigEndianWords pending = [acc] := by
          rw [hword]
          exact bigPackedWords_partial pending Hif (by omega)
        have hc : n - done.length = 1 + if increment then 1 else 0 := by
          rw [hpartial, List.length_singleton] at hcountsplit
          omega
        have hindex : 0 ≤ sint.Z (W64 done.length) ∧ sint.Z (W64 done.length) < sint.Z words.len := by word
        rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
        wp_apply wp_store_slice_index (t := math.big.Word.ty) words (sint.Z (W64 done.length))
          (done ++ List.replicate (n - done.length) (W64 0)) acc $$ [Hout] with Hout
        · iframe Hout; ipureintro; simp only [List.length_append, List.length_replicate]; word
        wp_join_done
        have hidx : (sint.Z (W64 done.length)).toNat = done.length := by word
        have hstore : (done ++ List.replicate (n - done.length) (W64 0)).set done.length acc =
            packBigEndianWords xs ++ List.replicate (if increment then 1 else 0) (W64 0) := by
          rw [hc, Hpacked, hpartial, Nat.add_comm 1]
          exact initialized_prefix_step done _ (W64 0) acc
        isimp only [hidx, hstore] at Hout
        iframe

    let packed := packBigEndianWords xs
    let finalWords := if increment then incrementMagnitudeWords (packed ++ [W64 0]) else packed
    wp_join (iprop(words_ptr ↦ words ∗ words ↦* finalWords) : IProp GF)
      at (if: _ then _ else _) with [Hwords Hout] as ⟨Hwords, Hout⟩
    · cases increment with
      | false =>
        wp_auto
        wp_join_done
        isimp only [Bool.false_eq_true, ite_false, List.replicate_zero, List.append_nil] at Hout
        iframe
      | true =>
        wp_auto
        rename_i itemIndex
        irename i => Hrange
        irename i => Hitem
        isimp only [ite_true, List.replicate_one] at Hout
        let original := packed ++ [W64 0]
        have horiglen : original.length = n := by
          simp only [original, packed, n, ite_true, List.length_append, List.length_singleton]
        ihave HK : (∃ (k : Nat) (todo : List w64) (oldi : w64),
            "Hk" ∷ i_ptr ↦ W64 k ∗ "Hitem" ∷ itemIndex ↦ oldi ∗
            "Hout" ∷ words ↦* (List.replicate k (W64 0) ++ todo) ∗
            "%Hklen" ∷ ⌜k + todo.length = n⌝ ∗
            "%Hresult" ∷ ⌜incrementMagnitudeWords original =
              List.replicate k (W64 0) ++ incrementMagnitudeWords todo⌝ : IProp GF)
            $$ [Hrange Hitem Hout]
        · iexists 0, original, (W64 0)
          isimp only [List.replicate_zero, List.nil_append]
          iframe
          ipureintro
          exact ⟨by omega, trivial⟩
        wp_for HK
        by_cases moreWords : k < n
        · have hkcond : sint.Z (W64 k) < sint.Z words.len := by word
          simp only [hkcond, _root_.decide_true, ite_true]
          wp_auto
          cases todo with
          | nil => simp only [List.length_nil] at Hklen; omega
          | cons w todo =>
            have hindex : 0 ≤ sint.Z (W64 k) ∧ sint.Z (W64 k) < sint.Z words.len := by word
            have hkcast : (sint.Z (W64 k)).toNat = k := by word
            have hlookup : (List.replicate k (W64 0) ++ w :: todo)[(sint.Z (W64 k)).toNat]? = some w := by
              rw [hkcast]
              simp
            rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
            wp_apply wp_load_slice_index (t := math.big.Word.ty) words (sint.Z (W64 k))
              (List.replicate k (W64 0) ++ w :: todo) (DFrac.own 1) w hindex.1 $$ [Hout] with Hout
            · iframe Hout; ipureintro; exact hlookup
            rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
            wp_auto
            rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
            wp_apply wp_load_slice_index (t := math.big.Word.ty) words (sint.Z (W64 k))
              (List.replicate k (W64 0) ++ w :: todo) (DFrac.own 1) w hindex.1 $$ [Hout] with Hout
            · iframe Hout; ipureintro; exact hlookup
            wp_apply wp_store_slice_index (t := math.big.Word.ty) words (sint.Z (W64 k))
              (List.replicate k (W64 0) ++ w :: todo) (w + W64 1) $$ [Hout] with Hout
            · iframe Hout; ipureintro; simp only [List.length_append, List.length_replicate, List.length_cons]; word
            have hstore : (List.replicate k (W64 0) ++ w :: todo).set k (w + W64 1) =
                List.replicate k (W64 0) ++ (w + W64 1) :: todo := by
              simp
            isimp only [hkcast, hstore] at Hout
            rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
            wp_apply wp_load_slice_index (t := math.big.Word.ty) words (sint.Z (W64 k))
              (List.replicate k (W64 0) ++ (w + W64 1) :: todo) (DFrac.own 1) (w + W64 1) hindex.1 $$ [Hout] with Hout
            · iframe Hout; ipureintro; rw [hkcast]; simp
            by_cases carry : w + W64 1 = W64 0
            · simp only [carry, _root_.decide_true, Bool.not_true]
              wp_auto
              wp_for_post
              have hkinc : W64 k + W64 1 = W64 ((k + 1 : Nat) : Int) := by word
              isimp only [hkinc] at Hk
              have hzero (tail : List w64) : List.replicate k (W64 0) ++ W64 0 :: tail =
                  List.replicate (k + 1) (W64 0) ++ tail := by
                have helper (count : Nat) : List.replicate count (W64 0) ++ W64 0 :: tail =
                    List.replicate (count + 1) (W64 0) ++ tail := by
                  induction count with
                  | zero => rfl
                  | succ count ih => simp only [List.replicate_succ, List.cons_append] at ih ⊢; rw [ih]
                exact helper k
              isimp only [carry, hzero] at Hout
              iframe
              iexists (k + 1), todo, (W64 k)
              iframe
              ipureintro
              constructor
              · simp only [List.length_cons] at Hklen; omega
              · simpa only [incrementMagnitudeWords, carry, ite_true, hzero] using Hresult
            · simp only [carry, _root_.decide_false, Bool.not_false]
              wp_auto
              wp_for_post
              wp_join_done
              have hresult : finalWords = List.replicate k (W64 0) ++ (w + W64 1) :: todo := by
                simpa only [finalWords, ite_true, incrementMagnitudeWords, carry, ite_false] using Hresult
              isimp only [← hresult] at Hout
              iframe
        · have hkcond : ¬ sint.Z (W64 k) < sint.Z words.len := by word
          simp only [hkcond, _root_.decide_false, Bool.false_eq_true, ite_false]
          wp_join_done
          have htodo : todo = [] := List.eq_nil_of_length_eq_zero (by omega)
          have hresult : finalWords = List.replicate k (W64 0) ++ todo := by
            simpa only [finalWords, ite_true, htodo, incrementMagnitudeWords] using Hresult
          isimp only [← hresult] at Hout
          iframe

    have hfinalvalue : magnitudeValue finalWords = bigEndianMagnitude xs + if increment then 1 else 0 := by
      dsimp only [finalWords, packed]
      split
      · exact bigPackedWords_increment_value xs
      · simp only [bigPackedWords_bigEndian_value, Int.add_zero]
    wp_alloc p as Hp
    wp_apply wp_bigSetBits p (zero_val math.big.Int') words finalWords (DFrac.own 1)
      $$ [Hp Hout] as %used ⟨Hp, Hout, %Hnorm⟩
    · iframe Hp Hout
    let output := slice.slice words w64 (W64 0) used
    let sign := negative && decide (0 < sint.Z used)
    wp_join (iprop(out_ptr ↦ p ∗ p ↦ math.big.Int'.mk sign output) : IProp GF)
      at (if: _ then _ else _) with [out Hp] as ⟨out, Hp⟩
    · cases negative with
      | false => wp_auto; wp_join_done
      | true =>
        wp_auto
        wp_apply wp_bigNeg_self p (math.big.Int'.mk false output) $$ [Hp] with Hp
        · iframe Hp
        wp_join_done
        isimp only [output, slice.slice, show used - W64 0 = used by word,
          Bool.not_false, Bool.and_true] at Hp
        isimp only [output, sign, slice.slice, show used - W64 0 = used by word, Bool.true_and]
        iframe
    ihave %Hfinallen := ownSlice_len _ _ _ $$ Hout
    have hfinaln : finalWords.length = n := by
      simp only [sint.nat, sint.Z] at Hfinallen houtlen ⊢; omega
    have hb : 0 ≤ sint.Z (W64 0) ∧ sint.Z (W64 0) ≤ sint.Z used ∧ sint.Z used ≤ sint.Z words.len := by
      have hb := Hnorm.1
      unfold normalizedPrefix at hb
      word
    icases (ownSlice_slice (W64 0) used words (DFrac.own 1) finalWords hb).1 $$ Hout with ⟨-, Hout, -⟩
    have ht : subslice (sint.nat (W64 0)) (sint.nat used) finalWords = finalWords.take (sint.nat used) := by
      simp only [subslice, show sint.nat (W64 0) = 0 from rfl, List.drop_zero]
    isimp only [ht] at Hout
    iapply HΦ
    iframe Hs
    unfold BigMagnitudeOwned
    iexists output, (finalWords.take (sint.nat used)), sign
    iframe Hp Hout
    ipureintro
    refine ⟨bigNormalizedPrefix_take finalWords (sint.Z used) Hnorm.1, ?_, ?_⟩
    · rw [Hnorm.2, hfinalvalue]
      by_cases hpos : 0 < sint.Z used
      · cases negative <;> simp [sign, hpos]
      · have hz : sint.Z used = 0 := by have := Hnorm.1.1; omega
        have hm : bigEndianMagnitude xs + (if increment then 1 else 0) = 0 := by
          have h := Hnorm.2
          simp only [sint.nat, hz, Int.toNat_zero, List.take_zero, magnitudeValue] at h
          rw [hfinalvalue] at h
          omega
        rw [hm]
        simp
    · intro hempty
      have hlen := congrArg List.length hempty
      have hb := Hnorm.1
      unfold normalizedPrefix at hb
      simp only [List.length_take, List.length_nil, sint.nat] at hlen
      have hz : sint.Z used = 0 := by simp only [sint.Z] at hb ⊢; omega
      simp [sign, hz]

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.bigNormalizedPrefix_take
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bigMagnitude
