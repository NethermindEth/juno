import ProofCheck.SierraEntryPointValues
import ProofCheck.InitializationErrors

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

def SierraEntryPointOwned (dst : Loc) (value : SierraEntryPointValue) : IProp GF := iprop(
  ∃ ptr : Loc, dst ↦ SierraEntryPoint.mk value.index ptr ∗ FeltResult ptr value.selector)

/-- Borrow only the index cell, retaining ownership of the nullable selector. -/
theorem sierraEntryPoint_index_acc (dst : Loc) (old : SierraEntryPointValue) :
    SierraEntryPointOwned dst old ⊢ (iprop(
      structFieldRef SierraEntryPoint go!"Index" dst ↦ old.index ∗
      (∀ value : w64, structFieldRef SierraEntryPoint go!"Index" dst ↦ value -∗
        SierraEntryPointOwned dst {old with index := value})) : IProp GF) := by
  unfold SierraEntryPointOwned
  iintro ⟨%ptr, Hh, Hselector⟩
  icases typedPointsto_not_null_dup dst (SierraEntryPoint.mk old.index ptr) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (SierraEntryPoint.mk old.index ptr) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hindex, Hptr, -⟩
  iframe Hindex
  iintro %value Hindex
  iexists ptr
  iframe Hselector
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hindex Hptr

/-- Borrow the selector pointer and child separately, so publication can replace
    the child while a rejected call reconstructs the exact previous record. -/
theorem sierraEntryPoint_selector_acc (dst : Loc) (old : SierraEntryPointValue) :
    SierraEntryPointOwned dst old ⊢ (iprop(∃ ptr : Loc,
      structFieldRef SierraEntryPoint go!"Selector" dst ↦ ptr ∗ FeltResult ptr old.selector ∗
      (∀ (out : Loc) (value : Option (GoArray w64 4)),
        structFieldRef SierraEntryPoint go!"Selector" dst ↦ out ∗ FeltResult out value -∗
        SierraEntryPointOwned dst {old with selector := value})) : IProp GF) := by
  unfold SierraEntryPointOwned
  iintro ⟨%ptr, Hh, Hselector⟩
  icases typedPointsto_not_null_dup dst (SierraEntryPoint.mk old.index ptr) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (SierraEntryPoint.mk old.index ptr) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hindex, Hptr, -⟩
  iexists ptr
  iframe Hptr Hselector
  iintro %out %value ⟨Hptr, Hselector⟩
  iexists out
  iframe Hselector
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hindex Hptr

theorem sierraEntryPointOwned_zero (dst : Loc) :
    dst ↦ zero_val SierraEntryPoint ⊢ (SierraEntryPointOwned dst sierraEntryPointZero : IProp GF) := by
  iintro Hdst
  unfold SierraEntryPointOwned
  iexists null
  have hz : zero_val SierraEntryPoint = SierraEntryPoint.mk (W64 0) null := rfl
  isimp only [hz] at Hdst
  isimp only [sierraEntryPointZero, FeltResult]
  iframe Hdst

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryPoint_index_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryPoint_selector_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryPointOwned_zero
