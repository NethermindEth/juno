import ProofCheck.SliceAllocation

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

/-- Detach the first element's ownership from a nonempty reserve. This leaves
    disjoint ownership of all remaining elements, so callers may retain the slot. -/
theorem feltReserve_take_front (data : GoSlice) (head : GoArray w64 4) (tail : List (GoArray w64 4)) :
    (data ↦* (head :: tail)) ⊢ (data.ptr ↦ head ∗
      (slice.slice data (GoArray w64 4) (W64 1) data.len) ↦* tail : IProp GF) := by
  rw [ownSlice_unseal]
  unfold ownSliceDef
  iintro (%Hnil | ⟨Harray, %Hcap⟩)
  · simp at Hnil
  · ihave %Hnn := typedPointsto_not_null _ _ _ $$ Harray
    ihave %Hlen := array_len _ _ _ _ $$ Harray
    icases typedPointsto_split _ _ _ $$ Harray with Harray
    isimp only [TypedPointsto.typedPointstoDef] at Harray
    icases Harray with ⟨_, Hslots⟩
    icases (arrayElems_cons _ _ _ _).1 $$ Hslots with ⟨Hhead, Htail⟩
    isimp only [go.arrayIndexRef_0] at Hhead
    iframe Hhead
    iright
    simp only [slice.slice, sliceIndexRef, show sint.Z (W64 1) = 1 from rfl]
    isplitl [Htail]
    · rw [typedPointsto_unseal]
      unfold typedPointstoWrap
      simp only [TypedPointsto.typedPointstoDef]
      iframe Htail
      isplit
      · ipureintro
        simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hlen
        simp only [slice.slice]
        word
      · ipureintro
        exact fun h => Hnn (go.arrayIndexRef_null_inv (GoArray w64 4) 1 data.ptr h)
    · ipureintro
      simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hlen
      word

/-- The actual helper reuses a nonempty reserve for any want value. No allocation
    or integer addition is performed on this branch. -/
theorem wp_nextFelt_reuse (spare : Loc) (data : GoSlice) (head : GoArray w64 4)
    (tail : List (GoArray w64 4)) (want : w64) :
    {{ (spare ↦ data ∗ data ↦* (head :: tail) : IProp GF) }}
      (App (App (Val (@! cborNextFelt)) (Val #spare)) (Val #want))
    {{ RET #data.ptr; data.ptr ↦ head ∗
       spare ↦ (slice.slice data (GoArray w64 4) (W64 1) data.len) ∗
       (slice.slice data (GoArray w64 4) (W64 1) data.len) ↦* tail }} := by
  wp_start as ⟨Hspare, Hdata⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hdata
  have hnonzero : data.len ≠ W64 0 := by simp only [List.length_cons] at Hlen; word
  have hindex : 0 ≤ sint.Z (W64 0) ∧ sint.Z (W64 0) < sint.Z data.len := by
    simp only [List.length_cons] at Hlen; word
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hdata
  have hslice : 0 ≤ sint.Z (W64 1) ∧ sint.Z (W64 1) ≤ sint.Z data.len ∧
      sint.Z data.len ≤ sint.Z data.cap := by
    simp only [List.length_cons] at Hlen; word
  wp_auto
  simp only [hnonzero, _root_.decide_false]
  wp_auto
  rw [ite_eq_left hindex]
  wp_auto
  rw [ite_eq_left hslice]
  wp_auto
  ihave ⟨Hhead, Htail⟩ := feltReserve_take_front data head tail $$ Hdata
  simp only [sliceIndexRef, show sint.Z (W64 0) = 0 from rfl, go.arrayIndexRef_0]
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.feltReserve_take_front
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextFelt_reuse
