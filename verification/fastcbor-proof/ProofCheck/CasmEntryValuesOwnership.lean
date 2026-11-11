import ProofCheck.CasmEntryPointOwnership

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
def CasmEntrySlots (data : GoSlice) (values : List CasmEntryPointValue) : IProp GF :=
  iprop([∗list] i ↦ value ∈ values, CasmEntryPointOwned (sliceIndexRef CasmEntryPoint (i : Int) data) value)

def CasmEntryValuesOwned (out : GoSlice) (values : List CasmEntryPointValue) : IProp GF := iprop(
  ⌜out.ptr ≠ null ∧ sint.Z out.len = values.length⌝ ∗ CasmEntrySlots out values)

def CasmEntryValuesResult (out : GoSlice) : Option (List CasmEntryPointValue) → IProp GF
  | none => iprop(⌜out = slice.nil⌝)
  | some values => CasmEntryValuesOwned out values

/-- Unused zero records are separate from already decoded slots. -/
def CasmEntryStorage (data : GoSlice) (used count : Nat) : IProp GF :=
  arrayElems (sliceIndexRef CasmEntryPoint (used : Int) data)
    (List.replicate (count - used) (zero_val CasmEntryPoint)) (DFrac.own 1)

theorem casmEntrySlots_nil (data : GoSlice) : ⊢ (CasmEntrySlots data [] : IProp GF) := by
  unfold CasmEntrySlots
  iapply BigSepL.bigSepL_nil.2
  iempintro

theorem casmEntrySlots_snoc (data : GoSlice) (values : List CasmEntryPointValue) (value : CasmEntryPointValue) :
    CasmEntrySlots data values ∗ CasmEntryPointOwned (sliceIndexRef CasmEntryPoint (values.length : Int) data) value ⊢
      (CasmEntrySlots data (values ++ [value]) : IProp GF) := by
  unfold CasmEntrySlots
  iintro ⟨Hslots, Hvalue⟩
  iapply BigSepL.bigSepL_snoc.2
  iframe Hslots Hvalue

/-- Convert the fresh backing array into an inventory of unused zero slots. -/
theorem casmEntryStorage_init (data : GoSlice) (count : Nat) :
    data ↦* List.replicate count (zero_val CasmEntryPoint) ⊢ (CasmEntryStorage data 0 count : IProp GF) := by
  rw [ownSlice_unseal]
  unfold ownSliceDef CasmEntryStorage
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
theorem casmEntryStorage_take_cell (data : GoSlice) (used count : Nat) (more : used < count) :
    CasmEntryStorage data used count ⊢ (iprop(
      sliceIndexRef CasmEntryPoint (used : Int) data ↦ zero_val CasmEntryPoint ∗
      CasmEntryStorage data (used + 1) count) : IProp GF) := by
  have hcount : count - used = (count - (used + 1)) + 1 := by omega
  unfold CasmEntryStorage
  rw [hcount, List.replicate_succ]
  iintro Hslots
  icases (arrayElems_cons _ _ _ _).1 $$ Hslots with ⟨Hfirst, Hslots⟩
  isimp only [go.arrayIndexRef_0] at Hfirst
  have hnext : arrayIndexRef CasmEntryPoint 1 (sliceIndexRef CasmEntryPoint (used : Int) data) =
      sliceIndexRef CasmEntryPoint ((used + 1 : Nat) : Int) data := by
    simp only [sliceIndexRef, ← go.arrayIndexRef_add]
    congr 1
  isimp only [hnext] at Hslots
  iframe Hslots
  iexact Hfirst

theorem casmEntryStorage_take (data : GoSlice) (used count : Nat) (more : used < count) :
    CasmEntryStorage data used count ⊢ (iprop(
      CasmEntryPointOwned (sliceIndexRef CasmEntryPoint (used : Int) data) casmEntryPointZero ∗
      CasmEntryStorage data (used + 1) count) : IProp GF) := by
  have hcount : count - used = (count - (used + 1)) + 1 := by omega
  unfold CasmEntryStorage
  rw [hcount, List.replicate_succ]
  iintro Hslots
  icases (arrayElems_cons _ _ _ _).1 $$ Hslots with ⟨Hfirst, Hslots⟩
  isimp only [go.arrayIndexRef_0] at Hfirst
  have hnext : arrayIndexRef CasmEntryPoint 1 (sliceIndexRef CasmEntryPoint (used : Int) data) =
      sliceIndexRef CasmEntryPoint ((used + 1 : Nat) : Int) data := by
    simp only [sliceIndexRef, ← go.arrayIndexRef_add]
    congr 1
  isimp only [hnext] at Hslots
  iframe Hslots
  iapply casmEntryPointOwned_zero $$ Hfirst

theorem casmEntryValuesOwned_length (out : GoSlice) (values : List CasmEntryPointValue) :
    CasmEntryValuesOwned (GF := GF) out values ⊢ ⌜sint.Z out.len = values.length ∧ out.ptr ≠ null⌝ := by
  unfold CasmEntryValuesOwned
  iintro ⟨%Hsize, _⟩
  ipureintro; exact ⟨Hsize.2, Hsize.1⟩

theorem casmEntryValuesResult_null_iff (out : GoSlice) (values : Option (List CasmEntryPointValue)) :
    CasmEntryValuesResult (GF := GF) out values ⊢ ⌜out.ptr = null ↔ values = none⌝ := by
  cases values with
  | none => isimp only [CasmEntryValuesResult]; iintro %h; ipureintro; simp [h, slice.nil]
  | some values =>
    isimp only [CasmEntryValuesResult, CasmEntryValuesOwned]
    iintro ⟨%Hsize, _⟩
    ipureintro; simp [Hsize.1]

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntrySlots_nil
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntrySlots_snoc
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryStorage_init
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryStorage_take_cell
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryStorage_take
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryValuesOwned_length
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryValuesResult_null_iff
