import ProofCheck.SierraEntriesValues
import ProofCheck.SierraEntryValuesComplete
import ProofCheck.ScalarComplete
import ProofCheck.FeltValid
import ProofCheck.FeltFailureSound

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

def SierraEntriesOwned (dst : Loc) (value : SierraEntriesValue) : IProp GF := iprop(
  ∃ (constructors external l1Handler : GoSlice),
    dst ↦ SierraEntryPointsByType.mk constructors external l1Handler ∗
    SierraEntryValuesResult constructors value.constructors ∗
    SierraEntryValuesResult external value.external ∗
    SierraEntryValuesResult l1Handler value.l1Handler)

/-- Borrow the constructors slice and children while retaining the other groups. -/
theorem sierraEntries_constructors_acc (dst : Loc) (old : SierraEntriesValue) :
    SierraEntriesOwned dst old ⊢ (iprop(∃ constructors : GoSlice,
      structFieldRef SierraEntryPointsByType go!"Constructor" dst ↦ constructors ∗ SierraEntryValuesResult constructors old.constructors ∗
      (∀ (out : GoSlice) (value : Option (List SierraEntryPointValue)),
        structFieldRef SierraEntryPointsByType go!"Constructor" dst ↦ out ∗ SierraEntryValuesResult out value -∗
        SierraEntriesOwned dst {old with constructors := value})) : IProp GF) := by
  unfold SierraEntriesOwned
  iintro ⟨%constructors, %external, %l1Handler, Hh, Hconstructors, Hexternal, Hl1Handler⟩
  icases typedPointsto_not_null_dup dst (SierraEntryPointsByType.mk constructors external l1Handler) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (SierraEntryPointsByType.mk constructors external l1Handler) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨HconstructorsCell, HexternalCell, Hl1HandlerCell, -⟩
  iexists constructors
  iframe HconstructorsCell Hconstructors
  iintro %out %value ⟨HconstructorsCell, Hconstructors⟩
  iexists out, external, l1Handler
  iframe Hconstructors Hexternal Hl1Handler
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe HconstructorsCell HexternalCell Hl1HandlerCell

/-- Borrow the external slice and children while retaining the other groups. -/
theorem sierraEntries_external_acc (dst : Loc) (old : SierraEntriesValue) :
    SierraEntriesOwned dst old ⊢ (iprop(∃ external : GoSlice,
      structFieldRef SierraEntryPointsByType go!"External" dst ↦ external ∗ SierraEntryValuesResult external old.external ∗
      (∀ (out : GoSlice) (value : Option (List SierraEntryPointValue)),
        structFieldRef SierraEntryPointsByType go!"External" dst ↦ out ∗ SierraEntryValuesResult out value -∗
        SierraEntriesOwned dst {old with external := value})) : IProp GF) := by
  unfold SierraEntriesOwned
  iintro ⟨%constructors, %external, %l1Handler, Hh, Hconstructors, Hexternal, Hl1Handler⟩
  icases typedPointsto_not_null_dup dst (SierraEntryPointsByType.mk constructors external l1Handler) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (SierraEntryPointsByType.mk constructors external l1Handler) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨HconstructorsCell, HexternalCell, Hl1HandlerCell, -⟩
  iexists external
  iframe HexternalCell Hexternal
  iintro %out %value ⟨HexternalCell, Hexternal⟩
  iexists constructors, out, l1Handler
  iframe Hconstructors Hexternal Hl1Handler
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe HconstructorsCell HexternalCell Hl1HandlerCell

/-- Borrow the l1Handler slice and children while retaining the other groups. -/
theorem sierraEntries_l1Handler_acc (dst : Loc) (old : SierraEntriesValue) :
    SierraEntriesOwned dst old ⊢ (iprop(∃ l1Handler : GoSlice,
      structFieldRef SierraEntryPointsByType go!"L1Handler" dst ↦ l1Handler ∗ SierraEntryValuesResult l1Handler old.l1Handler ∗
      (∀ (out : GoSlice) (value : Option (List SierraEntryPointValue)),
        structFieldRef SierraEntryPointsByType go!"L1Handler" dst ↦ out ∗ SierraEntryValuesResult out value -∗
        SierraEntriesOwned dst {old with l1Handler := value})) : IProp GF) := by
  unfold SierraEntriesOwned
  iintro ⟨%constructors, %external, %l1Handler, Hh, Hconstructors, Hexternal, Hl1Handler⟩
  icases typedPointsto_not_null_dup dst (SierraEntryPointsByType.mk constructors external l1Handler) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (SierraEntryPointsByType.mk constructors external l1Handler) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨HconstructorsCell, HexternalCell, Hl1HandlerCell, -⟩
  iexists l1Handler
  iframe Hl1HandlerCell Hl1Handler
  iintro %out %value ⟨Hl1HandlerCell, Hl1Handler⟩
  iexists constructors, external, out
  iframe Hconstructors Hexternal Hl1Handler
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe HconstructorsCell HexternalCell Hl1HandlerCell

theorem sierraEntriesOwned_zero (dst : Loc) :
    dst ↦ zero_val SierraEntryPointsByType ⊢ (SierraEntriesOwned dst sierraEntriesZero : IProp GF) := by
  iintro Hdst
  unfold SierraEntriesOwned
  iexists slice.nil, slice.nil, slice.nil
  have hz : zero_val SierraEntryPointsByType = SierraEntryPointsByType.mk slice.nil slice.nil slice.nil := rfl
  isimp only [hz] at Hdst
  isimp only [sierraEntriesZero, SierraEntryValuesResult]
  iframe Hdst
  ipureintro; trivial

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntries_constructors_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntries_external_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntries_l1Handler_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntriesOwned_zero
