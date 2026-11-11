import ProofCheck.InvokeValues
import ProofCheck.ResourceMapComplete
import ProofCheck.FeltValuesTyped
import ProofCheck.TransactionVersionRead
import ProofCheck.DAModeComplete
import ProofCheck.ScalarComplete

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

/-- Scalar cells agree with the independent value; each nullable child owns
    its exact allocation, including the records held inside the resource map. -/
def InvokeChildren (h : InvokeTransaction) (value : InvokeValue) : IProp GF := iprop(
  ⌜h.Tip' = value.tip ∧ h.NonceDAMode' = value.nonceMode ∧ h.FeeDAMode' = value.feeMode⌝ ∗
  FeltResult h.TransactionHash' value.hash ∗
  FeltValuesResult h.CallData' value.callData ∗
  FeltValuesResult h.TransactionSignature' value.signature ∗
  FeltResult h.MaxFee' value.maxFee ∗
  FeltResult h.ContractAddress' value.contract ∗
  FeltResult h.Version' value.version ∗
  FeltResult h.EntryPointSelector' value.entry ∗
  FeltResult h.Nonce' value.nonce ∗
  FeltResult h.SenderAddress' value.sender ∗
  ResourceMapResult h.ResourceBounds' value.resources ∗
  FeltValuesResult h.PaymasterData' value.paymaster ∗
  FeltValuesResult h.AccountDeploymentData' value.deployment ∗
  FeltValuesResult h.ProofFacts' value.proofFacts)

def InvokeOwned (dst : Loc) (value : InvokeValue) : IProp GF := iprop(
  ∃ h : InvokeTransaction, dst ↦ h ∗ InvokeChildren h value)

theorem invoke_hash_acc (dst : Loc) (h : InvokeTransaction) :
    dst ↦ h ⊢ (structFieldRef InvokeTransaction go!"TransactionHash" dst ↦ h.TransactionHash' ∗
      (∀ value : Loc, structFieldRef InvokeTransaction go!"TransactionHash" dst ↦ value -∗
        dst ↦ {h with TransactionHash' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, HcallData, Hsignature, HmaxFee, Hcontract, Hversion, Hentry, Hnonce, Hsender, Hresources, Htip, Hpaymaster, Hdeployment, HnonceMode, HfeeMode, HproofFacts, -⟩
  iframe Hhash
  iintro %value Hhash
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash HcallData Hsignature HmaxFee Hcontract Hversion Hentry Hnonce Hsender Hresources Htip Hpaymaster Hdeployment HnonceMode HfeeMode HproofFacts

theorem invoke_callData_acc (dst : Loc) (h : InvokeTransaction) :
    dst ↦ h ⊢ (structFieldRef InvokeTransaction go!"CallData" dst ↦ h.CallData' ∗
      (∀ value : GoSlice, structFieldRef InvokeTransaction go!"CallData" dst ↦ value -∗
        dst ↦ {h with CallData' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, HcallData, Hsignature, HmaxFee, Hcontract, Hversion, Hentry, Hnonce, Hsender, Hresources, Htip, Hpaymaster, Hdeployment, HnonceMode, HfeeMode, HproofFacts, -⟩
  iframe HcallData
  iintro %value HcallData
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash HcallData Hsignature HmaxFee Hcontract Hversion Hentry Hnonce Hsender Hresources Htip Hpaymaster Hdeployment HnonceMode HfeeMode HproofFacts

theorem invoke_signature_acc (dst : Loc) (h : InvokeTransaction) :
    dst ↦ h ⊢ (structFieldRef InvokeTransaction go!"TransactionSignature" dst ↦ h.TransactionSignature' ∗
      (∀ value : GoSlice, structFieldRef InvokeTransaction go!"TransactionSignature" dst ↦ value -∗
        dst ↦ {h with TransactionSignature' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, HcallData, Hsignature, HmaxFee, Hcontract, Hversion, Hentry, Hnonce, Hsender, Hresources, Htip, Hpaymaster, Hdeployment, HnonceMode, HfeeMode, HproofFacts, -⟩
  iframe Hsignature
  iintro %value Hsignature
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash HcallData Hsignature HmaxFee Hcontract Hversion Hentry Hnonce Hsender Hresources Htip Hpaymaster Hdeployment HnonceMode HfeeMode HproofFacts

theorem invoke_maxFee_acc (dst : Loc) (h : InvokeTransaction) :
    dst ↦ h ⊢ (structFieldRef InvokeTransaction go!"MaxFee" dst ↦ h.MaxFee' ∗
      (∀ value : Loc, structFieldRef InvokeTransaction go!"MaxFee" dst ↦ value -∗
        dst ↦ {h with MaxFee' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, HcallData, Hsignature, HmaxFee, Hcontract, Hversion, Hentry, Hnonce, Hsender, Hresources, Htip, Hpaymaster, Hdeployment, HnonceMode, HfeeMode, HproofFacts, -⟩
  iframe HmaxFee
  iintro %value HmaxFee
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash HcallData Hsignature HmaxFee Hcontract Hversion Hentry Hnonce Hsender Hresources Htip Hpaymaster Hdeployment HnonceMode HfeeMode HproofFacts

theorem invoke_contract_acc (dst : Loc) (h : InvokeTransaction) :
    dst ↦ h ⊢ (structFieldRef InvokeTransaction go!"ContractAddress" dst ↦ h.ContractAddress' ∗
      (∀ value : Loc, structFieldRef InvokeTransaction go!"ContractAddress" dst ↦ value -∗
        dst ↦ {h with ContractAddress' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, HcallData, Hsignature, HmaxFee, Hcontract, Hversion, Hentry, Hnonce, Hsender, Hresources, Htip, Hpaymaster, Hdeployment, HnonceMode, HfeeMode, HproofFacts, -⟩
  iframe Hcontract
  iintro %value Hcontract
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash HcallData Hsignature HmaxFee Hcontract Hversion Hentry Hnonce Hsender Hresources Htip Hpaymaster Hdeployment HnonceMode HfeeMode HproofFacts

theorem invoke_version_acc (dst : Loc) (h : InvokeTransaction) :
    dst ↦ h ⊢ (structFieldRef InvokeTransaction go!"Version" dst ↦ h.Version' ∗
      (∀ value : Loc, structFieldRef InvokeTransaction go!"Version" dst ↦ value -∗
        dst ↦ {h with Version' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, HcallData, Hsignature, HmaxFee, Hcontract, Hversion, Hentry, Hnonce, Hsender, Hresources, Htip, Hpaymaster, Hdeployment, HnonceMode, HfeeMode, HproofFacts, -⟩
  iframe Hversion
  iintro %value Hversion
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash HcallData Hsignature HmaxFee Hcontract Hversion Hentry Hnonce Hsender Hresources Htip Hpaymaster Hdeployment HnonceMode HfeeMode HproofFacts

theorem invoke_entry_acc (dst : Loc) (h : InvokeTransaction) :
    dst ↦ h ⊢ (structFieldRef InvokeTransaction go!"EntryPointSelector" dst ↦ h.EntryPointSelector' ∗
      (∀ value : Loc, structFieldRef InvokeTransaction go!"EntryPointSelector" dst ↦ value -∗
        dst ↦ {h with EntryPointSelector' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, HcallData, Hsignature, HmaxFee, Hcontract, Hversion, Hentry, Hnonce, Hsender, Hresources, Htip, Hpaymaster, Hdeployment, HnonceMode, HfeeMode, HproofFacts, -⟩
  iframe Hentry
  iintro %value Hentry
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash HcallData Hsignature HmaxFee Hcontract Hversion Hentry Hnonce Hsender Hresources Htip Hpaymaster Hdeployment HnonceMode HfeeMode HproofFacts

theorem invoke_nonce_acc (dst : Loc) (h : InvokeTransaction) :
    dst ↦ h ⊢ (structFieldRef InvokeTransaction go!"Nonce" dst ↦ h.Nonce' ∗
      (∀ value : Loc, structFieldRef InvokeTransaction go!"Nonce" dst ↦ value -∗
        dst ↦ {h with Nonce' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, HcallData, Hsignature, HmaxFee, Hcontract, Hversion, Hentry, Hnonce, Hsender, Hresources, Htip, Hpaymaster, Hdeployment, HnonceMode, HfeeMode, HproofFacts, -⟩
  iframe Hnonce
  iintro %value Hnonce
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash HcallData Hsignature HmaxFee Hcontract Hversion Hentry Hnonce Hsender Hresources Htip Hpaymaster Hdeployment HnonceMode HfeeMode HproofFacts

theorem invoke_sender_acc (dst : Loc) (h : InvokeTransaction) :
    dst ↦ h ⊢ (structFieldRef InvokeTransaction go!"SenderAddress" dst ↦ h.SenderAddress' ∗
      (∀ value : Loc, structFieldRef InvokeTransaction go!"SenderAddress" dst ↦ value -∗
        dst ↦ {h with SenderAddress' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, HcallData, Hsignature, HmaxFee, Hcontract, Hversion, Hentry, Hnonce, Hsender, Hresources, Htip, Hpaymaster, Hdeployment, HnonceMode, HfeeMode, HproofFacts, -⟩
  iframe Hsender
  iintro %value Hsender
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash HcallData Hsignature HmaxFee Hcontract Hversion Hentry Hnonce Hsender Hresources Htip Hpaymaster Hdeployment HnonceMode HfeeMode HproofFacts

theorem invoke_resources_acc (dst : Loc) (h : InvokeTransaction) :
    dst ↦ h ⊢ (structFieldRef InvokeTransaction go!"ResourceBounds" dst ↦ h.ResourceBounds' ∗
      (∀ value : Loc, structFieldRef InvokeTransaction go!"ResourceBounds" dst ↦ value -∗
        dst ↦ {h with ResourceBounds' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, HcallData, Hsignature, HmaxFee, Hcontract, Hversion, Hentry, Hnonce, Hsender, Hresources, Htip, Hpaymaster, Hdeployment, HnonceMode, HfeeMode, HproofFacts, -⟩
  iframe Hresources
  iintro %value Hresources
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash HcallData Hsignature HmaxFee Hcontract Hversion Hentry Hnonce Hsender Hresources Htip Hpaymaster Hdeployment HnonceMode HfeeMode HproofFacts

theorem invoke_tip_acc (dst : Loc) (h : InvokeTransaction) :
    dst ↦ h ⊢ (structFieldRef InvokeTransaction go!"Tip" dst ↦ h.Tip' ∗
      (∀ value : w64, structFieldRef InvokeTransaction go!"Tip" dst ↦ value -∗
        dst ↦ {h with Tip' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, HcallData, Hsignature, HmaxFee, Hcontract, Hversion, Hentry, Hnonce, Hsender, Hresources, Htip, Hpaymaster, Hdeployment, HnonceMode, HfeeMode, HproofFacts, -⟩
  iframe Htip
  iintro %value Htip
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash HcallData Hsignature HmaxFee Hcontract Hversion Hentry Hnonce Hsender Hresources Htip Hpaymaster Hdeployment HnonceMode HfeeMode HproofFacts

theorem invoke_paymaster_acc (dst : Loc) (h : InvokeTransaction) :
    dst ↦ h ⊢ (structFieldRef InvokeTransaction go!"PaymasterData" dst ↦ h.PaymasterData' ∗
      (∀ value : GoSlice, structFieldRef InvokeTransaction go!"PaymasterData" dst ↦ value -∗
        dst ↦ {h with PaymasterData' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, HcallData, Hsignature, HmaxFee, Hcontract, Hversion, Hentry, Hnonce, Hsender, Hresources, Htip, Hpaymaster, Hdeployment, HnonceMode, HfeeMode, HproofFacts, -⟩
  iframe Hpaymaster
  iintro %value Hpaymaster
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash HcallData Hsignature HmaxFee Hcontract Hversion Hentry Hnonce Hsender Hresources Htip Hpaymaster Hdeployment HnonceMode HfeeMode HproofFacts

theorem invoke_deployment_acc (dst : Loc) (h : InvokeTransaction) :
    dst ↦ h ⊢ (structFieldRef InvokeTransaction go!"AccountDeploymentData" dst ↦ h.AccountDeploymentData' ∗
      (∀ value : GoSlice, structFieldRef InvokeTransaction go!"AccountDeploymentData" dst ↦ value -∗
        dst ↦ {h with AccountDeploymentData' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, HcallData, Hsignature, HmaxFee, Hcontract, Hversion, Hentry, Hnonce, Hsender, Hresources, Htip, Hpaymaster, Hdeployment, HnonceMode, HfeeMode, HproofFacts, -⟩
  iframe Hdeployment
  iintro %value Hdeployment
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash HcallData Hsignature HmaxFee Hcontract Hversion Hentry Hnonce Hsender Hresources Htip Hpaymaster Hdeployment HnonceMode HfeeMode HproofFacts

theorem invoke_nonceMode_acc (dst : Loc) (h : InvokeTransaction) :
    dst ↦ h ⊢ (structFieldRef InvokeTransaction go!"NonceDAMode" dst ↦ h.NonceDAMode' ∗
      (∀ value : w32, structFieldRef InvokeTransaction go!"NonceDAMode" dst ↦ value -∗
        dst ↦ {h with NonceDAMode' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, HcallData, Hsignature, HmaxFee, Hcontract, Hversion, Hentry, Hnonce, Hsender, Hresources, Htip, Hpaymaster, Hdeployment, HnonceMode, HfeeMode, HproofFacts, -⟩
  iframe HnonceMode
  iintro %value HnonceMode
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash HcallData Hsignature HmaxFee Hcontract Hversion Hentry Hnonce Hsender Hresources Htip Hpaymaster Hdeployment HnonceMode HfeeMode HproofFacts

theorem invoke_feeMode_acc (dst : Loc) (h : InvokeTransaction) :
    dst ↦ h ⊢ (structFieldRef InvokeTransaction go!"FeeDAMode" dst ↦ h.FeeDAMode' ∗
      (∀ value : w32, structFieldRef InvokeTransaction go!"FeeDAMode" dst ↦ value -∗
        dst ↦ {h with FeeDAMode' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, HcallData, Hsignature, HmaxFee, Hcontract, Hversion, Hentry, Hnonce, Hsender, Hresources, Htip, Hpaymaster, Hdeployment, HnonceMode, HfeeMode, HproofFacts, -⟩
  iframe HfeeMode
  iintro %value HfeeMode
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash HcallData Hsignature HmaxFee Hcontract Hversion Hentry Hnonce Hsender Hresources Htip Hpaymaster Hdeployment HnonceMode HfeeMode HproofFacts

theorem invoke_proofFacts_acc (dst : Loc) (h : InvokeTransaction) :
    dst ↦ h ⊢ (structFieldRef InvokeTransaction go!"ProofFacts" dst ↦ h.ProofFacts' ∗
      (∀ value : GoSlice, structFieldRef InvokeTransaction go!"ProofFacts" dst ↦ value -∗
        dst ↦ {h with ProofFacts' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, HcallData, Hsignature, HmaxFee, Hcontract, Hversion, Hentry, Hnonce, Hsender, Hresources, Htip, Hpaymaster, Hdeployment, HnonceMode, HfeeMode, HproofFacts, -⟩
  iframe HproofFacts
  iintro %value HproofFacts
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash HcallData Hsignature HmaxFee Hcontract Hversion Hentry Hnonce Hsender Hresources Htip Hpaymaster Hdeployment HnonceMode HfeeMode HproofFacts

theorem invokeChildren_hash (h : InvokeTransaction) (old : InvokeValue) (out : Loc) (value : Option (GoArray w64 4)) :
    InvokeChildren h old ∗ FeltResult out value ⊢
      (InvokeChildren {h with TransactionHash' := out} {old with hash := value} : IProp GF) := by
  unfold InvokeChildren
  iintro ⟨⟨%Hscalar, _, HcallData, Hsignature, HmaxFee, Hcontract, Hversion, Hentry, Hnonce, Hsender, Hresources, Hpaymaster, Hdeployment, HproofFacts⟩, Hnew⟩
  iframe Hnew HcallData Hsignature HmaxFee Hcontract Hversion Hentry Hnonce Hsender Hresources Hpaymaster Hdeployment HproofFacts
  ipureintro; exact Hscalar

theorem invokeChildren_callData (h : InvokeTransaction) (old : InvokeValue) (out : GoSlice) (value : Option (List (GoArray w64 4))) :
    InvokeChildren h old ∗ FeltValuesResult out value ⊢
      (InvokeChildren {h with CallData' := out} {old with callData := value} : IProp GF) := by
  unfold InvokeChildren
  iintro ⟨⟨%Hscalar, Hhash, _, Hsignature, HmaxFee, Hcontract, Hversion, Hentry, Hnonce, Hsender, Hresources, Hpaymaster, Hdeployment, HproofFacts⟩, Hnew⟩
  iframe Hhash Hnew Hsignature HmaxFee Hcontract Hversion Hentry Hnonce Hsender Hresources Hpaymaster Hdeployment HproofFacts
  ipureintro; exact Hscalar

theorem invokeChildren_signature (h : InvokeTransaction) (old : InvokeValue) (out : GoSlice) (value : Option (List (GoArray w64 4))) :
    InvokeChildren h old ∗ FeltValuesResult out value ⊢
      (InvokeChildren {h with TransactionSignature' := out} {old with signature := value} : IProp GF) := by
  unfold InvokeChildren
  iintro ⟨⟨%Hscalar, Hhash, HcallData, _, HmaxFee, Hcontract, Hversion, Hentry, Hnonce, Hsender, Hresources, Hpaymaster, Hdeployment, HproofFacts⟩, Hnew⟩
  iframe Hhash HcallData Hnew HmaxFee Hcontract Hversion Hentry Hnonce Hsender Hresources Hpaymaster Hdeployment HproofFacts
  ipureintro; exact Hscalar

theorem invokeChildren_maxFee (h : InvokeTransaction) (old : InvokeValue) (out : Loc) (value : Option (GoArray w64 4)) :
    InvokeChildren h old ∗ FeltResult out value ⊢
      (InvokeChildren {h with MaxFee' := out} {old with maxFee := value} : IProp GF) := by
  unfold InvokeChildren
  iintro ⟨⟨%Hscalar, Hhash, HcallData, Hsignature, _, Hcontract, Hversion, Hentry, Hnonce, Hsender, Hresources, Hpaymaster, Hdeployment, HproofFacts⟩, Hnew⟩
  iframe Hhash HcallData Hsignature Hnew Hcontract Hversion Hentry Hnonce Hsender Hresources Hpaymaster Hdeployment HproofFacts
  ipureintro; exact Hscalar

theorem invokeChildren_contract (h : InvokeTransaction) (old : InvokeValue) (out : Loc) (value : Option (GoArray w64 4)) :
    InvokeChildren h old ∗ FeltResult out value ⊢
      (InvokeChildren {h with ContractAddress' := out} {old with contract := value} : IProp GF) := by
  unfold InvokeChildren
  iintro ⟨⟨%Hscalar, Hhash, HcallData, Hsignature, HmaxFee, _, Hversion, Hentry, Hnonce, Hsender, Hresources, Hpaymaster, Hdeployment, HproofFacts⟩, Hnew⟩
  iframe Hhash HcallData Hsignature HmaxFee Hnew Hversion Hentry Hnonce Hsender Hresources Hpaymaster Hdeployment HproofFacts
  ipureintro; exact Hscalar

theorem invokeChildren_version (h : InvokeTransaction) (old : InvokeValue) (out : Loc) (value : Option (GoArray w64 4)) :
    InvokeChildren h old ∗ FeltResult out value ⊢
      (InvokeChildren {h with Version' := out} {old with version := value} : IProp GF) := by
  unfold InvokeChildren
  iintro ⟨⟨%Hscalar, Hhash, HcallData, Hsignature, HmaxFee, Hcontract, _, Hentry, Hnonce, Hsender, Hresources, Hpaymaster, Hdeployment, HproofFacts⟩, Hnew⟩
  iframe Hhash HcallData Hsignature HmaxFee Hcontract Hnew Hentry Hnonce Hsender Hresources Hpaymaster Hdeployment HproofFacts
  ipureintro; exact Hscalar

theorem invokeChildren_entry (h : InvokeTransaction) (old : InvokeValue) (out : Loc) (value : Option (GoArray w64 4)) :
    InvokeChildren h old ∗ FeltResult out value ⊢
      (InvokeChildren {h with EntryPointSelector' := out} {old with entry := value} : IProp GF) := by
  unfold InvokeChildren
  iintro ⟨⟨%Hscalar, Hhash, HcallData, Hsignature, HmaxFee, Hcontract, Hversion, _, Hnonce, Hsender, Hresources, Hpaymaster, Hdeployment, HproofFacts⟩, Hnew⟩
  iframe Hhash HcallData Hsignature HmaxFee Hcontract Hversion Hnew Hnonce Hsender Hresources Hpaymaster Hdeployment HproofFacts
  ipureintro; exact Hscalar

theorem invokeChildren_nonce (h : InvokeTransaction) (old : InvokeValue) (out : Loc) (value : Option (GoArray w64 4)) :
    InvokeChildren h old ∗ FeltResult out value ⊢
      (InvokeChildren {h with Nonce' := out} {old with nonce := value} : IProp GF) := by
  unfold InvokeChildren
  iintro ⟨⟨%Hscalar, Hhash, HcallData, Hsignature, HmaxFee, Hcontract, Hversion, Hentry, _, Hsender, Hresources, Hpaymaster, Hdeployment, HproofFacts⟩, Hnew⟩
  iframe Hhash HcallData Hsignature HmaxFee Hcontract Hversion Hentry Hnew Hsender Hresources Hpaymaster Hdeployment HproofFacts
  ipureintro; exact Hscalar

theorem invokeChildren_sender (h : InvokeTransaction) (old : InvokeValue) (out : Loc) (value : Option (GoArray w64 4)) :
    InvokeChildren h old ∗ FeltResult out value ⊢
      (InvokeChildren {h with SenderAddress' := out} {old with sender := value} : IProp GF) := by
  unfold InvokeChildren
  iintro ⟨⟨%Hscalar, Hhash, HcallData, Hsignature, HmaxFee, Hcontract, Hversion, Hentry, Hnonce, _, Hresources, Hpaymaster, Hdeployment, HproofFacts⟩, Hnew⟩
  iframe Hhash HcallData Hsignature HmaxFee Hcontract Hversion Hentry Hnonce Hnew Hresources Hpaymaster Hdeployment HproofFacts
  ipureintro; exact Hscalar

theorem invokeChildren_resources (h : InvokeTransaction) (old : InvokeValue) (out : Loc) (value : Option (GMap w32 ResourceBoundsValue)) :
    InvokeChildren h old ∗ ResourceMapResult out value ⊢
      (InvokeChildren {h with ResourceBounds' := out} {old with resources := value} : IProp GF) := by
  unfold InvokeChildren
  iintro ⟨⟨%Hscalar, Hhash, HcallData, Hsignature, HmaxFee, Hcontract, Hversion, Hentry, Hnonce, Hsender, _, Hpaymaster, Hdeployment, HproofFacts⟩, Hnew⟩
  iframe Hhash HcallData Hsignature HmaxFee Hcontract Hversion Hentry Hnonce Hsender Hnew Hpaymaster Hdeployment HproofFacts
  ipureintro; exact Hscalar

theorem invokeChildren_paymaster (h : InvokeTransaction) (old : InvokeValue) (out : GoSlice) (value : Option (List (GoArray w64 4))) :
    InvokeChildren h old ∗ FeltValuesResult out value ⊢
      (InvokeChildren {h with PaymasterData' := out} {old with paymaster := value} : IProp GF) := by
  unfold InvokeChildren
  iintro ⟨⟨%Hscalar, Hhash, HcallData, Hsignature, HmaxFee, Hcontract, Hversion, Hentry, Hnonce, Hsender, Hresources, _, Hdeployment, HproofFacts⟩, Hnew⟩
  iframe Hhash HcallData Hsignature HmaxFee Hcontract Hversion Hentry Hnonce Hsender Hresources Hnew Hdeployment HproofFacts
  ipureintro; exact Hscalar

theorem invokeChildren_deployment (h : InvokeTransaction) (old : InvokeValue) (out : GoSlice) (value : Option (List (GoArray w64 4))) :
    InvokeChildren h old ∗ FeltValuesResult out value ⊢
      (InvokeChildren {h with AccountDeploymentData' := out} {old with deployment := value} : IProp GF) := by
  unfold InvokeChildren
  iintro ⟨⟨%Hscalar, Hhash, HcallData, Hsignature, HmaxFee, Hcontract, Hversion, Hentry, Hnonce, Hsender, Hresources, Hpaymaster, _, HproofFacts⟩, Hnew⟩
  iframe Hhash HcallData Hsignature HmaxFee Hcontract Hversion Hentry Hnonce Hsender Hresources Hpaymaster Hnew HproofFacts
  ipureintro; exact Hscalar

theorem invokeChildren_proofFacts (h : InvokeTransaction) (old : InvokeValue) (out : GoSlice) (value : Option (List (GoArray w64 4))) :
    InvokeChildren h old ∗ FeltValuesResult out value ⊢
      (InvokeChildren {h with ProofFacts' := out} {old with proofFacts := value} : IProp GF) := by
  unfold InvokeChildren
  iintro ⟨⟨%Hscalar, Hhash, HcallData, Hsignature, HmaxFee, Hcontract, Hversion, Hentry, Hnonce, Hsender, Hresources, Hpaymaster, Hdeployment, _⟩, Hnew⟩
  iframe Hhash HcallData Hsignature HmaxFee Hcontract Hversion Hentry Hnonce Hsender Hresources Hpaymaster Hdeployment Hnew
  ipureintro; exact Hscalar

theorem invokeChildren_tip (h : InvokeTransaction) (old : InvokeValue) (value : w64) :
    InvokeChildren h old ⊢ (InvokeChildren {h with Tip' := value} {old with tip := value} : IProp GF) := by
  unfold InvokeChildren
  iintro ⟨%Hscalar, Hhash, HcallData, Hsignature, HmaxFee, Hcontract, Hversion, Hentry, Hnonce, Hsender, Hresources, Hpaymaster, Hdeployment, HproofFacts⟩
  iframe Hhash HcallData Hsignature HmaxFee Hcontract Hversion Hentry Hnonce Hsender Hresources Hpaymaster Hdeployment HproofFacts
  ipureintro; exact ⟨rfl, Hscalar.2.1, Hscalar.2.2⟩

theorem invokeChildren_nonceMode (h : InvokeTransaction) (old : InvokeValue) (value : w32) :
    InvokeChildren h old ⊢ (InvokeChildren {h with NonceDAMode' := value} {old with nonceMode := value} : IProp GF) := by
  unfold InvokeChildren
  iintro ⟨%Hscalar, Hhash, HcallData, Hsignature, HmaxFee, Hcontract, Hversion, Hentry, Hnonce, Hsender, Hresources, Hpaymaster, Hdeployment, HproofFacts⟩
  iframe Hhash HcallData Hsignature HmaxFee Hcontract Hversion Hentry Hnonce Hsender Hresources Hpaymaster Hdeployment HproofFacts
  ipureintro; exact ⟨Hscalar.1, rfl, Hscalar.2.2⟩

theorem invokeChildren_feeMode (h : InvokeTransaction) (old : InvokeValue) (value : w32) :
    InvokeChildren h old ⊢ (InvokeChildren {h with FeeDAMode' := value} {old with feeMode := value} : IProp GF) := by
  unfold InvokeChildren
  iintro ⟨%Hscalar, Hhash, HcallData, Hsignature, HmaxFee, Hcontract, Hversion, Hentry, Hnonce, Hsender, Hresources, Hpaymaster, Hdeployment, HproofFacts⟩
  iframe Hhash HcallData Hsignature HmaxFee Hcontract Hversion Hentry Hnonce Hsender Hresources Hpaymaster Hdeployment HproofFacts
  ipureintro; exact ⟨Hscalar.1, Hscalar.2.1, rfl⟩

theorem invokeChildren_scalars (h : InvokeTransaction) (value : InvokeValue) :
    InvokeChildren h value ⊢ (⌜h.Tip' = value.tip ∧ h.NonceDAMode' = value.nonceMode ∧ h.FeeDAMode' = value.feeMode⌝ : IProp GF) := by
  unfold InvokeChildren
  iintro ⟨%Hscalar, _⟩
  ipureintro; exact Hscalar

theorem invokeChildren_zero : ⊢ (InvokeChildren (zero_val InvokeTransaction) invokeZero : IProp GF) := by
  isimp only [InvokeChildren, invokeZero, FeltResult, FeltValuesResult, ResourceMapResult]
  ipureintro
  exact ⟨⟨rfl, rfl, rfl⟩, rfl, rfl, rfl, rfl, rfl, rfl, rfl, rfl, rfl, rfl, rfl, rfl, rfl⟩

theorem invokeOwned_zero (dst : Loc) :
    dst ↦ zero_val InvokeTransaction ⊢ (InvokeOwned dst invokeZero : IProp GF) := by
  iintro Hdst
  unfold InvokeOwned
  iexists zero_val InvokeTransaction
  iframe Hdst
  iapply invokeChildren_zero

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.invoke_hash_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.invoke_callData_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.invoke_signature_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.invoke_maxFee_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.invoke_contract_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.invoke_version_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.invoke_entry_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.invoke_nonce_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.invoke_sender_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.invoke_resources_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.invoke_tip_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.invoke_paymaster_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.invoke_deployment_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.invoke_nonceMode_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.invoke_feeMode_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.invoke_proofFacts_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeChildren_hash
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeChildren_callData
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeChildren_signature
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeChildren_maxFee
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeChildren_contract
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeChildren_version
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeChildren_entry
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeChildren_nonce
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeChildren_sender
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeChildren_resources
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeChildren_paymaster
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeChildren_deployment
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeChildren_proofFacts
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeChildren_tip
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeChildren_nonceMode
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeChildren_feeMode
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeChildren_scalars
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeChildren_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeOwned_zero
