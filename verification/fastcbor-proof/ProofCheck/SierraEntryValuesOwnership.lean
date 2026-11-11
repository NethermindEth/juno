import ProofCheck.SierraEntryPointOwnership

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

/-- Each slot owns the actual embedded record and its separately allocated children. -/
def SierraEntrySlots (data : GoSlice) (values : List SierraEntryPointValue) : IProp GF :=
  iprop([∗list] i ↦ value ∈ values, SierraEntryPointOwned (sliceIndexRef SierraEntryPoint (i : Int) data) value)

def SierraEntryValuesOwned (out : GoSlice) (values : List SierraEntryPointValue) : IProp GF := iprop(
  ⌜out.ptr ≠ null ∧ sint.Z out.len = values.length⌝ ∗ SierraEntrySlots out values)

def SierraEntryValuesResult (out : GoSlice) : Option (List SierraEntryPointValue) → IProp GF
  | none => iprop(⌜out = slice.nil⌝)
  | some values => SierraEntryValuesOwned out values

/-- Unused zero records are separate from already decoded slots. -/
def SierraEntryStorage (data : GoSlice) (used count : Nat) : IProp GF :=
  arrayElems (sliceIndexRef SierraEntryPoint (used : Int) data)
    (List.replicate (count - used) (zero_val SierraEntryPoint)) (DFrac.own 1)

theorem sierraEntrySlots_nil (data : GoSlice) : ⊢ (SierraEntrySlots data [] : IProp GF) := by
  unfold SierraEntrySlots
  iapply BigSepL.bigSepL_nil.2
  iempintro

theorem sierraEntrySlots_snoc (data : GoSlice) (values : List SierraEntryPointValue) (value : SierraEntryPointValue) :
    SierraEntrySlots data values ∗ SierraEntryPointOwned (sliceIndexRef SierraEntryPoint (values.length : Int) data) value ⊢
      (SierraEntrySlots data (values ++ [value]) : IProp GF) := by
  unfold SierraEntrySlots
  iintro ⟨Hslots, Hvalue⟩
  iapply BigSepL.bigSepL_snoc.2
  iframe Hslots Hvalue

/-- Convert the fresh backing array into an inventory of unused zero slots. -/
theorem sierraEntryStorage_init (data : GoSlice) (count : Nat) :
    data ↦* List.replicate count (zero_val SierraEntryPoint) ⊢ (SierraEntryStorage data 0 count : IProp GF) := by
  rw [ownSlice_unseal]
  unfold ownSliceDef SierraEntryStorage
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
theorem sierraEntryStorage_take_cell (data : GoSlice) (used count : Nat) (more : used < count) :
    SierraEntryStorage data used count ⊢ (iprop(
      sliceIndexRef SierraEntryPoint (used : Int) data ↦ zero_val SierraEntryPoint ∗
      SierraEntryStorage data (used + 1) count) : IProp GF) := by
  have hcount : count - used = (count - (used + 1)) + 1 := by omega
  unfold SierraEntryStorage
  rw [hcount, List.replicate_succ]
  iintro Hslots
  icases (arrayElems_cons _ _ _ _).1 $$ Hslots with ⟨Hfirst, Hslots⟩
  isimp only [go.arrayIndexRef_0] at Hfirst
  have hnext : arrayIndexRef SierraEntryPoint 1 (sliceIndexRef SierraEntryPoint (used : Int) data) =
      sliceIndexRef SierraEntryPoint ((used + 1 : Nat) : Int) data := by
    simp only [sliceIndexRef, ← go.arrayIndexRef_add]
    congr 1
  isimp only [hnext] at Hslots
  iframe Hslots
  iexact Hfirst

theorem sierraEntryStorage_take (data : GoSlice) (used count : Nat) (more : used < count) :
    SierraEntryStorage data used count ⊢ (iprop(
      SierraEntryPointOwned (sliceIndexRef SierraEntryPoint (used : Int) data) sierraEntryPointZero ∗
      SierraEntryStorage data (used + 1) count) : IProp GF) := by
  have hcount : count - used = (count - (used + 1)) + 1 := by omega
  unfold SierraEntryStorage
  rw [hcount, List.replicate_succ]
  iintro Hslots
  icases (arrayElems_cons _ _ _ _).1 $$ Hslots with ⟨Hfirst, Hslots⟩
  isimp only [go.arrayIndexRef_0] at Hfirst
  have hnext : arrayIndexRef SierraEntryPoint 1 (sliceIndexRef SierraEntryPoint (used : Int) data) =
      sliceIndexRef SierraEntryPoint ((used + 1 : Nat) : Int) data := by
    simp only [sliceIndexRef, ← go.arrayIndexRef_add]
    congr 1
  isimp only [hnext] at Hslots
  iframe Hslots
  iapply sierraEntryPointOwned_zero $$ Hfirst

theorem sierraEntryValuesOwned_length (out : GoSlice) (values : List SierraEntryPointValue) :
    SierraEntryValuesOwned (GF := GF) out values ⊢ ⌜sint.Z out.len = values.length ∧ out.ptr ≠ null⌝ := by
  unfold SierraEntryValuesOwned
  iintro ⟨%Hsize, _⟩
  ipureintro; exact ⟨Hsize.2, Hsize.1⟩

theorem sierraEntryValuesResult_null_iff (out : GoSlice) (values : Option (List SierraEntryPointValue)) :
    SierraEntryValuesResult (GF := GF) out values ⊢ ⌜out.ptr = null ↔ values = none⌝ := by
  cases values with
  | none => isimp only [SierraEntryValuesResult]; iintro %h; ipureintro; simp [h, slice.nil]
  | some values =>
    isimp only [SierraEntryValuesResult, SierraEntryValuesOwned]
    iintro ⟨%Hsize, _⟩
    ipureintro; simp [Hsize.1]

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntrySlots_nil
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntrySlots_snoc
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryStorage_init
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryStorage_take
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryValuesOwned_length
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryValuesResult_null_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryStorage_take_cell
