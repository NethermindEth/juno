import ProofCheck.BigInt

set_option maxHeartbeats 6400000
set_option maxRecDepth 4096

namespace Perennial
namespace github_com.NethermindEth.juno.core

/-- An unbounded natural magnitude specified by big-endian bytes. -/
def bigEndianMagnitude (bytes : List w8) : Int :=
  (LittleEndian.combine bytes.reverse : Int)

/-- Mathematical word grouping, not a replacement for the translated Go loop. -/
def packLittleEndianWords (bytes : List w8) : List w64 :=
  if bytes = [] then []
  else leToU64 (bytes.take 8) :: packLittleEndianWords (bytes.drop 8)
termination_by bytes.length
decreasing_by
  simp only [List.length_drop]
  have : 0 < bytes.length := List.length_pos_iff.mpr (by assumption)
  omega

def packBigEndianWords (bytes : List w8) : List w64 :=
  packLittleEndianWords bytes.reverse

/-- Fixed-length carry propagation; the caller must supply a spare high word. -/
def incrementMagnitudeWords : List w64 → List w64
  | [] => []
  | word :: words =>
      let next := word + W64 1
      next :: (if next = W64 0 then incrementMagnitudeWords words else words)

theorem bigEndianMagnitude_nonnegative (bytes : List w8) :
    0 ≤ bigEndianMagnitude bytes := by
  exact Int.natCast_nonneg _

theorem bigEndianMagnitude_bound (bytes : List w8) :
    bigEndianMagnitude bytes < (2 : Int) ^ (8 * bytes.length) := by
  have h := LittleEndian.combine_bound bytes.reverse
  simp only [List.length_reverse] at h
  unfold bigEndianMagnitude
  exact_mod_cast h

theorem bigEndianMagnitude_append_byte (bytes : List w8) (byte : w8) :
    bigEndianMagnitude (bytes ++ [byte]) = 256 * bigEndianMagnitude bytes + uint.Z byte := by
  simp only [bigEndianMagnitude, List.reverse_append, List.reverse_singleton,
    List.singleton_append, LittleEndian.combine, Int.natCast_add, Int.natCast_mul,
    uint.Z]
  omega

theorem bigCombine_append (left right : List w8) :
    LittleEndian.combine (left ++ right) =
      LittleEndian.combine left + 2 ^ (8 * left.length) * LittleEndian.combine right := by
  induction left with
  | nil => simp [LittleEndian.combine]
  | cons byte rest ih =>
    simp only [List.cons_append, LittleEndian.combine, List.length_cons, ih]
    rw [show 8 * (rest.length + 1) = 8 + 8 * rest.length by omega, Nat.pow_add]
    simp only [Nat.reducePow, Nat.mul_add, Nat.mul_assoc, Nat.add_assoc]

theorem bigWord_take_value (bytes : List w8) :
    uint.Z (leToU64 (bytes.take 8)) = (LittleEndian.combine (bytes.take 8) : Int) := by
  have hb := LittleEndian.combine_bound (bytes.take 8)
  have hp : 2 ^ (8 * (bytes.take 8).length) ≤ 2 ^ 64 :=
    Nat.pow_le_pow_right (by decide) (by simp only [List.length_take]; omega)
  have hfit : LittleEndian.combine (bytes.take 8) < 2 ^ 64 := by omega
  simp only [uint.Z, leToU64, leToU64Def, BitVec.toNat_ofNat, Nat.mod_eq_of_lt hfit]

theorem bigWord_bigEndian_value (bytes : List w8) (hlen : bytes.length ≤ 8) :
    uint.Z (leToU64 bytes.reverse) = bigEndianMagnitude bytes := by
  have h := bigWord_take_value bytes.reverse
  rw [List.take_of_length_le (by simpa only [List.length_reverse] using hlen)] at h
  exact h

