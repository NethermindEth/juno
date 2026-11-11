import ProofCheck.AvailabilityValues
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

def availabilityConcrete (value : AvailabilityValue) : DataAvailability := ⟨value.gas, value.dataGas⟩
def AvailabilityOwned (dst : Loc) (value : AvailabilityValue) : IProp GF := iprop(dst ↦ availabilityConcrete value)

/-- Each scalar call borrows its actual field and retains the other counter. -/
theorem availability_field_acc (dst : Loc) (old : AvailabilityValue) (field : AvailabilityField) :
    AvailabilityOwned dst old ⊢ (iprop(
      structFieldRef DataAvailability (availabilityFieldName field) dst ↦ availabilityGet field old ∗
      (∀ value : w64, structFieldRef DataAvailability (availabilityFieldName field) dst ↦ value -∗
        AvailabilityOwned dst (availabilitySet field old value))) : IProp GF) := by
  unfold AvailabilityOwned
  iintro Hh
  icases typedPointsto_not_null_dup dst (availabilityConcrete old) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (availabilityConcrete old) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef, availabilityConcrete] at Hh
  icases Hh with ⟨Hgas, Hdata, -⟩
  cases field <;> isimp only [availabilityFieldName, availabilityGet, availabilitySet]
  · iframe Hgas
    iintro %value Hgas
    iapply typedPointsto_combine _ _ _ Hnn
    isimp only [TypedPointsto.typedPointstoDef, availabilityConcrete]
    iframe Hgas Hdata
  · iframe Hdata
    iintro %value Hdata
    iapply typedPointsto_combine _ _ _ Hnn
    isimp only [TypedPointsto.typedPointstoDef, availabilityConcrete]
    iframe Hgas Hdata

theorem availabilityOwned_zero (dst : Loc) :
    dst ↦ zero_val DataAvailability ⊢ (AvailabilityOwned dst availabilityZero : IProp GF) := by
  unfold AvailabilityOwned
  have hzero : availabilityConcrete availabilityZero = zero_val DataAvailability := rfl
  rw [hzero]

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.availability_field_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilityOwned_zero
