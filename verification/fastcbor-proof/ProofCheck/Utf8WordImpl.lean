import ProofCheck.Payload
import ProofCheck.Utf8WordContract
import ProofCheck.GeneratedProof.unicode.utf8
import Perennial.Proof.encoding.binary.Base

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 1600000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
open encoding.binary
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- The actual generic UTF-8 word helper, instantiated at []byte on the pinned 64-bit target. -/
theorem wp_utf8_word_prefix (data : GoSlice) (bs rem : List w8) (dq : DFrac) (hlen : bs.length = 8) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.unicode.utf8 ∗ data ↦*{dq} (bs ++ rem) }}
      (App (App (Val (GoInstruction (FuncResolve unicode.utf8.word [go.SliceType go.byte]))) (Val #())) (Val #data))
    {{ RET #(leToU64 bs); data ↦*{dq} (bs ++ rem) }} := by
  obtain ⟨w0, w1, w2, w3, w4, w5, w6, w7, rfl⟩ :
      ∃ w0 w1 w2 w3 w4 w5 w6 w7, bs = [w0, w1, w2, w3, w4, w5, w6, w7] := by
    match bs, hlen with
    | [w0, w1, w2, w3, w4, w5, w6, w7], _ => exact ⟨w0, w1, w2, w3, w4, w5, w6, w7, rfl⟩
  wp_start as Hs
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  wp_auto
  wp_func_call
  wp_call
  wp_auto
  (try wp_auto)
  idx_if
  wp_apply wp_load_slice_index data _ _ dq w0 (by decide) $$ [Hs] with Hs
  · iframe Hs; ipureintro; rfl
  (try wp_auto)
  idx_if
  wp_apply wp_load_slice_index data _ _ dq w1 (by decide) $$ [Hs] with Hs
  · iframe Hs; ipureintro; rfl
  (try wp_auto)
  idx_if
  wp_apply wp_load_slice_index data _ _ dq w2 (by decide) $$ [Hs] with Hs
  · iframe Hs; ipureintro; rfl
  (try wp_auto)
  idx_if
  wp_apply wp_load_slice_index data _ _ dq w3 (by decide) $$ [Hs] with Hs
  · iframe Hs; ipureintro; rfl
  (try wp_auto)
  idx_if
  wp_apply wp_load_slice_index data _ _ dq w4 (by decide) $$ [Hs] with Hs
  · iframe Hs; ipureintro; rfl
  (try wp_auto)
  idx_if
  wp_apply wp_load_slice_index data _ _ dq w5 (by decide) $$ [Hs] with Hs
  · iframe Hs; ipureintro; rfl
  (try wp_auto)
  idx_if
  wp_apply wp_load_slice_index data _ _ dq w6 (by decide) $$ [Hs] with Hs
  · iframe Hs; ipureintro; rfl
  (try wp_auto)
  idx_if
  wp_apply wp_load_slice_index data _ _ dq w7 (by decide) $$ [Hs] with Hs
  · iframe Hs; ipureintro; rfl
  rw [encoding.binary.leToU64_8]
  iapply HΦ $$ Hs


/-- The generic helper reads eight bytes at any valid validator offset, preserving
    fractional ownership of the entire original slice. -/
theorem utf8WordWindowContract_proved : Utf8WordWindowContract (GF := GF) := by
  intro data xs off dq bound
  have hlen : ((xs.drop off).take 8).length = 8 := by simp; omega
  obtain ⟨w0, w1, w2, w3, w4, w5, w6, w7, hbs⟩ :
      ∃ w0 w1 w2 w3 w4 w5 w6 w7,
        (xs.drop off).take 8 = [w0, w1, w2, w3, w4, w5, w6, w7] := by
    match (xs.drop off).take 8, hlen with
    | [w0, w1, w2, w3, w4, w5, w6, w7], _ => exact ⟨w0, w1, w2, w3, w4, w5, w6, w7, rfl⟩
  have lookup0 : xs[off + 0]? = some w0 := by
    have h := congrArg (fun ys : List w8 => ys[0]?) hbs
    simpa using h
  have lookup1 : xs[off + 1]? = some w1 := by
    have h := congrArg (fun ys : List w8 => ys[1]?) hbs
    simpa using h
  have lookup2 : xs[off + 2]? = some w2 := by
    have h := congrArg (fun ys : List w8 => ys[2]?) hbs
    simpa using h
  have lookup3 : xs[off + 3]? = some w3 := by
    have h := congrArg (fun ys : List w8 => ys[3]?) hbs
    simpa using h
  have lookup4 : xs[off + 4]? = some w4 := by
    have h := congrArg (fun ys : List w8 => ys[4]?) hbs
    simpa using h
  have lookup5 : xs[off + 5]? = some w5 := by
    have h := congrArg (fun ys : List w8 => ys[5]?) hbs
    simpa using h
  have lookup6 : xs[off + 6]? = some w6 := by
    have h := congrArg (fun ys : List w8 => ys[6]?) hbs
    simpa using h
  have lookup7 : xs[off + 7]? = some w7 := by
    have h := congrArg (fun ys : List w8 => ys[7]?) hbs
    simpa using h
  wp_start as Hs
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  wp_auto
  (try wp_auto)
  have hi0 : 0 ≤ sint.Z (W64 0) ∧ sint.Z (W64 0) < sint.Z (data.len - W64 off) := by word
  rw [ite_eq_left_of_eq_true _ _ (eq_true hi0), utf8Window_index]
  have hsum0 : sint.Z (W64 off) + sint.Z (W64 0) = (off + 0 : Nat) := by word
  rw [hsum0]
  wp_apply wp_load_slice_index (t := go.byte) data ((off + 0 : Nat) : Int) xs dq w0 (by omega) $$ [Hs] with Hs
  · iframe Hs; ipureintro; simpa only [Int.toNat_natCast] using lookup0
  (try wp_auto)
  have hi1 : 0 ≤ sint.Z (W64 1) ∧ sint.Z (W64 1) < sint.Z (data.len - W64 off) := by word
  rw [ite_eq_left_of_eq_true _ _ (eq_true hi1), utf8Window_index]
  have hsum1 : sint.Z (W64 off) + sint.Z (W64 1) = (off + 1 : Nat) := by word
  rw [hsum1]
  wp_apply wp_load_slice_index (t := go.byte) data ((off + 1 : Nat) : Int) xs dq w1 (by omega) $$ [Hs] with Hs
  · iframe Hs; ipureintro; simpa only [Int.toNat_natCast] using lookup1
  (try wp_auto)
  have hi2 : 0 ≤ sint.Z (W64 2) ∧ sint.Z (W64 2) < sint.Z (data.len - W64 off) := by word
  rw [ite_eq_left_of_eq_true _ _ (eq_true hi2), utf8Window_index]
  have hsum2 : sint.Z (W64 off) + sint.Z (W64 2) = (off + 2 : Nat) := by word
  rw [hsum2]
  wp_apply wp_load_slice_index (t := go.byte) data ((off + 2 : Nat) : Int) xs dq w2 (by omega) $$ [Hs] with Hs
  · iframe Hs; ipureintro; simpa only [Int.toNat_natCast] using lookup2
  (try wp_auto)
  have hi3 : 0 ≤ sint.Z (W64 3) ∧ sint.Z (W64 3) < sint.Z (data.len - W64 off) := by word
  rw [ite_eq_left_of_eq_true _ _ (eq_true hi3), utf8Window_index]
  have hsum3 : sint.Z (W64 off) + sint.Z (W64 3) = (off + 3 : Nat) := by word
  rw [hsum3]
  wp_apply wp_load_slice_index (t := go.byte) data ((off + 3 : Nat) : Int) xs dq w3 (by omega) $$ [Hs] with Hs
  · iframe Hs; ipureintro; simpa only [Int.toNat_natCast] using lookup3
  (try wp_auto)
  have hi4 : 0 ≤ sint.Z (W64 4) ∧ sint.Z (W64 4) < sint.Z (data.len - W64 off) := by word
  rw [ite_eq_left_of_eq_true _ _ (eq_true hi4), utf8Window_index]
  have hsum4 : sint.Z (W64 off) + sint.Z (W64 4) = (off + 4 : Nat) := by word
  rw [hsum4]
  wp_apply wp_load_slice_index (t := go.byte) data ((off + 4 : Nat) : Int) xs dq w4 (by omega) $$ [Hs] with Hs
  · iframe Hs; ipureintro; simpa only [Int.toNat_natCast] using lookup4
  (try wp_auto)
  have hi5 : 0 ≤ sint.Z (W64 5) ∧ sint.Z (W64 5) < sint.Z (data.len - W64 off) := by word
  rw [ite_eq_left_of_eq_true _ _ (eq_true hi5), utf8Window_index]
  have hsum5 : sint.Z (W64 off) + sint.Z (W64 5) = (off + 5 : Nat) := by word
  rw [hsum5]
  wp_apply wp_load_slice_index (t := go.byte) data ((off + 5 : Nat) : Int) xs dq w5 (by omega) $$ [Hs] with Hs
  · iframe Hs; ipureintro; simpa only [Int.toNat_natCast] using lookup5
  (try wp_auto)
  have hi6 : 0 ≤ sint.Z (W64 6) ∧ sint.Z (W64 6) < sint.Z (data.len - W64 off) := by word
  rw [ite_eq_left_of_eq_true _ _ (eq_true hi6), utf8Window_index]
  have hsum6 : sint.Z (W64 off) + sint.Z (W64 6) = (off + 6 : Nat) := by word
  rw [hsum6]
  wp_apply wp_load_slice_index (t := go.byte) data ((off + 6 : Nat) : Int) xs dq w6 (by omega) $$ [Hs] with Hs
  · iframe Hs; ipureintro; simpa only [Int.toNat_natCast] using lookup6
  (try wp_auto)
  have hi7 : 0 ≤ sint.Z (W64 7) ∧ sint.Z (W64 7) < sint.Z (data.len - W64 off) := by word
  rw [ite_eq_left_of_eq_true _ _ (eq_true hi7), utf8Window_index]
  have hsum7 : sint.Z (W64 off) + sint.Z (W64 7) = (off + 7 : Nat) := by word
  rw [hsum7]
  wp_apply wp_load_slice_index (t := go.byte) data ((off + 7 : Nat) : Int) xs dq w7 (by omega) $$ [Hs] with Hs
  · iframe Hs; ipureintro; simpa only [Int.toNat_natCast] using lookup7
  rw [hbs, encoding.binary.leToU64_8]
  iapply HΦ $$ Hs

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_utf8_word_prefix

#print axioms Perennial.github_com.NethermindEth.juno.core.utf8WordWindowContract_proved
