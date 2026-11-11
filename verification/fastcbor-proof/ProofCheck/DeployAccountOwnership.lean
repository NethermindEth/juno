import ProofCheck.DeployAccountValues
import ProofCheck.DeployOwnership
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

/-- The independent record is flat on the wire. Its concrete first six fields
    reside in the embedded DeployTransaction; each child owns its exact value. -/
def DeployAccountChildren (h : DeployAccountTransaction) (value : DeployAccountValue) : IProp GF := iprop(
  ⌜h.Tip' = value.tip ∧ h.NonceDAMode' = value.nonceMode ∧ h.FeeDAMode' = value.feeMode⌝ ∗
  FeltResult h.DeployTransaction'.TransactionHash' value.hash ∗
  FeltResult h.DeployTransaction'.ContractAddressSalt' value.salt ∗
  FeltResult h.DeployTransaction'.ContractAddress' value.contract ∗
  FeltResult h.DeployTransaction'.ClassHash' value.classHash ∗
  FeltValuesResult h.DeployTransaction'.ConstructorCallData' value.constructorCallData ∗
  FeltResult h.DeployTransaction'.Version' value.version ∗
  FeltResult h.MaxFee' value.maxFee ∗
  FeltValuesResult h.TransactionSignature' value.signature ∗
  FeltResult h.Nonce' value.nonce ∗
  ResourceMapResult h.ResourceBounds' value.resources ∗
  FeltValuesResult h.PaymasterData' value.paymaster)

def DeployAccountOwned (dst : Loc) (value : DeployAccountValue) : IProp GF := iprop(
  ∃ h : DeployAccountTransaction, dst ↦ h ∗ DeployAccountChildren h value)

/-- Actual nested or outer address used by each generated field call. -/
def deployAccountFieldRef (field : DeployAccountField) (dst : Loc) : Loc :=
  match field with
  | .hash => (structFieldRef DeployTransaction go!"TransactionHash" (structFieldRef DeployAccountTransaction go!"DeployTransaction" dst))
  | .salt => (structFieldRef DeployTransaction go!"ContractAddressSalt" (structFieldRef DeployAccountTransaction go!"DeployTransaction" dst))
  | .contract => (structFieldRef DeployTransaction go!"ContractAddress" (structFieldRef DeployAccountTransaction go!"DeployTransaction" dst))
  | .classHash => (structFieldRef DeployTransaction go!"ClassHash" (structFieldRef DeployAccountTransaction go!"DeployTransaction" dst))
  | .constructorCallData => (structFieldRef DeployTransaction go!"ConstructorCallData" (structFieldRef DeployAccountTransaction go!"DeployTransaction" dst))
  | .version => (structFieldRef DeployTransaction go!"Version" (structFieldRef DeployAccountTransaction go!"DeployTransaction" dst))
  | .maxFee => (structFieldRef DeployAccountTransaction go!"MaxFee" dst)
  | .signature => (structFieldRef DeployAccountTransaction go!"TransactionSignature" dst)
  | .nonce => (structFieldRef DeployAccountTransaction go!"Nonce" dst)
  | .resources => (structFieldRef DeployAccountTransaction go!"ResourceBounds" dst)
  | .tip => (structFieldRef DeployAccountTransaction go!"Tip" dst)
  | .paymaster => (structFieldRef DeployAccountTransaction go!"PaymasterData" dst)
  | .nonceMode => (structFieldRef DeployAccountTransaction go!"NonceDAMode" dst)
  | .feeMode => (structFieldRef DeployAccountTransaction go!"FeeDAMode" dst)

theorem deployAccount_embedded_acc (dst : Loc) (h : DeployAccountTransaction) :
    dst ↦ h ⊢ (structFieldRef DeployAccountTransaction go!"DeployTransaction" dst ↦ h.DeployTransaction' ∗
      (∀ value : DeployTransaction, structFieldRef DeployAccountTransaction go!"DeployTransaction" dst ↦ value -∗
        dst ↦ {h with DeployTransaction' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hembedded, HmaxFee, Hsignature, Hnonce, Hresources, Htip, Hpaymaster, HnonceMode, HfeeMode, -⟩
  iframe Hembedded
  iintro %value Hembedded
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hembedded HmaxFee Hsignature Hnonce Hresources Htip Hpaymaster HnonceMode HfeeMode

theorem deployAccount_maxFee_acc (dst : Loc) (h : DeployAccountTransaction) :
    dst ↦ h ⊢ (structFieldRef DeployAccountTransaction go!"MaxFee" dst ↦ h.MaxFee' ∗
      (∀ value : Loc, structFieldRef DeployAccountTransaction go!"MaxFee" dst ↦ value -∗
        dst ↦ {h with MaxFee' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hembedded, HmaxFee, Hsignature, Hnonce, Hresources, Htip, Hpaymaster, HnonceMode, HfeeMode, -⟩
  iframe HmaxFee
  iintro %value HmaxFee
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hembedded HmaxFee Hsignature Hnonce Hresources Htip Hpaymaster HnonceMode HfeeMode

theorem deployAccount_signature_acc (dst : Loc) (h : DeployAccountTransaction) :
    dst ↦ h ⊢ (structFieldRef DeployAccountTransaction go!"TransactionSignature" dst ↦ h.TransactionSignature' ∗
      (∀ value : GoSlice, structFieldRef DeployAccountTransaction go!"TransactionSignature" dst ↦ value -∗
        dst ↦ {h with TransactionSignature' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hembedded, HmaxFee, Hsignature, Hnonce, Hresources, Htip, Hpaymaster, HnonceMode, HfeeMode, -⟩
  iframe Hsignature
  iintro %value Hsignature
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hembedded HmaxFee Hsignature Hnonce Hresources Htip Hpaymaster HnonceMode HfeeMode

theorem deployAccount_nonce_acc (dst : Loc) (h : DeployAccountTransaction) :
    dst ↦ h ⊢ (structFieldRef DeployAccountTransaction go!"Nonce" dst ↦ h.Nonce' ∗
      (∀ value : Loc, structFieldRef DeployAccountTransaction go!"Nonce" dst ↦ value -∗
        dst ↦ {h with Nonce' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hembedded, HmaxFee, Hsignature, Hnonce, Hresources, Htip, Hpaymaster, HnonceMode, HfeeMode, -⟩
  iframe Hnonce
  iintro %value Hnonce
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hembedded HmaxFee Hsignature Hnonce Hresources Htip Hpaymaster HnonceMode HfeeMode

theorem deployAccount_resources_acc (dst : Loc) (h : DeployAccountTransaction) :
    dst ↦ h ⊢ (structFieldRef DeployAccountTransaction go!"ResourceBounds" dst ↦ h.ResourceBounds' ∗
      (∀ value : Loc, structFieldRef DeployAccountTransaction go!"ResourceBounds" dst ↦ value -∗
        dst ↦ {h with ResourceBounds' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hembedded, HmaxFee, Hsignature, Hnonce, Hresources, Htip, Hpaymaster, HnonceMode, HfeeMode, -⟩
  iframe Hresources
  iintro %value Hresources
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hembedded HmaxFee Hsignature Hnonce Hresources Htip Hpaymaster HnonceMode HfeeMode

theorem deployAccount_tip_acc (dst : Loc) (h : DeployAccountTransaction) :
    dst ↦ h ⊢ (structFieldRef DeployAccountTransaction go!"Tip" dst ↦ h.Tip' ∗
      (∀ value : w64, structFieldRef DeployAccountTransaction go!"Tip" dst ↦ value -∗
        dst ↦ {h with Tip' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hembedded, HmaxFee, Hsignature, Hnonce, Hresources, Htip, Hpaymaster, HnonceMode, HfeeMode, -⟩
  iframe Htip
  iintro %value Htip
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hembedded HmaxFee Hsignature Hnonce Hresources Htip Hpaymaster HnonceMode HfeeMode

theorem deployAccount_paymaster_acc (dst : Loc) (h : DeployAccountTransaction) :
    dst ↦ h ⊢ (structFieldRef DeployAccountTransaction go!"PaymasterData" dst ↦ h.PaymasterData' ∗
      (∀ value : GoSlice, structFieldRef DeployAccountTransaction go!"PaymasterData" dst ↦ value -∗
        dst ↦ {h with PaymasterData' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hembedded, HmaxFee, Hsignature, Hnonce, Hresources, Htip, Hpaymaster, HnonceMode, HfeeMode, -⟩
  iframe Hpaymaster
  iintro %value Hpaymaster
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hembedded HmaxFee Hsignature Hnonce Hresources Htip Hpaymaster HnonceMode HfeeMode

theorem deployAccount_nonceMode_acc (dst : Loc) (h : DeployAccountTransaction) :
    dst ↦ h ⊢ (structFieldRef DeployAccountTransaction go!"NonceDAMode" dst ↦ h.NonceDAMode' ∗
      (∀ value : w32, structFieldRef DeployAccountTransaction go!"NonceDAMode" dst ↦ value -∗
        dst ↦ {h with NonceDAMode' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hembedded, HmaxFee, Hsignature, Hnonce, Hresources, Htip, Hpaymaster, HnonceMode, HfeeMode, -⟩
  iframe HnonceMode
  iintro %value HnonceMode
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hembedded HmaxFee Hsignature Hnonce Hresources Htip Hpaymaster HnonceMode HfeeMode

theorem deployAccount_feeMode_acc (dst : Loc) (h : DeployAccountTransaction) :
    dst ↦ h ⊢ (structFieldRef DeployAccountTransaction go!"FeeDAMode" dst ↦ h.FeeDAMode' ∗
      (∀ value : w32, structFieldRef DeployAccountTransaction go!"FeeDAMode" dst ↦ value -∗
        dst ↦ {h with FeeDAMode' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hembedded, HmaxFee, Hsignature, Hnonce, Hresources, Htip, Hpaymaster, HnonceMode, HfeeMode, -⟩
  iframe HfeeMode
  iintro %value HfeeMode
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hembedded HmaxFee Hsignature Hnonce Hresources Htip Hpaymaster HnonceMode HfeeMode

/-- Borrow the TransactionHash cell inside the embedded record, framing both its
    siblings and every outer DeployAccount field. -/
theorem deployAccount_hash_acc (dst : Loc) (h : DeployAccountTransaction) :
    dst ↦ h ⊢ ((structFieldRef DeployTransaction go!"TransactionHash" (structFieldRef DeployAccountTransaction go!"DeployTransaction" dst)) ↦ h.DeployTransaction'.TransactionHash' ∗
      (∀ value : Loc, (structFieldRef DeployTransaction go!"TransactionHash" (structFieldRef DeployAccountTransaction go!"DeployTransaction" dst)) ↦ value -∗
        dst ↦ {h with DeployTransaction' := {h.DeployTransaction' with TransactionHash' := value}}) : IProp GF) := by
  iintro Hh
  icases deployAccount_embedded_acc dst h $$ Hh with ⟨Hembedded, Houter⟩
  icases deploy_hash_acc (structFieldRef DeployAccountTransaction go!"DeployTransaction" dst) h.DeployTransaction'
    $$ Hembedded with ⟨Hfield, Hinner⟩
  iframe Hfield
  iintro %value Hfield
  ihave Hembedded := Hinner $$ %value Hfield
  iapply Houter $$ %{h.DeployTransaction' with TransactionHash' := value} Hembedded

/-- Borrow the ContractAddressSalt cell inside the embedded record, framing both its
    siblings and every outer DeployAccount field. -/
theorem deployAccount_salt_acc (dst : Loc) (h : DeployAccountTransaction) :
    dst ↦ h ⊢ ((structFieldRef DeployTransaction go!"ContractAddressSalt" (structFieldRef DeployAccountTransaction go!"DeployTransaction" dst)) ↦ h.DeployTransaction'.ContractAddressSalt' ∗
      (∀ value : Loc, (structFieldRef DeployTransaction go!"ContractAddressSalt" (structFieldRef DeployAccountTransaction go!"DeployTransaction" dst)) ↦ value -∗
        dst ↦ {h with DeployTransaction' := {h.DeployTransaction' with ContractAddressSalt' := value}}) : IProp GF) := by
  iintro Hh
  icases deployAccount_embedded_acc dst h $$ Hh with ⟨Hembedded, Houter⟩
  icases deploy_salt_acc (structFieldRef DeployAccountTransaction go!"DeployTransaction" dst) h.DeployTransaction'
    $$ Hembedded with ⟨Hfield, Hinner⟩
  iframe Hfield
  iintro %value Hfield
  ihave Hembedded := Hinner $$ %value Hfield
  iapply Houter $$ %{h.DeployTransaction' with ContractAddressSalt' := value} Hembedded

/-- Borrow the ContractAddress cell inside the embedded record, framing both its
    siblings and every outer DeployAccount field. -/
theorem deployAccount_contract_acc (dst : Loc) (h : DeployAccountTransaction) :
    dst ↦ h ⊢ ((structFieldRef DeployTransaction go!"ContractAddress" (structFieldRef DeployAccountTransaction go!"DeployTransaction" dst)) ↦ h.DeployTransaction'.ContractAddress' ∗
      (∀ value : Loc, (structFieldRef DeployTransaction go!"ContractAddress" (structFieldRef DeployAccountTransaction go!"DeployTransaction" dst)) ↦ value -∗
        dst ↦ {h with DeployTransaction' := {h.DeployTransaction' with ContractAddress' := value}}) : IProp GF) := by
  iintro Hh
  icases deployAccount_embedded_acc dst h $$ Hh with ⟨Hembedded, Houter⟩
  icases deploy_contract_acc (structFieldRef DeployAccountTransaction go!"DeployTransaction" dst) h.DeployTransaction'
    $$ Hembedded with ⟨Hfield, Hinner⟩
  iframe Hfield
  iintro %value Hfield
  ihave Hembedded := Hinner $$ %value Hfield
  iapply Houter $$ %{h.DeployTransaction' with ContractAddress' := value} Hembedded

/-- Borrow the ClassHash cell inside the embedded record, framing both its
    siblings and every outer DeployAccount field. -/
theorem deployAccount_classHash_acc (dst : Loc) (h : DeployAccountTransaction) :
    dst ↦ h ⊢ ((structFieldRef DeployTransaction go!"ClassHash" (structFieldRef DeployAccountTransaction go!"DeployTransaction" dst)) ↦ h.DeployTransaction'.ClassHash' ∗
      (∀ value : Loc, (structFieldRef DeployTransaction go!"ClassHash" (structFieldRef DeployAccountTransaction go!"DeployTransaction" dst)) ↦ value -∗
        dst ↦ {h with DeployTransaction' := {h.DeployTransaction' with ClassHash' := value}}) : IProp GF) := by
  iintro Hh
  icases deployAccount_embedded_acc dst h $$ Hh with ⟨Hembedded, Houter⟩
  icases deploy_classHash_acc (structFieldRef DeployAccountTransaction go!"DeployTransaction" dst) h.DeployTransaction'
    $$ Hembedded with ⟨Hfield, Hinner⟩
  iframe Hfield
  iintro %value Hfield
  ihave Hembedded := Hinner $$ %value Hfield
  iapply Houter $$ %{h.DeployTransaction' with ClassHash' := value} Hembedded

/-- Borrow the ConstructorCallData cell inside the embedded record, framing both its
    siblings and every outer DeployAccount field. -/
theorem deployAccount_constructorCallData_acc (dst : Loc) (h : DeployAccountTransaction) :
    dst ↦ h ⊢ ((structFieldRef DeployTransaction go!"ConstructorCallData" (structFieldRef DeployAccountTransaction go!"DeployTransaction" dst)) ↦ h.DeployTransaction'.ConstructorCallData' ∗
      (∀ value : GoSlice, (structFieldRef DeployTransaction go!"ConstructorCallData" (structFieldRef DeployAccountTransaction go!"DeployTransaction" dst)) ↦ value -∗
        dst ↦ {h with DeployTransaction' := {h.DeployTransaction' with ConstructorCallData' := value}}) : IProp GF) := by
  iintro Hh
  icases deployAccount_embedded_acc dst h $$ Hh with ⟨Hembedded, Houter⟩
  icases deploy_constructorCallData_acc (structFieldRef DeployAccountTransaction go!"DeployTransaction" dst) h.DeployTransaction'
    $$ Hembedded with ⟨Hfield, Hinner⟩
  iframe Hfield
  iintro %value Hfield
  ihave Hembedded := Hinner $$ %value Hfield
  iapply Houter $$ %{h.DeployTransaction' with ConstructorCallData' := value} Hembedded

/-- Borrow the Version cell inside the embedded record, framing both its
    siblings and every outer DeployAccount field. -/
theorem deployAccount_version_acc (dst : Loc) (h : DeployAccountTransaction) :
    dst ↦ h ⊢ ((structFieldRef DeployTransaction go!"Version" (structFieldRef DeployAccountTransaction go!"DeployTransaction" dst)) ↦ h.DeployTransaction'.Version' ∗
      (∀ value : Loc, (structFieldRef DeployTransaction go!"Version" (structFieldRef DeployAccountTransaction go!"DeployTransaction" dst)) ↦ value -∗
        dst ↦ {h with DeployTransaction' := {h.DeployTransaction' with Version' := value}}) : IProp GF) := by
  iintro Hh
  icases deployAccount_embedded_acc dst h $$ Hh with ⟨Hembedded, Houter⟩
  icases deploy_version_acc (structFieldRef DeployAccountTransaction go!"DeployTransaction" dst) h.DeployTransaction'
    $$ Hembedded with ⟨Hfield, Hinner⟩
  iframe Hfield
  iintro %value Hfield
  ihave Hembedded := Hinner $$ %value Hfield
  iapply Houter $$ %{h.DeployTransaction' with Version' := value} Hembedded

theorem deployAccountChildren_hash (h : DeployAccountTransaction) (old : DeployAccountValue) (out : Loc) (value : Option (GoArray w64 4)) :
    DeployAccountChildren h old ∗ FeltResult out value ⊢
      (DeployAccountChildren {h with DeployTransaction' := {h.DeployTransaction' with TransactionHash' := out}} {old with hash := value} : IProp GF) := by
  unfold DeployAccountChildren
  iintro ⟨⟨%Hscalar, _, Hsalt, Hcontract, HclassHash, HconstructorCallData, Hversion, HmaxFee, Hsignature, Hnonce, Hresources, Hpaymaster⟩, Hnew⟩
  iframe Hnew Hsalt Hcontract HclassHash HconstructorCallData Hversion HmaxFee Hsignature Hnonce Hresources Hpaymaster
  ipureintro; exact Hscalar

theorem deployAccountChildren_salt (h : DeployAccountTransaction) (old : DeployAccountValue) (out : Loc) (value : Option (GoArray w64 4)) :
    DeployAccountChildren h old ∗ FeltResult out value ⊢
      (DeployAccountChildren {h with DeployTransaction' := {h.DeployTransaction' with ContractAddressSalt' := out}} {old with salt := value} : IProp GF) := by
  unfold DeployAccountChildren
  iintro ⟨⟨%Hscalar, Hhash, _, Hcontract, HclassHash, HconstructorCallData, Hversion, HmaxFee, Hsignature, Hnonce, Hresources, Hpaymaster⟩, Hnew⟩
  iframe Hhash Hnew Hcontract HclassHash HconstructorCallData Hversion HmaxFee Hsignature Hnonce Hresources Hpaymaster
  ipureintro; exact Hscalar

theorem deployAccountChildren_contract (h : DeployAccountTransaction) (old : DeployAccountValue) (out : Loc) (value : Option (GoArray w64 4)) :
    DeployAccountChildren h old ∗ FeltResult out value ⊢
      (DeployAccountChildren {h with DeployTransaction' := {h.DeployTransaction' with ContractAddress' := out}} {old with contract := value} : IProp GF) := by
  unfold DeployAccountChildren
  iintro ⟨⟨%Hscalar, Hhash, Hsalt, _, HclassHash, HconstructorCallData, Hversion, HmaxFee, Hsignature, Hnonce, Hresources, Hpaymaster⟩, Hnew⟩
  iframe Hhash Hsalt Hnew HclassHash HconstructorCallData Hversion HmaxFee Hsignature Hnonce Hresources Hpaymaster
  ipureintro; exact Hscalar

theorem deployAccountChildren_classHash (h : DeployAccountTransaction) (old : DeployAccountValue) (out : Loc) (value : Option (GoArray w64 4)) :
    DeployAccountChildren h old ∗ FeltResult out value ⊢
      (DeployAccountChildren {h with DeployTransaction' := {h.DeployTransaction' with ClassHash' := out}} {old with classHash := value} : IProp GF) := by
  unfold DeployAccountChildren
  iintro ⟨⟨%Hscalar, Hhash, Hsalt, Hcontract, _, HconstructorCallData, Hversion, HmaxFee, Hsignature, Hnonce, Hresources, Hpaymaster⟩, Hnew⟩
  iframe Hhash Hsalt Hcontract Hnew HconstructorCallData Hversion HmaxFee Hsignature Hnonce Hresources Hpaymaster
  ipureintro; exact Hscalar

theorem deployAccountChildren_constructorCallData (h : DeployAccountTransaction) (old : DeployAccountValue) (out : GoSlice) (value : Option (List (GoArray w64 4))) :
    DeployAccountChildren h old ∗ FeltValuesResult out value ⊢
      (DeployAccountChildren {h with DeployTransaction' := {h.DeployTransaction' with ConstructorCallData' := out}} {old with constructorCallData := value} : IProp GF) := by
  unfold DeployAccountChildren
  iintro ⟨⟨%Hscalar, Hhash, Hsalt, Hcontract, HclassHash, _, Hversion, HmaxFee, Hsignature, Hnonce, Hresources, Hpaymaster⟩, Hnew⟩
  iframe Hhash Hsalt Hcontract HclassHash Hnew Hversion HmaxFee Hsignature Hnonce Hresources Hpaymaster
  ipureintro; exact Hscalar

theorem deployAccountChildren_version (h : DeployAccountTransaction) (old : DeployAccountValue) (out : Loc) (value : Option (GoArray w64 4)) :
    DeployAccountChildren h old ∗ FeltResult out value ⊢
      (DeployAccountChildren {h with DeployTransaction' := {h.DeployTransaction' with Version' := out}} {old with version := value} : IProp GF) := by
  unfold DeployAccountChildren
  iintro ⟨⟨%Hscalar, Hhash, Hsalt, Hcontract, HclassHash, HconstructorCallData, _, HmaxFee, Hsignature, Hnonce, Hresources, Hpaymaster⟩, Hnew⟩
  iframe Hhash Hsalt Hcontract HclassHash HconstructorCallData Hnew HmaxFee Hsignature Hnonce Hresources Hpaymaster
  ipureintro; exact Hscalar

theorem deployAccountChildren_maxFee (h : DeployAccountTransaction) (old : DeployAccountValue) (out : Loc) (value : Option (GoArray w64 4)) :
    DeployAccountChildren h old ∗ FeltResult out value ⊢
      (DeployAccountChildren {h with MaxFee' := out} {old with maxFee := value} : IProp GF) := by
  unfold DeployAccountChildren
  iintro ⟨⟨%Hscalar, Hhash, Hsalt, Hcontract, HclassHash, HconstructorCallData, Hversion, _, Hsignature, Hnonce, Hresources, Hpaymaster⟩, Hnew⟩
  iframe Hhash Hsalt Hcontract HclassHash HconstructorCallData Hversion Hnew Hsignature Hnonce Hresources Hpaymaster
  ipureintro; exact Hscalar

theorem deployAccountChildren_signature (h : DeployAccountTransaction) (old : DeployAccountValue) (out : GoSlice) (value : Option (List (GoArray w64 4))) :
    DeployAccountChildren h old ∗ FeltValuesResult out value ⊢
      (DeployAccountChildren {h with TransactionSignature' := out} {old with signature := value} : IProp GF) := by
  unfold DeployAccountChildren
  iintro ⟨⟨%Hscalar, Hhash, Hsalt, Hcontract, HclassHash, HconstructorCallData, Hversion, HmaxFee, _, Hnonce, Hresources, Hpaymaster⟩, Hnew⟩
  iframe Hhash Hsalt Hcontract HclassHash HconstructorCallData Hversion HmaxFee Hnew Hnonce Hresources Hpaymaster
  ipureintro; exact Hscalar

theorem deployAccountChildren_nonce (h : DeployAccountTransaction) (old : DeployAccountValue) (out : Loc) (value : Option (GoArray w64 4)) :
    DeployAccountChildren h old ∗ FeltResult out value ⊢
      (DeployAccountChildren {h with Nonce' := out} {old with nonce := value} : IProp GF) := by
  unfold DeployAccountChildren
  iintro ⟨⟨%Hscalar, Hhash, Hsalt, Hcontract, HclassHash, HconstructorCallData, Hversion, HmaxFee, Hsignature, _, Hresources, Hpaymaster⟩, Hnew⟩
  iframe Hhash Hsalt Hcontract HclassHash HconstructorCallData Hversion HmaxFee Hsignature Hnew Hresources Hpaymaster
  ipureintro; exact Hscalar

theorem deployAccountChildren_resources (h : DeployAccountTransaction) (old : DeployAccountValue) (out : Loc) (value : Option (GMap w32 ResourceBoundsValue)) :
    DeployAccountChildren h old ∗ ResourceMapResult out value ⊢
      (DeployAccountChildren {h with ResourceBounds' := out} {old with resources := value} : IProp GF) := by
  unfold DeployAccountChildren
  iintro ⟨⟨%Hscalar, Hhash, Hsalt, Hcontract, HclassHash, HconstructorCallData, Hversion, HmaxFee, Hsignature, Hnonce, _, Hpaymaster⟩, Hnew⟩
  iframe Hhash Hsalt Hcontract HclassHash HconstructorCallData Hversion HmaxFee Hsignature Hnonce Hnew Hpaymaster
  ipureintro; exact Hscalar

theorem deployAccountChildren_paymaster (h : DeployAccountTransaction) (old : DeployAccountValue) (out : GoSlice) (value : Option (List (GoArray w64 4))) :
    DeployAccountChildren h old ∗ FeltValuesResult out value ⊢
      (DeployAccountChildren {h with PaymasterData' := out} {old with paymaster := value} : IProp GF) := by
  unfold DeployAccountChildren
  iintro ⟨⟨%Hscalar, Hhash, Hsalt, Hcontract, HclassHash, HconstructorCallData, Hversion, HmaxFee, Hsignature, Hnonce, Hresources, _⟩, Hnew⟩
  iframe Hhash Hsalt Hcontract HclassHash HconstructorCallData Hversion HmaxFee Hsignature Hnonce Hresources Hnew
  ipureintro; exact Hscalar

theorem deployAccountChildren_tip (h : DeployAccountTransaction) (old : DeployAccountValue) (value : w64) :
    DeployAccountChildren h old ⊢ (DeployAccountChildren {h with Tip' := value} {old with tip := value} : IProp GF) := by
  unfold DeployAccountChildren
  iintro ⟨%Hscalar, Hhash, Hsalt, Hcontract, HclassHash, HconstructorCallData, Hversion, HmaxFee, Hsignature, Hnonce, Hresources, Hpaymaster⟩
  iframe Hhash Hsalt Hcontract HclassHash HconstructorCallData Hversion HmaxFee Hsignature Hnonce Hresources Hpaymaster
  ipureintro; exact ⟨rfl, Hscalar.2.1, Hscalar.2.2⟩

theorem deployAccountChildren_nonceMode (h : DeployAccountTransaction) (old : DeployAccountValue) (value : w32) :
    DeployAccountChildren h old ⊢ (DeployAccountChildren {h with NonceDAMode' := value} {old with nonceMode := value} : IProp GF) := by
  unfold DeployAccountChildren
  iintro ⟨%Hscalar, Hhash, Hsalt, Hcontract, HclassHash, HconstructorCallData, Hversion, HmaxFee, Hsignature, Hnonce, Hresources, Hpaymaster⟩
  iframe Hhash Hsalt Hcontract HclassHash HconstructorCallData Hversion HmaxFee Hsignature Hnonce Hresources Hpaymaster
  ipureintro; exact ⟨Hscalar.1, rfl, Hscalar.2.2⟩

theorem deployAccountChildren_feeMode (h : DeployAccountTransaction) (old : DeployAccountValue) (value : w32) :
    DeployAccountChildren h old ⊢ (DeployAccountChildren {h with FeeDAMode' := value} {old with feeMode := value} : IProp GF) := by
  unfold DeployAccountChildren
  iintro ⟨%Hscalar, Hhash, Hsalt, Hcontract, HclassHash, HconstructorCallData, Hversion, HmaxFee, Hsignature, Hnonce, Hresources, Hpaymaster⟩
  iframe Hhash Hsalt Hcontract HclassHash HconstructorCallData Hversion HmaxFee Hsignature Hnonce Hresources Hpaymaster
  ipureintro; exact ⟨Hscalar.1, Hscalar.2.1, rfl⟩

theorem deployAccountChildren_scalars (h : DeployAccountTransaction) (value : DeployAccountValue) :
    DeployAccountChildren h value ⊢ (⌜h.Tip' = value.tip ∧ h.NonceDAMode' = value.nonceMode ∧ h.FeeDAMode' = value.feeMode⌝ : IProp GF) := by
  unfold DeployAccountChildren
  iintro ⟨%Hscalar, _⟩
  ipureintro; exact Hscalar

theorem deployAccountChildren_zero : ⊢ (DeployAccountChildren (zero_val DeployAccountTransaction) deployAccountZero : IProp GF) := by
  isimp only [DeployAccountChildren, deployAccountZero, FeltResult, FeltValuesResult, ResourceMapResult]
  ipureintro
  exact ⟨⟨rfl, rfl, rfl⟩, rfl, rfl, rfl, rfl, rfl, rfl, rfl, rfl, rfl, rfl, rfl⟩

theorem deployAccountOwned_zero (dst : Loc) :
    dst ↦ zero_val DeployAccountTransaction ⊢ (DeployAccountOwned dst deployAccountZero : IProp GF) := by
  iintro Hdst
  unfold DeployAccountOwned
  iexists zero_val DeployAccountTransaction
  iframe Hdst
  iapply deployAccountChildren_zero

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccount_embedded_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccount_maxFee_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccount_signature_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccount_nonce_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccount_resources_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccount_tip_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccount_paymaster_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccount_nonceMode_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccount_feeMode_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccount_hash_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccount_salt_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccount_contract_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccount_classHash_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccount_constructorCallData_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccount_version_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountChildren_hash
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountChildren_salt
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountChildren_contract
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountChildren_classHash
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountChildren_constructorCallData
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountChildren_version
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountChildren_maxFee
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountChildren_signature
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountChildren_nonce
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountChildren_resources
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountChildren_paymaster
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountChildren_tip
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountChildren_nonceMode
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountChildren_feeMode
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountChildren_scalars
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountChildren_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountOwned_zero
