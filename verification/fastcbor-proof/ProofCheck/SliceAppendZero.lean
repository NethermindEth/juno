import ProofCheck.SliceFullZero

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

/-- Appending a nonempty zero slice to nil allocates a fully zeroed capacity,
    not merely a zeroed visible prefix. No capacity-rounding formula is assumed. -/
theorem wp_slice_append_nil_zero {V : Type} [ZeroVal V] [TypedPointsto (GF := GF) V]
    {st t : go.GoType} [st ↓u go.SliceType t] [IntoValTyped (GF := GF) V t]
    (src : GoSlice) (hp : 0 < sint.Z src.len) :
    {{ (src ↦* List.replicate (sint.nat src.len) (zero_val V) : IProp GF) }}
      (App (App (Val #(functions go.append [st])) (Val #slice.nil)) (Val #src))
    {{ (out : GoSlice), RET #out;
        {out with len := out.cap} ↦* List.replicate (sint.nat out.cap) (zero_val V) ∗
        src ↦* List.replicate (sint.nat src.len) (zero_val V) ∗
        ⌜out.len = src.len ∧ sint.Z src.len ≤ sint.Z out.cap⌝ }} := by
  wp_start_folded as Hsrc
  wp_func_call
  wp_call
  wp_apply wp_sumAssumeNoOverflowSigned with %Hoverflow
  wp_if_destruct
  · exfalso; word
  · wp_apply wp_new_cap with %cap %Hcap_ge
    have hsum : (0 : w64) + src.len = src.len := by word
    simp only [hsum]
    wp_apply wp_slice_make3_full_zero (st := st) (V := V) src.len cap ⟨hp, by word⟩
      with %out ⟨Hfull, %Hout⟩
    wp_apply wp_slice_copy_of_len_zero (V := V) out slice.nil rfl
    rw [ite_eq_left ⟨by word, by word, by rw [Hout.2]; word⟩]
    have htrivial : slice.slice out V 0 src.len = out := by
      rw [← Hout.1]
      exact (slice_slice_trivial out).symm
    rw [htrivial]
    have hsplit : 0 ≤ sint.Z out.len ∧ sint.Z out.len ≤ sint.Z ({out with len := out.cap} : GoSlice).len := by
      simp only; rw [Hout.1, Hout.2]; word
    have hprefix : slice.slice {out with len := out.cap} V (W64 0) out.len = out := by
      simpa only [slice.slice, sliceIndexRef] using (slice_slice_trivial (V := V) out).symm
    have htake : (List.replicate (sint.nat cap) (zero_val V)).take (sint.nat out.len) =
        List.replicate (sint.nat src.len) (zero_val V) := by
      rw [List.take_replicate, Hout.1, Nat.min_eq_left (by word)]
    icases (ownSlice_split_all out.len {out with len := out.cap} (DFrac.own 1)
      (List.replicate (sint.nat cap) (zero_val V)) hsplit).1 $$ Hfull with ⟨Hprefix, Htail⟩
    isimp only [hprefix, htake] at Hprefix
    wp_apply wp_slice_copy out _ src _ (DFrac.own 1) $$ [Hprefix Hsrc] as %n ⟨_, Hprefix, Hsrc⟩
    · iframe Hprefix Hsrc
    isimp only [List.length_replicate, List.take_replicate, Nat.min_self,
      List.drop_replicate, Nat.sub_self, List.replicate_zero, List.append_nil] at Hprefix
    ihave Hfull : ({out with len := out.cap} ↦* List.replicate (sint.nat cap) (zero_val V) : IProp GF)
      $$ [Hprefix Htail]
    · iapply (ownSlice_split_all out.len {out with len := out.cap} (DFrac.own 1)
        (List.replicate (sint.nat cap) (zero_val V)) hsplit).2
      isimp only [hprefix, htake]
      iframe Hprefix Htail
    isimp only [← Hout.2] at Hfull
    iapply HΦ $$ %out
    iframe Hfull Hsrc
    ipureintro; exact ⟨Hout.1, by word⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_slice_append_nil_zero