theorem bigCombine_take_drop (bytes : List w8) :
    LittleEndian.combine bytes = LittleEndian.combine (bytes.take 8) +
      2 ^ 64 * LittleEndian.combine (bytes.drop 8) := by
  have h := bigCombine_append (bytes.take 8) (bytes.drop 8)
  rw [List.take_append_drop] at h
  by_cases hlen : 8 ≤ bytes.length
  · simpa only [List.length_take, Nat.min_eq_left hlen, Nat.reduceMul] using h
  · have hdrop : bytes.drop 8 = [] := List.drop_eq_nil_of_le (by omega)
    simp only [hdrop, LittleEndian.combine, Nat.mul_zero, Nat.add_zero] at h ⊢
    exact h

theorem bigPackedWords_value (bytes : List w8) :
    magnitudeValue (packLittleEndianWords bytes) = (LittleEndian.combine bytes : Int) := by
  rw [packLittleEndianWords]
  split
  · rename_i hempty
    subst bytes
    rfl
  · rename_i hnonempty
    rw [magnitudeValue, bigWord_take_value, bigPackedWords_value (bytes.drop 8)]
    have h := bigCombine_take_drop bytes
    exact_mod_cast h.symm
termination_by bytes.length
decreasing_by
  simp only [List.length_drop]
  have : 0 < bytes.length := List.length_pos_iff.mpr (by assumption)
  omega

theorem bigPackedWords_bigEndian_value (bytes : List w8) :
    magnitudeValue (packBigEndianWords bytes) = bigEndianMagnitude bytes :=
  bigPackedWords_value bytes.reverse

/-- Removing a full final byte group produces the next little-endian word. -/
theorem bigPackedWords_suffix (preBytes bytes : List w8) (hlen : bytes.length = 8) :
    packBigEndianWords (preBytes ++ bytes) =
      leToU64 bytes.reverse :: packBigEndianWords preBytes := by
  unfold packBigEndianWords
  rw [List.reverse_append, packLittleEndianWords]
  have hn : bytes.reverse ++ preBytes.reverse ≠ [] := by
    intro h
    have hlength := congrArg List.length h
    simp only [List.length_append, List.length_reverse, hlen, List.length_nil] at hlength
    omega
  rw [ite_eq_right hn]
  have ht : bytes.reverse.take 8 = bytes.reverse :=
    List.take_of_length_le (by simp only [List.length_reverse, hlen]; omega)
  have hd : bytes.reverse.drop 8 = [] :=
    List.drop_eq_nil_of_le (by simp only [List.length_reverse, hlen]; omega)
  simp only [List.take_append, List.drop_append, List.length_reverse, hlen,
    Nat.sub_self, List.take_zero, List.drop_zero, List.append_nil, ht, hd, List.nil_append]

/-- The final nonempty partial group occupies exactly one word. -/
theorem bigPackedWords_partial (bytes : List w8) (hnonempty : bytes ≠ [])
    (hlen : bytes.length < 8) :
    packBigEndianWords bytes = [leToU64 bytes.reverse] := by
  unfold packBigEndianWords
  rw [packLittleEndianWords, ite_eq_right (by simpa using hnonempty)]
  rw [List.take_of_length_le (by simp only [List.length_reverse]; omega),
    List.drop_eq_nil_of_le (by simp only [List.length_reverse]; omega)]
  rw [packLittleEndianWords]
  rfl

theorem bigPackedWords_length (bytes : List w8) :
    (packLittleEndianWords bytes).length = (bytes.length + 7) / 8 := by
  rw [packLittleEndianWords]
  split
  · rename_i hempty
    subst bytes
    rfl
  · rename_i hnonempty
    simp only [List.length_cons, bigPackedWords_length (bytes.drop 8), List.length_drop]
    have : 0 < bytes.length := List.length_pos_iff.mpr (by assumption)
    omega
termination_by bytes.length
decreasing_by
  simp only [List.length_drop]
  have : 0 < bytes.length := List.length_pos_iff.mpr (by assumption)
  omega

/-- The same count computed by len/8 followed by the partial-word increment. -/
theorem bigPackedWords_allocation_count (bytes : List w8) :
    (packBigEndianWords bytes).length =
      bytes.length / 8 + (if bytes.length % 8 = 0 then 0 else 1) := by
  simp only [packBigEndianWords, bigPackedWords_length, List.length_reverse]
  split <;> omega

