import ProofCheck.ExecutionValues
import ProofCheck.BuiltinsRecordComplete
import ProofCheck.AvailabilitySlotComplete
import ProofCheck.GasUsedSlotComplete

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

/-- The record points only to its supplied slots or nil. Child storage is owned
    exactly once, even while published, and also while a pointer is cleared. -/
def executionConcrete (availability gas : Loc) (value : ExecutionValue) : ExecutionResources :=
  ⟨builtinsConcrete value.builtins, value.holes, value.steps,
    if value.availabilityPresent then availability else null,
    if value.gasPresent then gas else null⟩

def ExecutionOwned (dst availability gas : Loc) (value : ExecutionValue) : IProp GF := iprop(
  dst ↦ executionConcrete availability gas value ∗
  AvailabilityOwned availability value.availabilityStorage ∗ GasUsedOwned gas value.gasStorage)

theorem execution_builtins_acc (dst : Loc) (h : ExecutionResources) :
    dst ↦ h ⊢ (structFieldRef ExecutionResources go!"BuiltinInstanceCounter" dst ↦ h.BuiltinInstanceCounter' ∗
      (∀ value : BuiltinInstanceCounter, structFieldRef ExecutionResources go!"BuiltinInstanceCounter" dst ↦ value -∗
        dst ↦ {h with BuiltinInstanceCounter' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hbuiltins, Hholes, Hsteps, Havailability, Hgas, -⟩
  iframe Hbuiltins
  iintro %value Hbuiltins
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hbuiltins Hholes Hsteps Havailability Hgas

theorem execution_holes_acc (dst : Loc) (h : ExecutionResources) :
    dst ↦ h ⊢ (structFieldRef ExecutionResources go!"MemoryHoles" dst ↦ h.MemoryHoles' ∗
      (∀ value : w64, structFieldRef ExecutionResources go!"MemoryHoles" dst ↦ value -∗
        dst ↦ {h with MemoryHoles' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hbuiltins, Hholes, Hsteps, Havailability, Hgas, -⟩
  iframe Hholes
  iintro %value Hholes
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hbuiltins Hholes Hsteps Havailability Hgas

theorem execution_steps_acc (dst : Loc) (h : ExecutionResources) :
    dst ↦ h ⊢ (structFieldRef ExecutionResources go!"Steps" dst ↦ h.Steps' ∗
      (∀ value : w64, structFieldRef ExecutionResources go!"Steps" dst ↦ value -∗
        dst ↦ {h with Steps' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hbuiltins, Hholes, Hsteps, Havailability, Hgas, -⟩
  iframe Hsteps
  iintro %value Hsteps
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hbuiltins Hholes Hsteps Havailability Hgas

theorem execution_availability_acc (dst : Loc) (h : ExecutionResources) :
    dst ↦ h ⊢ (structFieldRef ExecutionResources go!"DataAvailability" dst ↦ h.DataAvailability' ∗
      (∀ value : Loc, structFieldRef ExecutionResources go!"DataAvailability" dst ↦ value -∗
        dst ↦ {h with DataAvailability' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hbuiltins, Hholes, Hsteps, Havailability, Hgas, -⟩
  iframe Havailability
  iintro %value Havailability
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hbuiltins Hholes Hsteps Havailability Hgas

theorem execution_gas_acc (dst : Loc) (h : ExecutionResources) :
    dst ↦ h ⊢ (structFieldRef ExecutionResources go!"TotalGasConsumed" dst ↦ h.TotalGasConsumed' ∗
      (∀ value : Loc, structFieldRef ExecutionResources go!"TotalGasConsumed" dst ↦ value -∗
        dst ↦ {h with TotalGasConsumed' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hbuiltins, Hholes, Hsteps, Havailability, Hgas, -⟩
  iframe Hgas
  iintro %value Hgas
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hbuiltins Hholes Hsteps Havailability Hgas

theorem executionOwned_zero (dst availability gas : Loc) :
    dst ↦ zero_val ExecutionResources ∗ availability ↦ zero_val DataAvailability ∗ gas ↦ zero_val GasConsumed ⊢
      (ExecutionOwned dst availability gas executionZero : IProp GF) := by
  iintro ⟨Hdst, Ha, Hg⟩
  ihave Ha := availabilityOwned_zero availability $$ Ha
  ihave Hg := gasUsedOwned_zero gas $$ Hg
  unfold ExecutionOwned
  have hz : executionConcrete availability gas executionZero = zero_val ExecutionResources := rfl
  rw [hz]
  isimp only [executionZero]
  iframe Hdst Ha Hg

/-- The actual single block allocation separates into the inline record and its
    two child slots. No extra allocation or disjointness premise is postulated. -/
theorem executionBlock_zero (block : Loc) :
    block ↦ zero_val cborExecutionResourcesBlock ⊢
      (ExecutionOwned (structFieldRef cborExecutionResourcesBlock go!"Value" block)
        (structFieldRef cborExecutionResourcesBlock go!"Availability" block)
        (structFieldRef cborExecutionResourcesBlock go!"Gas" block) executionZero : IProp GF) := by
  iintro Hb
  icases typedPointsto_split block (zero_val cborExecutionResourcesBlock) (DFrac.own 1) $$ Hb with Hb
  isimp only [TypedPointsto.typedPointstoDef] at Hb
  icases Hb with ⟨Hdst, Ha, Hg, -⟩
  iapply executionOwned_zero
  iframe Hdst Ha Hg

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.execution_builtins_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.execution_holes_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.execution_steps_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.execution_availability_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.execution_gas_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.executionOwned_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.executionBlock_zero
