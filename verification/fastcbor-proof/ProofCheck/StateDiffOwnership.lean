import ProofCheck.StateDiffValues
import ProofCheck.StorageDiffsOwnership
import ProofCheck.MigratedClassesOwnership
import ProofCheck.FeltPointersComplete

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

def StateDiffOwned (dst : Loc) (value : StateDiffValue) : IProp GF := iprop(
  ∃ (storage : Loc) (nonces : Loc) (deployed : Loc) (declaredV0 : GoSlice) (declaredV1 : Loc) (replaced : Loc) (migrated : Loc),
    dst ↦ StateDiff.mk storage nonces deployed declaredV0 declaredV1 replaced migrated ∗
    StorageDiffsResult storage value.storage ∗
    FeltMapResult nonces value.nonces ∗
    FeltMapResult deployed value.deployed ∗
    FeltPointersResult declaredV0 value.declaredV0 ∗
    FeltMapResult declaredV1 value.declaredV1 ∗
    FeltMapResult replaced value.replaced ∗
    MigratedClassesResult migrated value.migrated)

/-- Borrow StorageDiffs and its owned children, retaining every other field. -/
theorem stateDiff_storage_acc (dst : Loc) (old : StateDiffValue) :
    StateDiffOwned dst old ⊢ (iprop(∃ storage : Loc,
      structFieldRef StateDiff go!"StorageDiffs" dst ↦ storage ∗ StorageDiffsResult storage old.storage ∗
      (∀ (out : Loc) (value : Option (GMap FeltMapKey StorageDiffsValue)),
        structFieldRef StateDiff go!"StorageDiffs" dst ↦ out ∗ StorageDiffsResult out value -∗
        StateDiffOwned dst {old with storage := value})) : IProp GF) := by
  unfold StateDiffOwned
  iintro ⟨%storage, %nonces, %deployed, %declaredV0, %declaredV1, %replaced, %migrated, Hh, Hstorage, Hnonces, Hdeployed, HdeclaredV0, HdeclaredV1, Hreplaced, Hmigrated⟩
  icases typedPointsto_not_null_dup dst (StateDiff.mk storage nonces deployed declaredV0 declaredV1 replaced migrated) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (StateDiff.mk storage nonces deployed declaredV0 declaredV1 replaced migrated) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨HstorageCell, HnoncesCell, HdeployedCell, HdeclaredV0Cell, HdeclaredV1Cell, HreplacedCell, HmigratedCell, -⟩
  iexists storage
  iframe HstorageCell Hstorage
  iintro %out %value ⟨HstorageCell, Hstorage⟩
  iexists out, nonces, deployed, declaredV0, declaredV1, replaced, migrated
  iframe Hstorage Hnonces Hdeployed HdeclaredV0 HdeclaredV1 Hreplaced Hmigrated
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe HstorageCell HnoncesCell HdeployedCell HdeclaredV0Cell HdeclaredV1Cell HreplacedCell HmigratedCell

/-- Borrow Nonces and its owned children, retaining every other field. -/
theorem stateDiff_nonces_acc (dst : Loc) (old : StateDiffValue) :
    StateDiffOwned dst old ⊢ (iprop(∃ nonces : Loc,
      structFieldRef StateDiff go!"Nonces" dst ↦ nonces ∗ FeltMapResult nonces old.nonces ∗
      (∀ (out : Loc) (value : Option (GMap FeltMapKey FeltMapValue)),
        structFieldRef StateDiff go!"Nonces" dst ↦ out ∗ FeltMapResult out value -∗
        StateDiffOwned dst {old with nonces := value})) : IProp GF) := by
  unfold StateDiffOwned
  iintro ⟨%storage, %nonces, %deployed, %declaredV0, %declaredV1, %replaced, %migrated, Hh, Hstorage, Hnonces, Hdeployed, HdeclaredV0, HdeclaredV1, Hreplaced, Hmigrated⟩
  icases typedPointsto_not_null_dup dst (StateDiff.mk storage nonces deployed declaredV0 declaredV1 replaced migrated) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (StateDiff.mk storage nonces deployed declaredV0 declaredV1 replaced migrated) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨HstorageCell, HnoncesCell, HdeployedCell, HdeclaredV0Cell, HdeclaredV1Cell, HreplacedCell, HmigratedCell, -⟩
  iexists nonces
  iframe HnoncesCell Hnonces
  iintro %out %value ⟨HnoncesCell, Hnonces⟩
  iexists storage, out, deployed, declaredV0, declaredV1, replaced, migrated
  iframe Hstorage Hnonces Hdeployed HdeclaredV0 HdeclaredV1 Hreplaced Hmigrated
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe HstorageCell HnoncesCell HdeployedCell HdeclaredV0Cell HdeclaredV1Cell HreplacedCell HmigratedCell

