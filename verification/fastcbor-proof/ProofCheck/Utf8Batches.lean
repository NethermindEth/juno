import ProofCheck.Utf8Scan

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

/-- An accepted eight-byte word mask advances the independent byte-level
    invariant. This is pure arithmetic, independent of the helper's WP. -/
theorem utf8Scan_word (xs : List w8) (off : Nat) (state : Utf8ScanState xs off)
    (bound : off + 8 ≤ xs.length)
    (mask : leToU64 ((xs.drop off).take 8) &&& asciiMask = W64 0) :
    Utf8ScanState xs (off + 8) := by
  apply utf8Scan_ascii xs off 8 state bound
  apply (asciiMask_word _ ?_).mp mask
  simp only [List.length_take, List.length_drop]
  omega

/-- The exact two-word OR expression used by unicode/utf8.Valid certifies the
    next sixteen bytes, preserving the validity of the entire remaining suffix. -/
theorem utf8Scan_twoWords (xs : List w8) (off : Nat) (state : Utf8ScanState xs off)
    (bound : off + 16 ≤ xs.length)
    (mask : (leToU64 ((xs.drop off).take 8) ||| leToU64 ((xs.drop (off + 8)).take 8)) &&& asciiMask = W64 0) :
    Utf8ScanState xs (off + 16) := by
  obtain ⟨first, second⟩ := (asciiMask_or _ _).mp mask
  have step := utf8Scan_word xs off state (by omega) first
  simpa only [Nat.add_assoc] using utf8Scan_word xs (off + 8) step (by omega) second

/-- The four-word expression used in the ASCII inner loop certifies exactly
    thirty-two bytes. A failed mask makes no claim about UTF-8 invalidity. -/
theorem utf8Scan_fourWords (xs : List w8) (off : Nat) (state : Utf8ScanState xs off)
    (bound : off + 32 ≤ xs.length)
    (mask : ((leToU64 ((xs.drop off).take 8) ||| leToU64 ((xs.drop (off + 8)).take 8)) |||
      (leToU64 ((xs.drop (off + 16)).take 8) ||| leToU64 ((xs.drop (off + 24)).take 8))) &&& asciiMask = W64 0) :
    Utf8ScanState xs (off + 32) := by
  obtain ⟨first, second⟩ := (asciiMask_or _ _).mp mask
  have step := utf8Scan_twoWords xs off state (by omega) first
  have second' : (leToU64 ((xs.drop (off + 16)).take 8) |||
      leToU64 ((xs.drop ((off + 16) + 8)).take 8)) &&& asciiMask = W64 0 := by
    simpa only [Nat.add_assoc] using second
  simpa only [Nat.add_assoc] using utf8Scan_twoWords xs (off + 16) step (by omega) second'

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.utf8Scan_word
#print axioms Perennial.github_com.NethermindEth.juno.core.utf8Scan_twoWords
#print axioms Perennial.github_com.NethermindEth.juno.core.utf8Scan_fourWords
