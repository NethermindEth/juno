import ProofCheck.Utf8Window

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

/-- Pure part of the validator loop invariant. All bytes consumed so far preserve
    whether the whole input is valid; the remaining suffix may still be malformed. -/
def Utf8ScanState (xs : List w8) (off : Nat) : Prop :=
  off ≤ xs.length ∧ (Utf8Valid xs ↔ Utf8Valid (xs.drop off))

theorem utf8Scan_start (xs : List w8) : Utf8ScanState xs 0 := by
  exact ⟨Nat.zero_le _, by simp⟩

/-- A successful character step advances the invariant by its exact consumed
    length, covering every valid UTF-8 character width. -/
theorem utf8Scan_character (xs next : List w8) (off : Nat)
    (state : Utf8ScanState xs off) (step : utf8Next (xs.drop off) = some next) :
    Utf8ScanState xs (off + ((xs.drop off).length - next.length)) := by
  have progress := utf8Next_progress _ _ step
  have suffix := utf8Next_suffix _ _ step
  have valid := utf8Next_preserves _ _ step
  have hd : xs.drop (off + ((xs.drop off).length - next.length)) = next := by
    rw [← List.drop_drop]
    exact suffix
  refine ⟨?_, ?_⟩
  · have hlen := List.length_drop (l := xs) (i := off)
    have hs := state.1
    omega
  · rw [hd]
    exact state.2.trans valid

/-- The same invariant supports every batched ASCII stride of the production
    fast path; its word-mask proofs must establish this per-byte premise. -/
theorem utf8Scan_ascii (xs : List w8) (off count : Nat)
    (state : Utf8ScanState xs off) (bound : off + count ≤ xs.length)
    (ascii : ASCIIBytes ((xs.drop off).take count)) :
    Utf8ScanState xs (off + count) := by
  refine ⟨bound, state.2.trans ?_⟩
  have h := utf8Valid_ascii_prefix ((xs.drop off).take count) ((xs.drop off).drop count) ascii
  rw [List.take_append_drop] at h
  simpa only [List.drop_drop] using h

/-- Exhausting the window certifies the whole original input. -/
theorem utf8Scan_done (xs : List w8) (off : Nat)
    (state : Utf8ScanState xs off) (done : xs.length ≤ off) : Utf8Valid xs := by
  apply state.2.mpr
  rw [List.drop_eq_nil_iff.mpr done]
  exact .empty

/-- Any rejected nonempty suffix refutes validity of the whole original input,
    including errors after arbitrary already-consumed valid characters. -/
theorem utf8Scan_reject (xs : List w8) (off : Nat)
    (state : Utf8ScanState xs off) (more : off < xs.length)
    (failure : utf8Next (xs.drop off) = none) : ¬ Utf8Valid xs := by
  intro valid
  have tailValid := state.2.mp valid
  cases ht : xs.drop off with
  | nil =>
    have hnil := List.drop_eq_nil_iff.mp ht
    omega
  | cons a rest =>
    rw [ht] at tailValid failure
    obtain ⟨next, hn, _⟩ := (utf8Valid_step a rest).mp tailValid
    rw [failure] at hn
    contradiction

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.utf8Scan_start
#print axioms Perennial.github_com.NethermindEth.juno.core.utf8Scan_character
#print axioms Perennial.github_com.NethermindEth.juno.core.utf8Scan_ascii
#print axioms Perennial.github_com.NethermindEth.juno.core.utf8Scan_done
#print axioms Perennial.github_com.NethermindEth.juno.core.utf8Scan_reject
