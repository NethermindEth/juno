import ProofCheck.SliceAppendZero
import ProofCheck.FeltReserveReuse

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

/-- Exhausted reserves allocate at least want zeroed slots, including rounded
    capacity, then transfer the first slot and retain disjoint ownership of the rest. -/
theorem wp_nextFelt_grow (spare : Loc) (old : GoSlice) (want : w64)
    (empty : old.len = W64 0) (positive : 0 < sint.Z want) :
    {{ (spare ↦ old : IProp GF) }}
      (App (App (Val (@! cborNextFelt)) (Val #spare)) (Val #want))
    {{ (data : GoSlice), RET #data.ptr;
       data.ptr ↦ (zero_val (GoArray w64 4)) ∗
       spare ↦ (slice.slice data (GoArray w64 4) (W64 1) data.len) ∗
       (slice.slice data (GoArray w64 4) (W64 1) data.len) ↦*
         List.replicate (sint.nat data.len - 1) (zero_val (GoArray w64 4)) ∗
       ⌜data.len = data.cap ∧ sint.Z want ≤ sint.Z data.len⌝ }} := by
  wp_start as Hspare
  wp_auto
  simp only [empty, _root_.decide_true]
  wp_auto
  wp_apply wp_slice_make2_nonnil (V := GoArray w64 4) want $$ [] as %src ⟨Hsrc, _, _⟩
  · ipureintro; omega
  ihave %Hlen := ownSlice_len _ _ _ $$ Hsrc
  have hsrclen : src.len = want := by simp only [List.length_replicate] at Hlen; word
  isimp only [← hsrclen] at Hsrc
  wp_apply wp_slice_append_nil_zero (V := GoArray w64 4) src (by rw [hsrclen]; exact positive)
    $$ [Hsrc] as %grown ⟨Hfull, _, %Hgrown⟩
  · iexact Hsrc
  have hgrown : 0 < sint.Z grown.cap := by rw [hsrclen] at Hgrown; omega
  rw [ite_eq_left ⟨by word, by word, by omega⟩]
  wp_auto
  have hfull : slice.slice grown (GoArray w64 4) (W64 0) grown.cap = {grown with len := grown.cap} := by
    simp only [slice.slice, sliceIndexRef, show sint.Z (W64 0) = 0 from rfl, go.arrayIndexRef_0]
    congr 1 <;> word
  rw [hfull]
  rw [ite_eq_left ⟨by word, by word⟩]
  wp_auto
  rw [ite_eq_left ⟨by word, by word, by word⟩]
  wp_auto
  have hcount : sint.nat grown.cap = (sint.nat grown.cap - 1) + 1 := by word
  have hzeros : List.replicate (sint.nat grown.cap) (zero_val (GoArray w64 4)) =
      zero_val (GoArray w64 4) :: List.replicate (sint.nat grown.cap - 1) (zero_val (GoArray w64 4)) := by
    simpa only [List.replicate_succ] using
      congrArg (fun n => List.replicate n (zero_val (GoArray w64 4))) hcount
  isimp only [hzeros] at Hfull
  ihave ⟨Hhead, Htail⟩ := feltReserve_take_front {grown with len := grown.cap}
    (zero_val (GoArray w64 4)) (List.replicate (sint.nat grown.cap - 1) (zero_val (GoArray w64 4))) $$ Hfull
  simp only [sliceIndexRef, show sint.Z (W64 0) = 0 from rfl, go.arrayIndexRef_0]
  iapply HΦ $$ %{grown with len := grown.cap}
  iframe Hhead Hspare Htail
  ipureintro
  exact ⟨rfl, by rw [hsrclen] at Hgrown; exact Hgrown.2⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextFelt_grow
