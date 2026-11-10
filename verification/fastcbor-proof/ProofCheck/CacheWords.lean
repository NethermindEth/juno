import ProofCheck.KeyFields

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 3200000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

/-- Keep precisely the low n bytes of a cached little-endian word. -/
def cacheMask (n : Nat) : w64 := BitVec.ofNat 64 (2 ^ (8 * n) - 1)

theorem masked_xor_zero (x y mask : w64) :
    (x ^^^ y) &&& mask = W64 0 ↔ x &&& mask = y &&& mask := by
  have h : (x ^^^ y) &&& mask = (x &&& mask) ^^^ (y &&& mask) := by
    apply BitVec.eq_of_getElem_eq
    intro i hi
    simp only [BitVec.getElem_and, BitVec.getElem_xor]
    cases x[i] <;> cases y[i] <;> cases mask[i] <;> rfl
  rw [h]
  exact BitVec.xor_eq_zero_iff

theorem cacheMask_toNat (n : Nat) (hn : n ≤ 8) :
    (cacheMask n).toNat = 2 ^ (8 * n) - 1 := by
  have hpow : 2 ^ (8 * n) ≤ 2 ^ 64 := Nat.pow_le_pow_right (by decide) (by omega)
  have hpos : 0 < 2 ^ (8 * n) := Nat.two_pow_pos _
  simp only [cacheMask, BitVec.toNat_ofNat]
  exact Nat.mod_eq_of_lt (by omega)

theorem masked_word_toNat (word : w64) (n : Nat) (hn : n ≤ 8) :
    (word &&& cacheMask n).toNat = word.toNat % 2 ^ (8 * n) := by
  rw [BitVec.toNat_and, cacheMask_toNat n hn, Nat.and_two_pow_sub_one_eq_mod]

theorem littleEndian_take_split (n m word : Nat) (hn : n ≤ m) :
    (LittleEndian.split m word).take n = LittleEndian.split n word := by
  induction n generalizing m word with
  | zero => rfl
  | succ n ih =>
    cases m with
    | zero => omega
    | succ m => simp only [LittleEndian.split, List.take_succ_cons]; rw [ih m _ (by omega)]

theorem littleEndian_combine_inj (xs ys : List w8) (hlen : xs.length = ys.length)
    (heq : LittleEndian.combine xs = LittleEndian.combine ys) : xs = ys := by
  have h := congrArg (LittleEndian.split xs.length) heq
  rw [LittleEndian.split_combine _ xs rfl, LittleEndian.split_combine _ ys hlen.symm] at h
  exact h

theorem leToU64_toNat (bs : List w8) (hlen : bs.length ≤ 8) :
    (leToU64 bs).toNat = LittleEndian.combine bs := by
  have hbound := LittleEndian.combine_bound bs
  have hpow : 2 ^ (8 * bs.length) ≤ 2 ^ 64 := Nat.pow_le_pow_right (by decide) (by omega)
  simp only [leToU64, leToU64Def, BitVec.toNat_ofNat]
  exact Nat.mod_eq_of_lt (by omega)

/-- A masked word comparison is exactly equality of the encoded byte prefix. -/
theorem cached_word_prefix (word : w64) (key : List w8) (hlen : key.length ≤ 8) :
    (word ^^^ leToU64 key) &&& cacheMask key.length = W64 0 ↔
      (u64Le word).take key.length = key := by
  have hlenWord := u64Le_length word
  have hbound := LittleEndian.combine_bound key
  have hmasked : (leToU64 key &&& cacheMask key.length).toNat = LittleEndian.combine key := by
    rw [masked_word_toNat _ _ hlen, leToU64_toNat key hlen, Nat.mod_eq_of_lt hbound]
  have hprefix : LittleEndian.combine ((u64Le word).take key.length) = word.toNat % 2 ^ (8 * key.length) := by
    simp only [u64Le, u64LeDef]
    rw [littleEndian_take_split _ 8 _ hlen, LittleEndian.combine_split, Nat.mul_comm key.length 8]
  rw [masked_xor_zero]
  constructor
  · intro heq
    apply littleEndian_combine_inj _ _ (by simp only [List.length_take, hlenWord]; omega)
    have h := congrArg BitVec.toNat heq
    rwa [masked_word_toNat _ _ hlen, hmasked, ← hprefix] at h
  · intro heq
    apply BitVec.eq_of_toNat_eq
    rw [masked_word_toNat _ _ hlen, hmasked, ← hprefix, heq]

/-- Prefix equality expressed by the bytes that the encoded key actually contains. -/
theorem take_prefix_iff (input key : List w8) :
    input.take key.length = key ↔ ∀ i, i < key.length → input[i]? = key[i]? := by
  constructor
  · intro heq i hi
    have h := congrArg (fun xs : List w8 => xs[i]?) heq
    rwa [List.getElem?_take_of_lt hi] at h
  · intro h
    apply List.ext_getElem?
    intro i
    by_cases hi : i < key.length
    · rw [List.getElem?_take_of_lt hi]
      exact h i hi
    · rw [List.getElem?_eq_none (by simp; omega), List.getElem?_eq_none (by omega)]

def cacheChunk (input key : List w8) (off : Nat) : w64 :=
  (leToU64 ((input.drop off).take 8) ^^^ leToU64 ((key.drop off).take 8)) &&&
    cacheMask ((key.drop off).take 8).length

