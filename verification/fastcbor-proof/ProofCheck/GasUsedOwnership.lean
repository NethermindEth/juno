import ProofCheck.GasUsedValues
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

def gasUsedConcrete (value : GasUsedValue) : GasConsumed := ⟨value.gas, value.dataGas, value.l2Gas⟩
def GasUsedOwned (dst : Loc) (value : GasUsedValue) : IProp GF := iprop(dst ↦ gasUsedConcrete value)

/-- Each scalar call borrows its actual field and retains every other counter. -/
theorem gasUsed_field_acc (dst : Loc) (old : GasUsedValue) (field : GasUsedField) :
    GasUsedOwned dst old ⊢ (iprop(
      structFieldRef GasConsumed (gasUsedFieldName field) dst ↦ gasUsedGet field old ∗
      (∀ value : w64, structFieldRef GasConsumed (gasUsedFieldName field) dst ↦ value -∗
        GasUsedOwned dst (gasUsedSet field old value))) : IProp GF) := by
  unfold GasUsedOwned
  iintro Hh
  icases typedPointsto_not_null_dup dst (gasUsedConcrete old) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (gasUsedConcrete old) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef, gasUsedConcrete] at Hh
  icases Hh with ⟨Hgas, HdataGas, Hl2Gas, -⟩
  cases field <;> isimp only [gasUsedFieldName, gasUsedGet, gasUsedSet]
  · iframe Hgas
    iintro %value Hgas
    iapply typedPointsto_combine _ _ _ Hnn
    isimp only [TypedPointsto.typedPointstoDef, gasUsedConcrete]
    iframe Hgas HdataGas Hl2Gas
  · iframe HdataGas
    iintro %value HdataGas
    iapply typedPointsto_combine _ _ _ Hnn
    isimp only [TypedPointsto.typedPointstoDef, gasUsedConcrete]
    iframe Hgas HdataGas Hl2Gas
  · iframe Hl2Gas
    iintro %value Hl2Gas
    iapply typedPointsto_combine _ _ _ Hnn
    isimp only [TypedPointsto.typedPointstoDef, gasUsedConcrete]
    iframe Hgas HdataGas Hl2Gas

theorem gasUsedOwned_zero (dst : Loc) :
    dst ↦ zero_val GasConsumed ⊢ (GasUsedOwned dst gasUsedZero : IProp GF) := by
  unfold GasUsedOwned
  have hzero : gasUsedConcrete gasUsedZero = zero_val GasConsumed := rfl
  rw [hzero]

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsed_field_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedOwned_zero
