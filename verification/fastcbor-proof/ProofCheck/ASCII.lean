import ProofCheck.KeyReject

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 3200000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

/-- The byte-level ASCII contract, independent of the decoder's word-batching implementation. -/
def ASCIIBytes (xs : List w8) : Prop := ∀ b ∈ xs, uint.Z b < 128

def asciiMask : w64 := W64 9259542123273814144

theorem asciiBytes_append (xs ys : List w8) :
    ASCIIBytes (xs ++ ys) ↔ ASCIIBytes xs ∧ ASCIIBytes ys := by
  simp only [ASCIIBytes, List.mem_append, or_imp, forall_and]

theorem asciiMask_or (x y : w64) :
    (x ||| y) &&& asciiMask = W64 0 ↔ x &&& asciiMask = W64 0 ∧ y &&& asciiMask = W64 0 := by
  rw [BitVec.and_or_distrib_right]
  exact BitVec.or_eq_zero_iff

/-- Each of the eight byte positions contributes exactly its own high bit. -/
theorem asciiMask_byte_shift (b : w8) (i : Fin 8) :
    (W64 (uint.Z b) <<< W64 (8 * i.val)) &&& asciiMask = W64 0 ↔ uint.Z b < 128 := by
  have h : ∀ b : w8, ∀ i : Fin 8,
      (W64 (uint.Z b) <<< W64 (8 * i.val)) &&& asciiMask = W64 0 ↔ uint.Z b < 128 := by decide
  exact h b i

theorem asciiMask_byte (b : w8) :
    W64 (uint.Z b) &&& asciiMask = W64 0 ↔ uint.Z b < 128 := by
  simpa using asciiMask_byte_shift b ⟨0, by decide⟩