/-- Even with the extra carry word, every valid Go input length fits a signed int. -/
theorem bigPackedWords_allocation_bound (bytes : List w8) (hlen : bytes.length < 2 ^ 63) :
    (packBigEndianWords bytes).length + 1 < 2 ^ 63 := by
  simp only [packBigEndianWords, bigPackedWords_length, List.length_reverse]
  omega

theorem bigMagnitude_nonnegative (words : List w64) : 0 ≤ magnitudeValue words := by
  induction words with
  | nil => exact Int.le_refl 0
  | cons word words ih =>
    have hw : 0 ≤ uint.Z word := by word
    simp only [magnitudeValue]
    omega

theorem bigMagnitude_append_zero (words : List w64) :
    magnitudeValue (words ++ [W64 0]) = magnitudeValue words := by
  induction words with
  | nil => rfl
  | cons word words ih => simp only [List.cons_append, magnitudeValue, ih]

theorem bigIncrement_length (words : List w64) :
    (incrementMagnitudeWords words).length = words.length := by
  induction words with
  | nil => rfl
  | cons word words ih =>
    simp only [incrementMagnitudeWords, List.length_cons]
    split <;> simp only [ih]

/-- A spare zero word absorbs even a carry through every input word. -/
theorem bigIncrement_spare_value (words : List w64) :
    magnitudeValue (incrementMagnitudeWords (words ++ [W64 0])) = magnitudeValue words + 1 := by
  induction words with
  | nil => rfl
  | cons word words ih =>
    simp only [List.cons_append, incrementMagnitudeWords]
    by_cases overflow : word + W64 1 = W64 0
    · have hw : uint.Z word = 2 ^ 64 - 1 := by word
      simp only [overflow, ite_true, magnitudeValue, ih]
      rw [hw]
      change 0 + 2 ^ 64 * (magnitudeValue words + 1) =
        (2 ^ 64 - 1) + 2 ^ 64 * magnitudeValue words + 1
      omega
    · have hw : uint.Z (word + W64 1) = uint.Z word + 1 := by word
      simp only [overflow, ite_false, magnitudeValue, bigMagnitude_append_zero, hw]
      omega

/-- The unbounded magnitude needed by CBOR tag 3, including all carry lengths. -/
theorem bigPackedWords_increment_value (bytes : List w8) :
    magnitudeValue (incrementMagnitudeWords (packBigEndianWords bytes ++ [W64 0])) =
      bigEndianMagnitude bytes + 1 := by
  rw [bigIncrement_spare_value, bigPackedWords_bigEndian_value]

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.bigEndianMagnitude_nonnegative
#print axioms Perennial.github_com.NethermindEth.juno.core.bigEndianMagnitude_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.bigEndianMagnitude_append_byte
#print axioms Perennial.github_com.NethermindEth.juno.core.bigCombine_append
#print axioms Perennial.github_com.NethermindEth.juno.core.bigWord_take_value
#print axioms Perennial.github_com.NethermindEth.juno.core.bigWord_bigEndian_value
#print axioms Perennial.github_com.NethermindEth.juno.core.bigCombine_take_drop
#print axioms Perennial.github_com.NethermindEth.juno.core.bigPackedWords_value
#print axioms Perennial.github_com.NethermindEth.juno.core.bigPackedWords_bigEndian_value
#print axioms Perennial.github_com.NethermindEth.juno.core.bigPackedWords_suffix
#print axioms Perennial.github_com.NethermindEth.juno.core.bigPackedWords_partial
#print axioms Perennial.github_com.NethermindEth.juno.core.bigPackedWords_length
#print axioms Perennial.github_com.NethermindEth.juno.core.bigPackedWords_allocation_count
#print axioms Perennial.github_com.NethermindEth.juno.core.bigPackedWords_allocation_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.bigMagnitude_nonnegative
#print axioms Perennial.github_com.NethermindEth.juno.core.bigMagnitude_append_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.bigIncrement_length
#print axioms Perennial.github_com.NethermindEth.juno.core.bigIncrement_spare_value
#print axioms Perennial.github_com.NethermindEth.juno.core.bigPackedWords_increment_value
