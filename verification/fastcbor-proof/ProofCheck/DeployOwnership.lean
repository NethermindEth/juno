import ProofCheck.DeployValues
import ProofCheck.FeltValuesTyped

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

/-- Every stored field owns its nullable child independently. -/
def DeployChildren (h : DeployTransaction) (value : DeployValue) : IProp GF := iprop(
  FeltResult h.TransactionHash' value.hash ∗
  FeltResult h.ContractAddressSalt' value.salt ∗
  FeltResult h.ContractAddress' value.contract ∗
  FeltResult h.ClassHash' value.classHash ∗
  FeltValuesResult h.ConstructorCallData' value.constructorCallData ∗
  FeltResult h.Version' value.version)

def DeployOwned (dst : Loc) (value : DeployValue) : IProp GF := iprop(
  ∃ h : DeployTransaction, dst ↦ h ∗ DeployChildren h value)

/-- Borrow the TransactionHash field while retaining every other concrete cell. -/
theorem deploy_hash_acc (dst : Loc) (h : DeployTransaction) :
    dst ↦ h ⊢ (structFieldRef DeployTransaction go!"TransactionHash" dst ↦ h.TransactionHash' ∗
      (∀ value : Loc, structFieldRef DeployTransaction go!"TransactionHash" dst ↦ value -∗
        dst ↦ {h with TransactionHash' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, Hsalt, Hcontract, HclassHash, HconstructorCallData, Hversion, -⟩
  iframe Hhash
  iintro %value Hhash
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash Hsalt Hcontract HclassHash HconstructorCallData Hversion

/-- Borrow the ContractAddressSalt field while retaining every other concrete cell. -/
theorem deploy_salt_acc (dst : Loc) (h : DeployTransaction) :
    dst ↦ h ⊢ (structFieldRef DeployTransaction go!"ContractAddressSalt" dst ↦ h.ContractAddressSalt' ∗
      (∀ value : Loc, structFieldRef DeployTransaction go!"ContractAddressSalt" dst ↦ value -∗
        dst ↦ {h with ContractAddressSalt' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, Hsalt, Hcontract, HclassHash, HconstructorCallData, Hversion, -⟩
  iframe Hsalt
  iintro %value Hsalt
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash Hsalt Hcontract HclassHash HconstructorCallData Hversion

/-- Borrow the ContractAddress field while retaining every other concrete cell. -/
theorem deploy_contract_acc (dst : Loc) (h : DeployTransaction) :
    dst ↦ h ⊢ (structFieldRef DeployTransaction go!"ContractAddress" dst ↦ h.ContractAddress' ∗
      (∀ value : Loc, structFieldRef DeployTransaction go!"ContractAddress" dst ↦ value -∗
        dst ↦ {h with ContractAddress' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, Hsalt, Hcontract, HclassHash, HconstructorCallData, Hversion, -⟩
  iframe Hcontract
  iintro %value Hcontract
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash Hsalt Hcontract HclassHash HconstructorCallData Hversion

/-- Borrow the ClassHash field while retaining every other concrete cell. -/
theorem deploy_classHash_acc (dst : Loc) (h : DeployTransaction) :
    dst ↦ h ⊢ (structFieldRef DeployTransaction go!"ClassHash" dst ↦ h.ClassHash' ∗
      (∀ value : Loc, structFieldRef DeployTransaction go!"ClassHash" dst ↦ value -∗
        dst ↦ {h with ClassHash' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, Hsalt, Hcontract, HclassHash, HconstructorCallData, Hversion, -⟩
  iframe HclassHash
  iintro %value HclassHash
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash Hsalt Hcontract HclassHash HconstructorCallData Hversion

/-- Borrow the ConstructorCallData field while retaining every other concrete cell. -/
theorem deploy_constructorCallData_acc (dst : Loc) (h : DeployTransaction) :
    dst ↦ h ⊢ (structFieldRef DeployTransaction go!"ConstructorCallData" dst ↦ h.ConstructorCallData' ∗
      (∀ value : GoSlice, structFieldRef DeployTransaction go!"ConstructorCallData" dst ↦ value -∗
        dst ↦ {h with ConstructorCallData' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, Hsalt, Hcontract, HclassHash, HconstructorCallData, Hversion, -⟩
  iframe HconstructorCallData
  iintro %value HconstructorCallData
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash Hsalt Hcontract HclassHash HconstructorCallData Hversion

/-- Borrow the Version field while retaining every other concrete cell. -/
theorem deploy_version_acc (dst : Loc) (h : DeployTransaction) :
    dst ↦ h ⊢ (structFieldRef DeployTransaction go!"Version" dst ↦ h.Version' ∗
      (∀ value : Loc, structFieldRef DeployTransaction go!"Version" dst ↦ value -∗
        dst ↦ {h with Version' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, Hsalt, Hcontract, HclassHash, HconstructorCallData, Hversion, -⟩
  iframe Hversion
  iintro %value Hversion
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash Hsalt Hcontract HclassHash HconstructorCallData Hversion

theorem deployChildren_hash (h : DeployTransaction) (old : DeployValue) (out : Loc) (value : Option (GoArray w64 4)) :
    DeployChildren h old ∗ FeltResult out value ⊢
      (DeployChildren {h with TransactionHash' := out} {old with hash := value} : IProp GF) := by
  unfold DeployChildren
  iintro ⟨⟨_, Hsalt, Hcontract, HclassHash, HconstructorCallData, Hversion⟩, Hvalue⟩
  iframe Hvalue Hsalt Hcontract HclassHash HconstructorCallData Hversion

theorem deployChildren_salt (h : DeployTransaction) (old : DeployValue) (out : Loc) (value : Option (GoArray w64 4)) :
    DeployChildren h old ∗ FeltResult out value ⊢
      (DeployChildren {h with ContractAddressSalt' := out} {old with salt := value} : IProp GF) := by
  unfold DeployChildren
  iintro ⟨⟨Hhash, _, Hcontract, HclassHash, HconstructorCallData, Hversion⟩, Hvalue⟩
  iframe Hhash Hvalue Hcontract HclassHash HconstructorCallData Hversion

theorem deployChildren_contract (h : DeployTransaction) (old : DeployValue) (out : Loc) (value : Option (GoArray w64 4)) :
    DeployChildren h old ∗ FeltResult out value ⊢
      (DeployChildren {h with ContractAddress' := out} {old with contract := value} : IProp GF) := by
  unfold DeployChildren
  iintro ⟨⟨Hhash, Hsalt, _, HclassHash, HconstructorCallData, Hversion⟩, Hvalue⟩
  iframe Hhash Hsalt Hvalue HclassHash HconstructorCallData Hversion

theorem deployChildren_classHash (h : DeployTransaction) (old : DeployValue) (out : Loc) (value : Option (GoArray w64 4)) :
    DeployChildren h old ∗ FeltResult out value ⊢
      (DeployChildren {h with ClassHash' := out} {old with classHash := value} : IProp GF) := by
  unfold DeployChildren
  iintro ⟨⟨Hhash, Hsalt, Hcontract, _, HconstructorCallData, Hversion⟩, Hvalue⟩
  iframe Hhash Hsalt Hcontract Hvalue HconstructorCallData Hversion

theorem deployChildren_constructorCallData (h : DeployTransaction) (old : DeployValue) (out : GoSlice) (value : Option (List (GoArray w64 4))) :
    DeployChildren h old ∗ FeltValuesResult out value ⊢
      (DeployChildren {h with ConstructorCallData' := out} {old with constructorCallData := value} : IProp GF) := by
  unfold DeployChildren
  iintro ⟨⟨Hhash, Hsalt, Hcontract, HclassHash, _, Hversion⟩, Hvalue⟩
  iframe Hhash Hsalt Hcontract HclassHash Hvalue Hversion

theorem deployChildren_version (h : DeployTransaction) (old : DeployValue) (out : Loc) (value : Option (GoArray w64 4)) :
    DeployChildren h old ∗ FeltResult out value ⊢
      (DeployChildren {h with Version' := out} {old with version := value} : IProp GF) := by
  unfold DeployChildren
  iintro ⟨⟨Hhash, Hsalt, Hcontract, HclassHash, HconstructorCallData, _⟩, Hvalue⟩
  iframe Hhash Hsalt Hcontract HclassHash HconstructorCallData Hvalue

/-- The zero transaction owns no child allocations. -/
theorem deployChildren_zero : ⊢ (DeployChildren (zero_val DeployTransaction) deployZero : IProp GF) := by
  isimp only [DeployChildren, deployZero, FeltResult, FeltValuesResult]
  ipureintro
  exact ⟨rfl, rfl, rfl, rfl, rfl, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.deploy_hash_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.deploy_salt_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.deploy_contract_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.deploy_classHash_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.deploy_constructorCallData_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.deploy_version_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.deployChildren_hash
#print axioms Perennial.github_com.NethermindEth.juno.core.deployChildren_salt
#print axioms Perennial.github_com.NethermindEth.juno.core.deployChildren_contract
#print axioms Perennial.github_com.NethermindEth.juno.core.deployChildren_classHash
#print axioms Perennial.github_com.NethermindEth.juno.core.deployChildren_constructorCallData
#print axioms Perennial.github_com.NethermindEth.juno.core.deployChildren_version
#print axioms Perennial.github_com.NethermindEth.juno.core.deployChildren_zero
