import ProofCheck.Utf8TableSpec
import ProofCheck.Utf8Reject

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

/-- A successful character step leaves exactly the original unconsumed suffix. -/
theorem utf8Next_suffix (xs next : List w8) (h : utf8Next xs = some next) :
    xs.drop (xs.length - next.length) = next := by
  unfold utf8Next at h
  repeat' split at h
  all_goals simp_all [Nat.add_assoc]

/-- One valid character preserves validity of the remainder, including malformed
    or incomplete bytes later in the string. -/
theorem utf8Next_preserves (xs next : List w8) (h : utf8Next xs = some next) :
    Utf8Valid xs ↔ Utf8Valid next := by
  cases xs with
  | nil => simp [utf8Next] at h
  | cons a rest =>
    have step := utf8Valid_step a rest
    constructor
    · intro valid
      obtain ⟨ys, hy, valid⟩ := step.mp valid
      rw [h] at hy
      cases hy
      exact valid
    · intro valid
      exact step.mpr ⟨next, h, valid⟩

section window
variable [GooseExts] [GoLocalContext] [GoGlobalContext] [GoSemanticsFunctions]

/-- The validator first clamps capacity to length, then repeatedly takes suffixes.
    Ownership remains attached to the original input, not this moving window. -/
def utf8Window (data : GoSlice) (off : w64) : GoSlice :=
  slice.fullSlice data w8 off data.len data.len
end window

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

theorem utf8Window_zero (data : GoSlice) :
    utf8Window data (W64 0) = slice.mk data.ptr data.len data.len := by
  simp only [utf8Window, slice.fullSlice, sliceIndexRef,
    show sint.Z (W64 0) = 0 from rfl, go.arrayIndexRef_0,
    show data.len - W64 0 = data.len by word]

theorem utf8Window_advance (data : GoSlice) (off step : w64)
    (hb : 0 ≤ sint.Z off + sint.Z step ∧ sint.Z off + sint.Z step < 2^63) :
    slice.slice (utf8Window data off) w8 step (data.len - off) =
      utf8Window data (off + step) := by
  simp only [utf8Window, slice.slice, slice.fullSlice, sliceIndexRef]
  rw [← go.arrayIndexRef_add]
  have hs : sint.Z (off + step) = sint.Z off + sint.Z step := by word
  rw [hs]
  congr 1 <;> word

theorem utf8Window_index (data : GoSlice) (off index : w64) :
    sliceIndexRef w8 (sint.Z index) (utf8Window data off) =
      sliceIndexRef w8 (sint.Z off + sint.Z index) data := by
  simp only [utf8Window, slice.fullSlice, sliceIndexRef]
  rw [go.arrayIndexRef_add]

/-- The actual full-slice expression preserves ownership of the original input. -/
theorem wp_utf8Window_start (data : GoSlice) (xs : List w8) (dq : DFrac) :
    {{ (data ↦*{dq} xs : IProp GF) }}
      (App (Val (GoInstruction (FullSlice (go.SliceType go.byte))))
        (Val (PairV (PairV (PairV #data #(W64 0)) #data.len) #data.len)))
    {{ RET #(utf8Window data (W64 0)); data ↦*{dq} xs }} := by
  iintro %Φ Hs HΦ
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  wp_auto
  have hb : 0 ≤ sint.Z (W64 0) ∧ sint.Z (W64 0) ≤ sint.Z data.len ∧
      sint.Z data.len ≤ sint.Z data.len ∧ sint.Z data.len ≤ sint.Z data.cap :=
    ⟨by decide, Hwf.1, Int.le_refl _, Hwf.2⟩
  rw [ite_eq_left_of_eq_true _ _ (eq_true hb)]
  isimp only [utf8Window] at HΦ
  wp_end

/-- Direct byte indexing in any remaining window reads the corresponding byte
    of the whole input and returns the complete fractional ownership unchanged. -/
theorem wp_utf8Window_load (data : GoSlice) (xs : List w8) (off index : Nat)
    (b : w8) (dq : DFrac) (lookup : xs[off + index]? = some b) :
    {{ (data ↦*{dq} xs : IProp GF) }}
      (App (Val (GoInstruction (GoLoad go.byte)))
        (App (Val (GoInstruction (IndexRef (go.SliceType go.byte))))
          (Val (PairV #(utf8Window data (W64 off)) #(W64 index)))))
    {{ RET #b; data ↦*{dq} xs }} := by
  iintro %Φ Hs HΦ
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hbound : off + index < xs.length := (List.getElem?_eq_some_iff.mp lookup).choose
  have hi : 0 ≤ sint.Z (W64 index) ∧ sint.Z (W64 index) < sint.Z (data.len - W64 off) := by word
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true hi), utf8Window_index]
  have hsum : sint.Z (W64 off) + sint.Z (W64 index) = (off + index : Nat) := by word
  rw [hsum]
  wp_apply wp_load_slice_index (t := go.byte) data ((off + index : Nat) : Int) xs dq b (by omega)
    $$ [Hs] with Hs
  · iframe Hs
    ipureintro
    simpa only [Int.toNat_natCast] using lookup
  wp_end

/-- The actual suffix-slice operation advances the window without consuming
    any ownership or exposing bytes beyond the original length. -/
theorem wp_utf8Window_advance (data : GoSlice) (xs : List w8) (off count : Nat)
    (dq : DFrac) (bound : off + count ≤ xs.length) :
    {{ (data ↦*{dq} xs : IProp GF) }}
      (App (Val (GoInstruction (Slice (go.SliceType go.byte))))
        (Val (PairV (PairV #(utf8Window data (W64 off)) #(W64 count)) #(data.len - W64 off))))
    {{ RET #(utf8Window data (W64 (off + count))); data ↦*{dq} xs }} := by
  iintro %Φ Hs HΦ
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hb : 0 ≤ sint.Z (W64 count) ∧ sint.Z (W64 count) ≤ sint.Z (data.len - W64 off) ∧
      sint.Z (data.len - W64 off) ≤ sint.Z (data.len - W64 off) := by word
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true hb)]
  rw [utf8Window_advance data (W64 off) (W64 count) (by word)]
  rw [show W64 off + W64 count = W64 (off + count) by word]
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.utf8Next_suffix
#print axioms Perennial.github_com.NethermindEth.juno.core.utf8Next_preserves
#print axioms Perennial.github_com.NethermindEth.juno.core.utf8Window_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.utf8Window_advance
#print axioms Perennial.github_com.NethermindEth.juno.core.utf8Window_index
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_utf8Window_start
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_utf8Window_load
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_utf8Window_advance