/-- The comparison at one cache offset constrains exactly the key bytes at that offset. -/
theorem cached_chunk_prefix (input key : List w8) (off : Nat) (hinput : off + 8 ≤ input.length) :
    cacheChunk input key off = W64 0 ↔
      ∀ j, j < min 8 (key.length - off) → input[off + j]? = key[off + j]? := by
  have h8 : ((input.drop off).take 8).length = 8 := by simp; omega
  rw [cacheChunk, cached_word_prefix _ _ (by simp; omega), leToU64_le _ h8, take_prefix_iff]
  simp only [List.length_take, List.length_drop]
  constructor
  · intro h j hj
    have hj8 : j < 8 := by omega
    have h := h j hj
    simpa only [List.getElem?_take_of_lt hj8, List.getElem?_drop] using h
  · intro h j hj
    have hj8 : j < 8 := by omega
    simpa only [List.getElem?_take_of_lt hj8, List.getElem?_drop] using h j hj

/-- The three-word expression in NextField/find, before adding duplicate tracking. -/
def cacheCompare (input key : List w8) : w64 :=
  cacheChunk input key 0 ||| cacheChunk input key 8 ||| cacheChunk input key 16

/-- For short keys, no false positives or false negatives are hidden by the masks. -/
theorem cached_compare_prefix (input key : List w8) (hinput : 24 ≤ input.length) (hkey : key.length ≤ 24) :
    cacheCompare input key = W64 0 ↔ input.take key.length = key := by
  have hor (x y : w64) : x ||| y = W64 0 ↔ x = W64 0 ∧ y = W64 0 := BitVec.or_eq_zero_iff
  rw [cacheCompare, hor, hor,
    cached_chunk_prefix input key 0 (by omega), cached_chunk_prefix input key 8 (by omega),
    cached_chunk_prefix input key 16 (by omega), take_prefix_iff]
  constructor
  · rintro ⟨⟨h0, h1⟩, h2⟩ j hj
    by_cases hj0 : j < 8
    · simpa only [Nat.zero_add] using h0 j (by omega)
    · by_cases hj1 : j < 16
      · have hi : 8 + (j - 8) = j := by omega
        simpa only [hi] using h1 (j - 8) (by omega)
      · have hi : 16 + (j - 16) = j := by omega
        simpa only [hi] using h2 (j - 16) (by omega)
  · intro h
    exact ⟨⟨fun j hj => h (0 + j) (by omega), fun j hj => h (8 + j) (by omega)⟩,
      fun j hj => h (16 + j) (by omega)⟩

/-- Subtracting two ordered nonnegative indices gives the exact remaining length. -/
theorem cache_window_guard (pos len : w64) (hp : 0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z len) :
    24 ≤ sint.Z (len - pos) ↔ sint.Z pos + 24 ≤ sint.Z len := by word

/-- A successful remaining-length guard justifies the later end-index addition. -/
theorem cache_window_fits (pos len : w64) (hp : 0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z len)
    (hroom : 24 ≤ sint.Z (len - pos)) :
    sint.Z (pos + W64 24) = sint.Z pos + 24 ∧ sint.Z (pos + W64 24) ≤ sint.Z len := by word

/-- Regression witness for the old addition-before-check guard in the 64-bit model.
    It accepts an empty suffix near the signed limit; the remaining-length guard rejects it. -/
theorem cache_old_guard_overflow :
    sint.Z (W64 (2 ^ 63 - 1) + W64 24) ≤ sint.Z (W64 (2 ^ 63 - 1)) ∧
      ¬ 24 ≤ sint.Z (W64 (2 ^ 63 - 1) - W64 (2 ^ 63 - 1)) := by decide

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Independently check the actual generated word/mask arrays against the byte-level cache contract. -/
def cacheEntryWordsValid (key : cursor.StaticField) : Bool :=
  let chunks := [key.Encoded'.take 8, (key.Encoded'.drop 8).take 8, (key.Encoded'.drop 16).take 8]
  (key.Words'.arr == chunks.map leToU64) &&
    (key.Masks'.arr == chunks.map (fun bytes => cacheMask bytes.length))

theorem headerCacheWords_valid : headerFieldKeys.all cacheEntryWordsValid = true := by rfl

theorem gasCacheWords_valid : gasFieldKeys.all cacheEntryWordsValid = true := by rfl

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.masked_xor_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.cacheMask_toNat
#print axioms Perennial.github_com.NethermindEth.juno.core.masked_word_toNat
#print axioms Perennial.github_com.NethermindEth.juno.core.littleEndian_take_split
#print axioms Perennial.github_com.NethermindEth.juno.core.littleEndian_combine_inj
#print axioms Perennial.github_com.NethermindEth.juno.core.leToU64_toNat
#print axioms Perennial.github_com.NethermindEth.juno.core.cached_word_prefix

#print axioms Perennial.github_com.NethermindEth.juno.core.take_prefix_iff

#print axioms Perennial.github_com.NethermindEth.juno.core.cached_chunk_prefix

#print axioms Perennial.github_com.NethermindEth.juno.core.cached_compare_prefix

#print axioms Perennial.github_com.NethermindEth.juno.core.headerCacheWords_valid

#print axioms Perennial.github_com.NethermindEth.juno.core.gasCacheWords_valid

#print axioms Perennial.github_com.NethermindEth.juno.core.cache_window_guard

#print axioms Perennial.github_com.NethermindEth.juno.core.cache_window_fits

#print axioms Perennial.github_com.NethermindEth.juno.core.cache_old_guard_overflow
