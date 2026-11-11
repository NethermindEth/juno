import ProofCheck.L1HandlerValues
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
def L1HandlerChildren (h : L1HandlerTransaction) (value : L1HandlerValue) : IProp GF := iprop(
  FeltResult h.TransactionHash' value.hash ∗
  FeltResult h.ContractAddress' value.contract ∗
  FeltResult h.EntryPointSelector' value.entry ∗
  FeltResult h.Nonce' value.nonce ∗
  FeltValuesResult h.CallData' value.callData ∗
  FeltResult h.Version' value.version)

def L1HandlerOwned (dst : Loc) (value : L1HandlerValue) : IProp GF := iprop(
  ∃ h : L1HandlerTransaction, dst ↦ h ∗ L1HandlerChildren h value)

/-- Borrow the TransactionHash field while retaining every other concrete cell. -/
theorem l1Handler_hash_acc (dst : Loc) (h : L1HandlerTransaction) :
    dst ↦ h ⊢ (structFieldRef L1HandlerTransaction go!"TransactionHash" dst ↦ h.TransactionHash' ∗
      (∀ value : Loc, structFieldRef L1HandlerTransaction go!"TransactionHash" dst ↦ value -∗
        dst ↦ {h with TransactionHash' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, Hcontract, Hentry, Hnonce, HcallData, Hversion, -⟩
  iframe Hhash
  iintro %value Hhash
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash Hcontract Hentry Hnonce HcallData Hversion

/-- Borrow the ContractAddress field while retaining every other concrete cell. -/
theorem l1Handler_contract_acc (dst : Loc) (h : L1HandlerTransaction) :
    dst ↦ h ⊢ (structFieldRef L1HandlerTransaction go!"ContractAddress" dst ↦ h.ContractAddress' ∗
      (∀ value : Loc, structFieldRef L1HandlerTransaction go!"ContractAddress" dst ↦ value -∗
        dst ↦ {h with ContractAddress' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, Hcontract, Hentry, Hnonce, HcallData, Hversion, -⟩
  iframe Hcontract
  iintro %value Hcontract
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash Hcontract Hentry Hnonce HcallData Hversion

/-- Borrow the EntryPointSelector field while retaining every other concrete cell. -/
theorem l1Handler_entry_acc (dst : Loc) (h : L1HandlerTransaction) :
    dst ↦ h ⊢ (structFieldRef L1HandlerTransaction go!"EntryPointSelector" dst ↦ h.EntryPointSelector' ∗
      (∀ value : Loc, structFieldRef L1HandlerTransaction go!"EntryPointSelector" dst ↦ value -∗
        dst ↦ {h with EntryPointSelector' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, Hcontract, Hentry, Hnonce, HcallData, Hversion, -⟩
  iframe Hentry
  iintro %value Hentry
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash Hcontract Hentry Hnonce HcallData Hversion

/-- Borrow the Nonce field while retaining every other concrete cell. -/
theorem l1Handler_nonce_acc (dst : Loc) (h : L1HandlerTransaction) :
    dst ↦ h ⊢ (structFieldRef L1HandlerTransaction go!"Nonce" dst ↦ h.Nonce' ∗
      (∀ value : Loc, structFieldRef L1HandlerTransaction go!"Nonce" dst ↦ value -∗
        dst ↦ {h with Nonce' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, Hcontract, Hentry, Hnonce, HcallData, Hversion, -⟩
  iframe Hnonce
  iintro %value Hnonce
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash Hcontract Hentry Hnonce HcallData Hversion

/-- Borrow the CallData field while retaining every other concrete cell. -/
theorem l1Handler_callData_acc (dst : Loc) (h : L1HandlerTransaction) :
    dst ↦ h ⊢ (structFieldRef L1HandlerTransaction go!"CallData" dst ↦ h.CallData' ∗
      (∀ value : GoSlice, structFieldRef L1HandlerTransaction go!"CallData" dst ↦ value -∗
        dst ↦ {h with CallData' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, Hcontract, Hentry, Hnonce, HcallData, Hversion, -⟩
  iframe HcallData
  iintro %value HcallData
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash Hcontract Hentry Hnonce HcallData Hversion

/-- Borrow the Version field while retaining every other concrete cell. -/
theorem l1Handler_version_acc (dst : Loc) (h : L1HandlerTransaction) :
    dst ↦ h ⊢ (structFieldRef L1HandlerTransaction go!"Version" dst ↦ h.Version' ∗
      (∀ value : Loc, structFieldRef L1HandlerTransaction go!"Version" dst ↦ value -∗
        dst ↦ {h with Version' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, Hcontract, Hentry, Hnonce, HcallData, Hversion, -⟩
  iframe Hversion
  iintro %value Hversion
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash Hcontract Hentry Hnonce HcallData Hversion

theorem l1HandlerChildren_hash (h : L1HandlerTransaction) (old : L1HandlerValue) (out : Loc) (value : Option (GoArray w64 4)) :
    L1HandlerChildren h old ∗ FeltResult out value ⊢
      (L1HandlerChildren {h with TransactionHash' := out} {old with hash := value} : IProp GF) := by
  unfold L1HandlerChildren
  iintro ⟨⟨_, Hcontract, Hentry, Hnonce, HcallData, Hversion⟩, Hvalue⟩
  iframe Hvalue Hcontract Hentry Hnonce HcallData Hversion

theorem l1HandlerChildren_contract (h : L1HandlerTransaction) (old : L1HandlerValue) (out : Loc) (value : Option (GoArray w64 4)) :
    L1HandlerChildren h old ∗ FeltResult out value ⊢
      (L1HandlerChildren {h with ContractAddress' := out} {old with contract := value} : IProp GF) := by
  unfold L1HandlerChildren
  iintro ⟨⟨Hhash, _, Hentry, Hnonce, HcallData, Hversion⟩, Hvalue⟩
  iframe Hhash Hvalue Hentry Hnonce HcallData Hversion

theorem l1HandlerChildren_entry (h : L1HandlerTransaction) (old : L1HandlerValue) (out : Loc) (value : Option (GoArray w64 4)) :
    L1HandlerChildren h old ∗ FeltResult out value ⊢
      (L1HandlerChildren {h with EntryPointSelector' := out} {old with entry := value} : IProp GF) := by
  unfold L1HandlerChildren
  iintro ⟨⟨Hhash, Hcontract, _, Hnonce, HcallData, Hversion⟩, Hvalue⟩
  iframe Hhash Hcontract Hvalue Hnonce HcallData Hversion

theorem l1HandlerChildren_nonce (h : L1HandlerTransaction) (old : L1HandlerValue) (out : Loc) (value : Option (GoArray w64 4)) :
    L1HandlerChildren h old ∗ FeltResult out value ⊢
      (L1HandlerChildren {h with Nonce' := out} {old with nonce := value} : IProp GF) := by
  unfold L1HandlerChildren
  iintro ⟨⟨Hhash, Hcontract, Hentry, _, HcallData, Hversion⟩, Hvalue⟩
  iframe Hhash Hcontract Hentry Hvalue HcallData Hversion

theorem l1HandlerChildren_callData (h : L1HandlerTransaction) (old : L1HandlerValue) (out : GoSlice) (value : Option (List (GoArray w64 4))) :
    L1HandlerChildren h old ∗ FeltValuesResult out value ⊢
      (L1HandlerChildren {h with CallData' := out} {old with callData := value} : IProp GF) := by
  unfold L1HandlerChildren
  iintro ⟨⟨Hhash, Hcontract, Hentry, Hnonce, _, Hversion⟩, Hvalue⟩
  iframe Hhash Hcontract Hentry Hnonce Hvalue Hversion

theorem l1HandlerChildren_version (h : L1HandlerTransaction) (old : L1HandlerValue) (out : Loc) (value : Option (GoArray w64 4)) :
    L1HandlerChildren h old ∗ FeltResult out value ⊢
      (L1HandlerChildren {h with Version' := out} {old with version := value} : IProp GF) := by
  unfold L1HandlerChildren
  iintro ⟨⟨Hhash, Hcontract, Hentry, Hnonce, HcallData, _⟩, Hvalue⟩
  iframe Hhash Hcontract Hentry Hnonce HcallData Hvalue

/-- The zero transaction owns no child allocations. -/
theorem l1HandlerChildren_zero : ⊢ (L1HandlerChildren (zero_val L1HandlerTransaction) l1HandlerZero : IProp GF) := by
  isimp only [L1HandlerChildren, l1HandlerZero, FeltResult, FeltValuesResult]
  ipureintro
  exact ⟨rfl, rfl, rfl, rfl, rfl, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.l1Handler_hash_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.l1Handler_contract_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.l1Handler_entry_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.l1Handler_nonce_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.l1Handler_callData_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.l1Handler_version_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerChildren_hash
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerChildren_contract
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerChildren_entry
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerChildren_nonce
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerChildren_callData
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerChildren_version
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerChildren_zero
