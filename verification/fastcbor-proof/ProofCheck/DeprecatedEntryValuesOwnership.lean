import ProofCheck.DeprecatedEntryPointOwnership

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
def DeprecatedEntrySlots (data : GoSlice) (values : List DeprecatedEntryPointValue) : IProp GF :=
  iprop([∗list] i ↦ value ∈ values, DeprecatedEntryPointOwned (sliceIndexRef DeprecatedEntryPoint (i : Int) data) value)

def DeprecatedEntryValuesOwned (out : GoSlice) (values : List DeprecatedEntryPointValue) : IProp GF := iprop(
  ⌜out.ptr ≠ null ∧ sint.Z out.len = values.length⌝ ∗ DeprecatedEntrySlots out values)

def DeprecatedEntryValuesResult (out : GoSlice) : Option (List DeprecatedEntryPointValue) → IProp GF
  | none => iprop(⌜out = slice.nil⌝)
  | some values => DeprecatedEntryValuesOwned out values

/-- Unused zero records are separate from already decoded slots. -/
def DeprecatedEntryStorage (data : GoSlice) (used count : Nat) : IProp GF :=
  arrayElems (sliceIndexRef DeprecatedEntryPoint (used : Int) data)
    (List.replicate (count - used) (zero_val DeprecatedEntryPoint)) (DFrac.own 1)

theorem deprecatedEntrySlots_nil (data : GoSlice) : ⊢ (DeprecatedEntrySlots data [] : IProp GF) := by
  unfold DeprecatedEntrySlots
  iapply BigSepL.bigSepL_nil.2
  iempintro

theorem deprecatedEntrySlots_snoc (data : GoSlice) (values : List DeprecatedEntryPointValue) (value : DeprecatedEntryPointValue) :
    DeprecatedEntrySlots data values ∗ DeprecatedEntryPointOwned (sliceIndexRef DeprecatedEntryPoint (values.length : Int) data) value ⊢
      (DeprecatedEntrySlots data (values ++ [value]) : IProp GF) := by
  unfold DeprecatedEntrySlots
  iintro ⟨Hslots, Hvalue⟩
  iapply BigSepL.bigSepL_snoc.2
  iframe Hslots Hvalue

/-- Convert the fresh backing array into an inventory of unused zero slots. -/
theorem deprecatedEntryStorage_init (data : GoSlice) (count : Nat) :
    data ↦* List.replicate count (zero_val DeprecatedEntryPoint) ⊢ (DeprecatedEntryStorage data 0 count : IProp GF) := by
  rw [ownSlice_unseal]
  unfold ownSliceDef DeprecatedEntryStorage
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
theorem deprecatedEntryStorage_take_cell (data : GoSlice) (used count : Nat) (more : used < count) :
    DeprecatedEntryStorage data used count ⊢ (iprop(
      sliceIndexRef DeprecatedEntryPoint (used : Int) data ↦ zero_val DeprecatedEntryPoint ∗
      DeprecatedEntryStorage data (used + 1) count) : IProp GF) := by
  have hcount : count - used = (count - (used + 1)) + 1 := by omega
  unfold DeprecatedEntryStorage
  rw [hcount, List.replicate_succ]
  iintro Hslots
  icases (arrayElems_cons _ _ _ _).1 $$ Hslots with ⟨Hfirst, Hslots⟩
  isimp only [go.arrayIndexRef_0] at Hfirst
  have hnext : arrayIndexRef DeprecatedEntryPoint 1 (sliceIndexRef DeprecatedEntryPoint (used : Int) data) =
      sliceIndexRef DeprecatedEntryPoint ((used + 1 : Nat) : Int) data := by
    simp only [sliceIndexRef, ← go.arrayIndexRef_add]
    congr 1
  isimp only [hnext] at Hslots
  iframe Hslots
  iexact Hfirst

theorem deprecatedEntryStorage_take (data : GoSlice) (used count : Nat) (more : used < count) :
    DeprecatedEntryStorage data used count ⊢ (iprop(
      DeprecatedEntryPointOwned (sliceIndexRef DeprecatedEntryPoint (used : Int) data) deprecatedEntryPointZero ∗
      DeprecatedEntryStorage data (used + 1) count) : IProp GF) := by
  have hcount : count - used = (count - (used + 1)) + 1 := by omega
  unfold DeprecatedEntryStorage
  rw [hcount, List.replicate_succ]
  iintro Hslots
  icases (arrayElems_cons _ _ _ _).1 $$ Hslots with ⟨Hfirst, Hslots⟩
  isimp only [go.arrayIndexRef_0] at Hfirst
  have hnext : arrayIndexRef DeprecatedEntryPoint 1 (sliceIndexRef DeprecatedEntryPoint (used : Int) data) =
      sliceIndexRef DeprecatedEntryPoint ((used + 1 : Nat) : Int) data := by
    simp only [sliceIndexRef, ← go.arrayIndexRef_add]
    congr 1
  isimp only [hnext] at Hslots
  iframe Hslots
  iapply deprecatedEntryPointOwned_zero $$ Hfirst

theorem deprecatedEntryValuesOwned_length (out : GoSlice) (values : List DeprecatedEntryPointValue) :
    DeprecatedEntryValuesOwned (GF := GF) out values ⊢ ⌜sint.Z out.len = values.length ∧ out.ptr ≠ null⌝ := by
  unfold DeprecatedEntryValuesOwned
  iintro ⟨%Hsize, _⟩
  ipureintro; exact ⟨Hsize.2, Hsize.1⟩

theorem deprecatedEntryValuesResult_null_iff (out : GoSlice) (values : Option (List DeprecatedEntryPointValue)) :
    DeprecatedEntryValuesResult (GF := GF) out values ⊢ ⌜out.ptr = null ↔ values = none⌝ := by
  cases values with
  | none => isimp only [DeprecatedEntryValuesResult]; iintro %h; ipureintro; simp [h, slice.nil]
  | some values =>
    isimp only [DeprecatedEntryValuesResult, DeprecatedEntryValuesOwned]
    iintro ⟨%Hsize, _⟩
    ipureintro; simp [Hsize.1]

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntrySlots_nil
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntrySlots_snoc
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryStorage_init
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryStorage_take
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryValuesOwned_length
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryValuesResult_null_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryStorage_take_cell