/-- Borrow DeployedContracts and its owned children, retaining every other field. -/
theorem stateDiff_deployed_acc (dst : Loc) (old : StateDiffValue) :
    StateDiffOwned dst old ⊢ (iprop(∃ deployed : Loc,
      structFieldRef StateDiff go!"DeployedContracts" dst ↦ deployed ∗ FeltMapResult deployed old.deployed ∗
      (∀ (out : Loc) (value : Option (GMap FeltMapKey FeltMapValue)),
        structFieldRef StateDiff go!"DeployedContracts" dst ↦ out ∗ FeltMapResult out value -∗
        StateDiffOwned dst {old with deployed := value})) : IProp GF) := by
  unfold StateDiffOwned
  iintro ⟨%storage, %nonces, %deployed, %declaredV0, %declaredV1, %replaced, %migrated, Hh, Hstorage, Hnonces, Hdeployed, HdeclaredV0, HdeclaredV1, Hreplaced, Hmigrated⟩
  icases typedPointsto_not_null_dup dst (StateDiff.mk storage nonces deployed declaredV0 declaredV1 replaced migrated) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (StateDiff.mk storage nonces deployed declaredV0 declaredV1 replaced migrated) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨HstorageCell, HnoncesCell, HdeployedCell, HdeclaredV0Cell, HdeclaredV1Cell, HreplacedCell, HmigratedCell, -⟩
  iexists deployed
  iframe HdeployedCell Hdeployed
  iintro %out %value ⟨HdeployedCell, Hdeployed⟩
  iexists storage, nonces, out, declaredV0, declaredV1, replaced, migrated
  iframe Hstorage Hnonces Hdeployed HdeclaredV0 HdeclaredV1 Hreplaced Hmigrated
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe HstorageCell HnoncesCell HdeployedCell HdeclaredV0Cell HdeclaredV1Cell HreplacedCell HmigratedCell

/-- Borrow DeclaredV0Classes and its owned children, retaining every other field. -/
theorem stateDiff_declaredV0_acc (dst : Loc) (old : StateDiffValue) :
    StateDiffOwned dst old ⊢ (iprop(∃ declaredV0 : GoSlice,
      structFieldRef StateDiff go!"DeclaredV0Classes" dst ↦ declaredV0 ∗ FeltPointersResult declaredV0 old.declaredV0 ∗
      (∀ (out : GoSlice) (value : Option (List (Option (GoArray w64 4)))),
        structFieldRef StateDiff go!"DeclaredV0Classes" dst ↦ out ∗ FeltPointersResult out value -∗
        StateDiffOwned dst {old with declaredV0 := value})) : IProp GF) := by
  unfold StateDiffOwned
  iintro ⟨%storage, %nonces, %deployed, %declaredV0, %declaredV1, %replaced, %migrated, Hh, Hstorage, Hnonces, Hdeployed, HdeclaredV0, HdeclaredV1, Hreplaced, Hmigrated⟩
  icases typedPointsto_not_null_dup dst (StateDiff.mk storage nonces deployed declaredV0 declaredV1 replaced migrated) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (StateDiff.mk storage nonces deployed declaredV0 declaredV1 replaced migrated) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨HstorageCell, HnoncesCell, HdeployedCell, HdeclaredV0Cell, HdeclaredV1Cell, HreplacedCell, HmigratedCell, -⟩
  iexists declaredV0
  iframe HdeclaredV0Cell HdeclaredV0
  iintro %out %value ⟨HdeclaredV0Cell, HdeclaredV0⟩
  iexists storage, nonces, deployed, out, declaredV1, replaced, migrated
  iframe Hstorage Hnonces Hdeployed HdeclaredV0 HdeclaredV1 Hreplaced Hmigrated
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe HstorageCell HnoncesCell HdeployedCell HdeclaredV0Cell HdeclaredV1Cell HreplacedCell HmigratedCell

/-- Borrow DeclaredV1Classes and its owned children, retaining every other field. -/
theorem stateDiff_declaredV1_acc (dst : Loc) (old : StateDiffValue) :
    StateDiffOwned dst old ⊢ (iprop(∃ declaredV1 : Loc,
      structFieldRef StateDiff go!"DeclaredV1Classes" dst ↦ declaredV1 ∗ FeltMapResult declaredV1 old.declaredV1 ∗
      (∀ (out : Loc) (value : Option (GMap FeltMapKey FeltMapValue)),
        structFieldRef StateDiff go!"DeclaredV1Classes" dst ↦ out ∗ FeltMapResult out value -∗
        StateDiffOwned dst {old with declaredV1 := value})) : IProp GF) := by
  unfold StateDiffOwned
  iintro ⟨%storage, %nonces, %deployed, %declaredV0, %declaredV1, %replaced, %migrated, Hh, Hstorage, Hnonces, Hdeployed, HdeclaredV0, HdeclaredV1, Hreplaced, Hmigrated⟩
  icases typedPointsto_not_null_dup dst (StateDiff.mk storage nonces deployed declaredV0 declaredV1 replaced migrated) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (StateDiff.mk storage nonces deployed declaredV0 declaredV1 replaced migrated) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨HstorageCell, HnoncesCell, HdeployedCell, HdeclaredV0Cell, HdeclaredV1Cell, HreplacedCell, HmigratedCell, -⟩
  iexists declaredV1
  iframe HdeclaredV1Cell HdeclaredV1
  iintro %out %value ⟨HdeclaredV1Cell, HdeclaredV1⟩
  iexists storage, nonces, deployed, declaredV0, out, replaced, migrated
  iframe Hstorage Hnonces Hdeployed HdeclaredV0 HdeclaredV1 Hreplaced Hmigrated
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe HstorageCell HnoncesCell HdeployedCell HdeclaredV0Cell HdeclaredV1Cell HreplacedCell HmigratedCell