theorem asciiMask_word (bs : List w8) (hlen : bs.length = 8) :
    leToU64 bs &&& asciiMask = W64 0 ↔ ASCIIBytes bs := by
  rcases bs with _ | ⟨b0, bs⟩; · simp at hlen
  rcases bs with _ | ⟨b1, bs⟩; · simp at hlen
  rcases bs with _ | ⟨b2, bs⟩; · simp at hlen
  rcases bs with _ | ⟨b3, bs⟩; · simp at hlen
  rcases bs with _ | ⟨b4, bs⟩; · simp at hlen
  rcases bs with _ | ⟨b5, bs⟩; · simp at hlen
  rcases bs with _ | ⟨b6, bs⟩; · simp at hlen
  rcases bs with _ | ⟨b7, bs⟩; · simp at hlen
  have hnil : bs = [] := by simpa using hlen
  subst bs
  rw [encoding.binary.leToU64_8]
  have h1 : W64 (uint.Z b1) <<< W64 8 &&& asciiMask = W64 0 ↔ uint.Z b1 < 128 :=
    asciiMask_byte_shift b1 ⟨1, by decide⟩
  have h2 : W64 (uint.Z b2) <<< W64 16 &&& asciiMask = W64 0 ↔ uint.Z b2 < 128 :=
    asciiMask_byte_shift b2 ⟨2, by decide⟩
  have h3 : W64 (uint.Z b3) <<< W64 24 &&& asciiMask = W64 0 ↔ uint.Z b3 < 128 :=
    asciiMask_byte_shift b3 ⟨3, by decide⟩
  have h4 : W64 (uint.Z b4) <<< W64 32 &&& asciiMask = W64 0 ↔ uint.Z b4 < 128 :=
    asciiMask_byte_shift b4 ⟨4, by decide⟩
  have h5 : W64 (uint.Z b5) <<< W64 40 &&& asciiMask = W64 0 ↔ uint.Z b5 < 128 :=
    asciiMask_byte_shift b5 ⟨5, by decide⟩
  have h6 : W64 (uint.Z b6) <<< W64 48 &&& asciiMask = W64 0 ↔ uint.Z b6 < 128 :=
    asciiMask_byte_shift b6 ⟨6, by decide⟩
  have h7 : W64 (uint.Z b7) <<< W64 56 &&& asciiMask = W64 0 ↔ uint.Z b7 < 128 :=
    asciiMask_byte_shift b7 ⟨7, by decide⟩
  simp only [asciiMask_or, asciiMask_byte, h1, h2, h3, h4, h5, h6, h7,
    ASCIIBytes, List.mem_cons, List.not_mem_nil, or_false, forall_eq_or_imp, forall_eq, and_assoc]

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Word loads preserve ownership of the entire input, including an already scanned prefix. -/
theorem wp_littleEndian_offset (data : GoSlice) (xs bs rem : List w8) (off high : w64) (dq : DFrac)
    (hhigh : high = data.len) (hbound : 0 ≤ sint.Z off ∧ sint.Z off ≤ sint.Z data.len)
    (hbytes : subslice (sint.nat off) (sint.nat data.len) xs = bs ++ rem) (hlen : bs.length = 8) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.encoding.binary ∗ data ↦*{dq} xs }}
      (App (Val ((globalAddr encoding.binary.LittleEndian) @!!
        go.PointerType encoding.binary.littleEndian.ty @!! go!"Uint64"))
        (Val #(slice.slice data w8 off high)))
    {{ RET #(leToU64 bs); data ↦*{dq} xs }} := by
  subst high
  iintro %Φ ⟨#Hpkg, Hs⟩ HΦ
  icases (ownSlice_slice off data.len data dq xs
      ⟨hbound.1, hbound.2, Int.le_refl _⟩).1 $$ Hs with ⟨Hpre, Hbytes, Hpost⟩
  isimp only [hbytes] at Hbytes
  wp_apply encoding.binary.wp_LittleEndian_Uint64 (slice.slice data w8 off data.len)
    bs dq rem hlen $$ [Hbytes] with Hbytes
  · iframe Hbytes
  iapply HΦ
  iapply (ownSlice_slice off data.len data dq xs
      ⟨hbound.1, hbound.2, Int.le_refl _⟩).2
  rw [hbytes]
  iframe Hpre Hbytes Hpost

/-- The actual batched ASCII scan is equivalent to the independent per-byte predicate. -/
theorem wp_ascii (data : GoSlice) (xs : List w8) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗ data ↦*{dq} xs }}
      (App (Val (@! cursor.ascii)) (Val #data))
    {{ (ok : Bool), RET #ok; data ↦*{dq} xs ∗ ⌜ok = true ↔ ASCIIBytes xs⌝ }} := by
  wp_start as Hs
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  wp_auto
  ihave HI : (∃ (n : Nat) (bits : w64),
      "Hsvar" ∷ s_ptr ↦ slice.slice data w8 (W64 n) data.len ∗ "Hbits" ∷ bits_ptr ↦ bits ∗
      "%Hbound" ∷ ⌜n ≤ xs.length⌝ ∗
      "%Hmask" ∷ ⌜bits &&& asciiMask = W64 0 ↔ ASCIIBytes (xs.take n)⌝ : IProp GF) $$ [s bits]
  · iexists 0, (W64 0)
    have hz : slice.slice data w8 (W64 (0 : Nat)) data.len = data := (slice_slice_trivial data).symm
    rw [hz]
    iframe
    ipureintro
    exact ⟨Nat.zero_le _, by simp [asciiMask, ASCIIBytes]⟩
  wp_for HI
  by_cases hmore : n + 8 ≤ xs.length
  · have hcond : sint.Z (W64 8) ≤ sint.Z (data.len - W64 n) := by word
    simp only [hcond, _root_.decide_true, ite_true]
    wp_auto
    have hn : sint.nat (W64 n) = n := by word
    have htail : subslice (sint.nat (W64 n)) (sint.nat data.len) xs = xs.drop n := by
      rw [hn, ← Hlen.1]
      simp [subslice]
    have hsplit : subslice (sint.nat (W64 n)) (sint.nat data.len) xs =
        (xs.drop n).take 8 ++ (xs.drop n).drop 8 := by rw [htail, List.take_append_drop]
    have h8 : ((xs.drop n).take 8).length = 8 := by simp; omega
    wp_apply wp_littleEndian_offset data xs ((xs.drop n).take 8) ((xs.drop n).drop 8)
      (W64 n) data.len dq rfl ⟨by word, by word⟩ hsplit h8 $$ [Hs] with Hs
    · iframe Hs
    wp_for_post
    have hnl : (n : Int) ≤ sint.Z data.len := by word
    have hcap : sint.Z (data.len - W64 n) ≤ sint.Z (data.cap - W64 n) := by word
    rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, hcond, hcap⟩)]
    wp_auto
    have hslice : slice.slice (slice.slice data w8 (W64 n) data.len) w8 (W64 8)
        (data.len - W64 n) = slice.slice data w8 (W64 (n + 8)) data.len := by
      rw [slice_slice data (W64 n) data.len (W64 8) (data.len - W64 n) ⟨by word, by word⟩]
      congr 1 <;> word
    isimp only [hslice] at Hsvar
    iframe
    iexists (n + 8), (bits ||| leToU64 ((xs.drop n).take 8))
    iframe
    ipureintro
    refine ⟨hmore, ?_⟩
    rw [asciiMask_or, Hmask, asciiMask_word _ h8]
    rw [← asciiBytes_append]
    rw [List.take_drop]
    have hpre : (xs.take (n + 8)).take n = xs.take n := by simp [List.take_take]
    rw [← hpre, List.take_append_drop]
  · have hcond : ¬ sint.Z (W64 8) ≤ sint.Z (data.len - W64 n) := by word
    simp only [hcond, _root_.decide_false]
    simp
    wp_auto
    have hn : sint.nat (W64 n) = n := by word
    have htail : subslice (sint.nat (W64 n)) (sint.nat data.len) xs = xs.drop n := by
      rw [hn, ← Hlen.1]
      simp [subslice]
    icases (ownSlice_slice (W64 n) data.len data dq xs ⟨by word, by word, Int.le_refl _⟩).1
      $$ Hs with ⟨Hpre, Htail, Hpost⟩
    isimp only [htail] at Htail
    have htailLen : (xs.drop n).length = xs.length - n := List.length_drop
    ihave HI2 : (∃ (k : Nat) (bits2 : w64) (old : w8),
        "Hi" ∷ i_ptr ↦ W64 k ∗ "Hbits2" ∷ bits_ptr ↦ bits2 ∗ "Hc" ∷ c_ptr ↦ old ∗
        "%Hbound2" ∷ ⌜k ≤ (xs.drop n).length⌝ ∗
        "%Hmask2" ∷ ⌜bits2 &&& asciiMask = W64 0 ↔
          ASCIIBytes (xs.take n) ∧ ASCIIBytes ((xs.drop n).take k)⌝ : IProp GF) $$ [i Hbits c]
    · iexists 0, bits, (W8 0)
      iframe
      ipureintro
      refine ⟨Nat.zero_le _, ?_⟩
      simpa [ASCIIBytes] using Hmask
    rw [show BitVec.ofNat 64 n = W64 n by simp [W64]]
    wp_for HI2
    by_cases hmore2 : k < (xs.drop n).length
    · have hcond2 : sint.Z (W64 k) < sint.Z (data.len - W64 n) := by word
      simp only [hcond2, _root_.decide_true, ite_true]
      wp_auto
      rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by word, hcond2⟩)]
      have hk : sint.Z (W64 k) = k := by word
      rw [hk]
      let b := (xs.drop n)[k]'hmore2
      wp_apply wp_load_slice_index (slice.slice data w8 (W64 n) data.len) k
        (xs.drop n) dq b (by omega) $$ [Htail] with Htail
      · iframe Htail; ipureintro
        simp only [Int.toNat_natCast]
        exact List.getElem?_eq_getElem hmore2
      wp_for_post
      have hinc : W64 k + W64 1 = W64 (k + 1) := by word
      isimp only [hinc] at Hi
      iframe
      iexists (k + 1), (bits2 ||| W64 (uint.Z b)), b
      iframe
      ipureintro
      refine ⟨by omega, ?_⟩
      rw [asciiMask_or, Hmask2, asciiMask_byte, List.take_succ_eq_append_getElem hmore2,
        asciiBytes_append]
      simp only [ASCIIBytes, List.mem_cons, List.not_mem_nil, _root_.or_false, forall_eq, _root_.and_assoc, b]
    · have hcond2 : ¬ sint.Z (W64 k) < sint.Z (data.len - W64 n) := by word
      simp only [hcond2, _root_.decide_false, Bool.false_eq_true, ite_false]
      wp_auto
      ihave Hs : (data ↦*{dq} xs : IProp GF) $$ [Hpre Htail Hpost]
      · iapply (ownSlice_slice (W64 n) data.len data dq xs ⟨by word, by word, Int.le_refl _⟩).2
        rw [htail]
        iframe Hpre Htail Hpost
      iapply HΦ
      iframe Hs
      ipureintro
      have htake : (xs.drop n).take k = xs.drop n := List.take_of_length_le (by omega)
      simpa [htake, ← asciiBytes_append, asciiMask, W64] using Hmask2

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.asciiBytes_append
#print axioms Perennial.github_com.NethermindEth.juno.core.asciiMask_or
#print axioms Perennial.github_com.NethermindEth.juno.core.asciiMask_byte_shift
#print axioms Perennial.github_com.NethermindEth.juno.core.asciiMask_byte
#print axioms Perennial.github_com.NethermindEth.juno.core.asciiMask_word

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_littleEndian_offset
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_ascii
