import ProofCheck.SegmentOwnership

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Unused zero records are separate from already decoded slots. -/
def SegmentStorage (data : GoSlice) (used count : Nat) : IProp GF :=
  arrayElems (sliceIndexRef SegmentLengths (used : Int) data)
    (List.replicate (count - used) (zero_val SegmentLengths)) (DFrac.own 1)

theorem segmentSlots_nil (data : GoSlice) : ⊢ (SegmentSlots data [] : IProp GF) := by
  unfold SegmentSlots
  iapply BigSepL.bigSepL_nil.2
  iempintro

theorem segmentSlots_snoc (data : GoSlice) (values : List SegmentValue) (value : SegmentValue) :
    SegmentSlots data values ∗ SegmentOwned (sliceIndexRef SegmentLengths (values.length : Int) data) value ⊢
      (SegmentSlots data (values ++ [value]) : IProp GF) := by
  unfold SegmentSlots
  iintro ⟨Hslots, Hvalue⟩
  iapply BigSepL.bigSepL_snoc.2
  iframe Hslots Hvalue

/-- Convert the fresh backing array into an inventory of unused zero slots. -/
theorem segmentStorage_init (data : GoSlice) (count : Nat) :
    data ↦* List.replicate count (zero_val SegmentLengths) ⊢ (SegmentStorage data 0 count : IProp GF) := by
  rw [ownSlice_unseal]
  unfold ownSliceDef SegmentStorage
  simp only [Nat.sub_zero, Int.natCast_zero, sliceIndexRef, go.arrayIndexRef_0]
  iintro (%Hnil | ⟨Harray, _⟩)
  · have hc : count = 0 := by
      have h := congrArg List.length Hnil.2
      simpa using h
    subst count
    simp only [List.replicate_zero, arrayElems]
    iapply BigSepL.bigSepL_nil.2
    iempintro
  · icases typedPointsto_split _ _ _ $$ Harray with Harray
    isimp only [TypedPointsto.typedPointstoDef] at Harray
    icases Harray with ⟨_, Hslots⟩
    iexact Hslots

/-- Transfer exactly the next shared slot, retaining all later unused slots. -/
theorem segmentStorage_take_cell (data : GoSlice) (used count : Nat) (more : used < count) :
    SegmentStorage data used count ⊢ (iprop(
      sliceIndexRef SegmentLengths (used : Int) data ↦ zero_val SegmentLengths ∗
      SegmentStorage data (used + 1) count) : IProp GF) := by
  have hcount : count - used = (count - (used + 1)) + 1 := by omega
  unfold SegmentStorage
  rw [hcount, List.replicate_succ]
  iintro Hslots
  icases (arrayElems_cons _ _ _ _).1 $$ Hslots with ⟨Hfirst, Hslots⟩
  isimp only [go.arrayIndexRef_0] at Hfirst
  have hnext : arrayIndexRef SegmentLengths 1 (sliceIndexRef SegmentLengths (used : Int) data) =
      sliceIndexRef SegmentLengths ((used + 1 : Nat) : Int) data := by
    simp only [sliceIndexRef, ← go.arrayIndexRef_add]
    congr 1
  isimp only [hnext] at Hslots
  iframe Hslots
  iexact Hfirst

theorem segmentStorage_take (data : GoSlice) (used count : Nat) (more : used < count) :
    SegmentStorage data used count ⊢ (iprop(
      SegmentOwned (sliceIndexRef SegmentLengths (used : Int) data) segmentZero ∗
      SegmentStorage data (used + 1) count) : IProp GF) := by
  have hcount : count - used = (count - (used + 1)) + 1 := by omega
  unfold SegmentStorage
  rw [hcount, List.replicate_succ]
  iintro Hslots
  icases (arrayElems_cons _ _ _ _).1 $$ Hslots with ⟨Hfirst, Hslots⟩
  isimp only [go.arrayIndexRef_0] at Hfirst
  have hnext : arrayIndexRef SegmentLengths 1 (sliceIndexRef SegmentLengths (used : Int) data) =
      sliceIndexRef SegmentLengths ((used + 1 : Nat) : Int) data := by
    simp only [sliceIndexRef, ← go.arrayIndexRef_add]
    congr 1
  isimp only [hnext] at Hslots
  iframe Hslots
  iapply segmentOwned_zero $$ Hfirst

theorem segmentArrayOwned_length (out : GoSlice) (values : List SegmentValue) :
    SegmentArrayOwned (GF := GF) out values ⊢ ⌜sint.Z out.len = values.length ∧ out.ptr ≠ null⌝ := by
  unfold SegmentArrayOwned
  iintro ⟨%Hsize, _⟩
  ipureintro; exact ⟨Hsize.2, Hsize.1⟩

theorem segmentArrayResult_null_iff (out : GoSlice) (values : Option (List SegmentValue)) :
    SegmentArrayResult (GF := GF) out values ⊢ ⌜out.ptr = null ↔ values = none⌝ := by
  cases values with
  | none => isimp only [SegmentArrayResult]; iintro %h; ipureintro; simp [h, slice.nil]
  | some values =>
    isimp only [SegmentArrayResult, SegmentArrayOwned]
    iintro ⟨%Hsize, _⟩
    ipureintro; simp [Hsize.1]

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentSlots_nil
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentSlots_snoc
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentStorage_init
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentStorage_take_cell
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentStorage_take
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentArrayOwned_length
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentArrayResult_null_iff