/-- Borrow ReplacedClasses and its owned children, retaining every other field. -/
theorem stateDiff_replaced_acc (dst : Loc) (old : StateDiffValue) :
    StateDiffOwned dst old ⊢ (iprop(∃ replaced : Loc,
      structFieldRef StateDiff go!"ReplacedClasses" dst ↦ replaced ∗ FeltMapResult replaced old.replaced ∗
      (∀ (out : Loc) (value : Option (GMap FeltMapKey FeltMapValue)),
        structFieldRef StateDiff go!"ReplacedClasses" dst ↦ out ∗ FeltMapResult out value -∗
        StateDiffOwned dst {old with replaced := value})) : IProp GF) := by
  unfold StateDiffOwned
  iintro ⟨%storage, %nonces, %deployed, %declaredV0, %declaredV1, %replaced, %migrated, Hh, Hstorage, Hnonces, Hdeployed, HdeclaredV0, HdeclaredV1, Hreplaced, Hmigrated⟩
  icases typedPointsto_not_null_dup dst (StateDiff.mk storage nonces deployed declaredV0 declaredV1 replaced migrated) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (StateDiff.mk storage nonces deployed declaredV0 declaredV1 replaced migrated) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨HstorageCell, HnoncesCell, HdeployedCell, HdeclaredV0Cell, HdeclaredV1Cell, HreplacedCell, HmigratedCell, -⟩
  iexists replaced
  iframe HreplacedCell Hreplaced
  iintro %out %value ⟨HreplacedCell, Hreplaced⟩
  iexists storage, nonces, deployed, declaredV0, declaredV1, out, migrated
  iframe Hstorage Hnonces Hdeployed HdeclaredV0 HdeclaredV1 Hreplaced Hmigrated
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe HstorageCell HnoncesCell HdeployedCell HdeclaredV0Cell HdeclaredV1Cell HreplacedCell HmigratedCell

/-- Borrow MigratedClasses and its owned children, retaining every other field. -/
theorem stateDiff_migrated_acc (dst : Loc) (old : StateDiffValue) :
    StateDiffOwned dst old ⊢ (iprop(∃ migrated : Loc,
      structFieldRef StateDiff go!"MigratedClasses" dst ↦ migrated ∗ MigratedClassesResult migrated old.migrated ∗
      (∀ (out : Loc) (value : Option (GMap FeltMapKey FeltMapKey)),
        structFieldRef StateDiff go!"MigratedClasses" dst ↦ out ∗ MigratedClassesResult out value -∗
        StateDiffOwned dst {old with migrated := value})) : IProp GF) := by
  unfold StateDiffOwned
  iintro ⟨%storage, %nonces, %deployed, %declaredV0, %declaredV1, %replaced, %migrated, Hh, Hstorage, Hnonces, Hdeployed, HdeclaredV0, HdeclaredV1, Hreplaced, Hmigrated⟩
  icases typedPointsto_not_null_dup dst (StateDiff.mk storage nonces deployed declaredV0 declaredV1 replaced migrated) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (StateDiff.mk storage nonces deployed declaredV0 declaredV1 replaced migrated) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨HstorageCell, HnoncesCell, HdeployedCell, HdeclaredV0Cell, HdeclaredV1Cell, HreplacedCell, HmigratedCell, -⟩
  iexists migrated
  iframe HmigratedCell Hmigrated
  iintro %out %value ⟨HmigratedCell, Hmigrated⟩
  iexists storage, nonces, deployed, declaredV0, declaredV1, replaced, out
  iframe Hstorage Hnonces Hdeployed HdeclaredV0 HdeclaredV1 Hreplaced Hmigrated
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe HstorageCell HnoncesCell HdeployedCell HdeclaredV0Cell HdeclaredV1Cell HreplacedCell HmigratedCell

theorem stateDiffOwned_zero (dst : Loc) :
    dst ↦ zero_val StateDiff ⊢ (StateDiffOwned dst stateDiffZero : IProp GF) := by
  iintro Hdst
  unfold StateDiffOwned
  iexists map.nil, map.nil, map.nil, slice.nil, map.nil, map.nil, map.nil
  have hz : zero_val StateDiff = StateDiff.mk map.nil map.nil map.nil slice.nil map.nil map.nil map.nil := rfl
  isimp only [hz] at Hdst
  isimp only [stateDiffZero, StorageDiffsResult, FeltMapResult, FeltPointersResult, MigratedClassesResult]
  iframe Hdst
  ipureintro; trivial

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiff_storage_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiff_nonces_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiff_deployed_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiff_declaredV0_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiff_declaredV1_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiff_replaced_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiff_migrated_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffOwned_zero
