import ProofCheck.DeclareValues
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
def DeclareChildren (h : DeclareTransaction) (value : DeclareValue) : IProp GF := iprop(
  ⌜h.Tip' = value.tip ∧ h.NonceDAMode' = value.nonceMode ∧ h.FeeDAMode' = value.feeMode⌝ ∗
  FeltResult h.TransactionHash' value.hash ∗
  FeltResult h.ClassHash' value.classHash ∗
  FeltResult h.SenderAddress' value.sender ∗
  FeltResult h.MaxFee' value.maxFee ∗
  FeltValuesResult h.TransactionSignature' value.signature ∗
  FeltResult h.Nonce' value.nonce ∗
  FeltResult h.Version' value.version ∗
  FeltResult h.CompiledClassHash' value.compiledClassHash ∗
  ResourceMapResult h.ResourceBounds' value.resources ∗
  FeltValuesResult h.PaymasterData' value.paymaster ∗
  FeltValuesResult h.AccountDeploymentData' value.deployment)

def DeclareOwned (dst : Loc) (value : DeclareValue) : IProp GF := iprop(
  ∃ h : DeclareTransaction, dst ↦ h ∗ DeclareChildren h value)

theorem declare_hash_acc (dst : Loc) (h : DeclareTransaction) :
    dst ↦ h ⊢ (structFieldRef DeclareTransaction go!"TransactionHash" dst ↦ h.TransactionHash' ∗
      (∀ value : Loc, structFieldRef DeclareTransaction go!"TransactionHash" dst ↦ value -∗
        dst ↦ {h with TransactionHash' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, HclassHash, Hsender, HmaxFee, Hsignature, Hnonce, Hversion, HcompiledClassHash, Hresources, Htip, Hpaymaster, Hdeployment, HnonceMode, HfeeMode, -⟩
  iframe Hhash
  iintro %value Hhash
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash HclassHash Hsender HmaxFee Hsignature Hnonce Hversion HcompiledClassHash Hresources Htip Hpaymaster Hdeployment HnonceMode HfeeMode

theorem declare_classHash_acc (dst : Loc) (h : DeclareTransaction) :
    dst ↦ h ⊢ (structFieldRef DeclareTransaction go!"ClassHash" dst ↦ h.ClassHash' ∗
      (∀ value : Loc, structFieldRef DeclareTransaction go!"ClassHash" dst ↦ value -∗
        dst ↦ {h with ClassHash' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, HclassHash, Hsender, HmaxFee, Hsignature, Hnonce, Hversion, HcompiledClassHash, Hresources, Htip, Hpaymaster, Hdeployment, HnonceMode, HfeeMode, -⟩
  iframe HclassHash
  iintro %value HclassHash
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash HclassHash Hsender HmaxFee Hsignature Hnonce Hversion HcompiledClassHash Hresources Htip Hpaymaster Hdeployment HnonceMode HfeeMode

theorem declare_sender_acc (dst : Loc) (h : DeclareTransaction) :
    dst ↦ h ⊢ (structFieldRef DeclareTransaction go!"SenderAddress" dst ↦ h.SenderAddress' ∗
      (∀ value : Loc, structFieldRef DeclareTransaction go!"SenderAddress" dst ↦ value -∗
        dst ↦ {h with SenderAddress' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, HclassHash, Hsender, HmaxFee, Hsignature, Hnonce, Hversion, HcompiledClassHash, Hresources, Htip, Hpaymaster, Hdeployment, HnonceMode, HfeeMode, -⟩
  iframe Hsender
  iintro %value Hsender
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash HclassHash Hsender HmaxFee Hsignature Hnonce Hversion HcompiledClassHash Hresources Htip Hpaymaster Hdeployment HnonceMode HfeeMode

theorem declare_maxFee_acc (dst : Loc) (h : DeclareTransaction) :
    dst ↦ h ⊢ (structFieldRef DeclareTransaction go!"MaxFee" dst ↦ h.MaxFee' ∗
      (∀ value : Loc, structFieldRef DeclareTransaction go!"MaxFee" dst ↦ value -∗
        dst ↦ {h with MaxFee' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, HclassHash, Hsender, HmaxFee, Hsignature, Hnonce, Hversion, HcompiledClassHash, Hresources, Htip, Hpaymaster, Hdeployment, HnonceMode, HfeeMode, -⟩
  iframe HmaxFee
  iintro %value HmaxFee
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash HclassHash Hsender HmaxFee Hsignature Hnonce Hversion HcompiledClassHash Hresources Htip Hpaymaster Hdeployment HnonceMode HfeeMode

theorem declare_signature_acc (dst : Loc) (h : DeclareTransaction) :
    dst ↦ h ⊢ (structFieldRef DeclareTransaction go!"TransactionSignature" dst ↦ h.TransactionSignature' ∗
      (∀ value : GoSlice, structFieldRef DeclareTransaction go!"TransactionSignature" dst ↦ value -∗
        dst ↦ {h with TransactionSignature' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, HclassHash, Hsender, HmaxFee, Hsignature, Hnonce, Hversion, HcompiledClassHash, Hresources, Htip, Hpaymaster, Hdeployment, HnonceMode, HfeeMode, -⟩
  iframe Hsignature
  iintro %value Hsignature
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash HclassHash Hsender HmaxFee Hsignature Hnonce Hversion HcompiledClassHash Hresources Htip Hpaymaster Hdeployment HnonceMode HfeeMode

theorem declare_nonce_acc (dst : Loc) (h : DeclareTransaction) :
    dst ↦ h ⊢ (structFieldRef DeclareTransaction go!"Nonce" dst ↦ h.Nonce' ∗
      (∀ value : Loc, structFieldRef DeclareTransaction go!"Nonce" dst ↦ value -∗
        dst ↦ {h with Nonce' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, HclassHash, Hsender, HmaxFee, Hsignature, Hnonce, Hversion, HcompiledClassHash, Hresources, Htip, Hpaymaster, Hdeployment, HnonceMode, HfeeMode, -⟩
  iframe Hnonce
  iintro %value Hnonce
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash HclassHash Hsender HmaxFee Hsignature Hnonce Hversion HcompiledClassHash Hresources Htip Hpaymaster Hdeployment HnonceMode HfeeMode

theorem declare_version_acc (dst : Loc) (h : DeclareTransaction) :
    dst ↦ h ⊢ (structFieldRef DeclareTransaction go!"Version" dst ↦ h.Version' ∗
      (∀ value : Loc, structFieldRef DeclareTransaction go!"Version" dst ↦ value -∗
        dst ↦ {h with Version' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, HclassHash, Hsender, HmaxFee, Hsignature, Hnonce, Hversion, HcompiledClassHash, Hresources, Htip, Hpaymaster, Hdeployment, HnonceMode, HfeeMode, -⟩
  iframe Hversion
  iintro %value Hversion
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash HclassHash Hsender HmaxFee Hsignature Hnonce Hversion HcompiledClassHash Hresources Htip Hpaymaster Hdeployment HnonceMode HfeeMode

theorem declare_compiledClassHash_acc (dst : Loc) (h : DeclareTransaction) :
    dst ↦ h ⊢ (structFieldRef DeclareTransaction go!"CompiledClassHash" dst ↦ h.CompiledClassHash' ∗
      (∀ value : Loc, structFieldRef DeclareTransaction go!"CompiledClassHash" dst ↦ value -∗
        dst ↦ {h with CompiledClassHash' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, HclassHash, Hsender, HmaxFee, Hsignature, Hnonce, Hversion, HcompiledClassHash, Hresources, Htip, Hpaymaster, Hdeployment, HnonceMode, HfeeMode, -⟩
  iframe HcompiledClassHash
  iintro %value HcompiledClassHash
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash HclassHash Hsender HmaxFee Hsignature Hnonce Hversion HcompiledClassHash Hresources Htip Hpaymaster Hdeployment HnonceMode HfeeMode

theorem declare_resources_acc (dst : Loc) (h : DeclareTransaction) :
    dst ↦ h ⊢ (structFieldRef DeclareTransaction go!"ResourceBounds" dst ↦ h.ResourceBounds' ∗
      (∀ value : Loc, structFieldRef DeclareTransaction go!"ResourceBounds" dst ↦ value -∗
        dst ↦ {h with ResourceBounds' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, HclassHash, Hsender, HmaxFee, Hsignature, Hnonce, Hversion, HcompiledClassHash, Hresources, Htip, Hpaymaster, Hdeployment, HnonceMode, HfeeMode, -⟩
  iframe Hresources
  iintro %value Hresources
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash HclassHash Hsender HmaxFee Hsignature Hnonce Hversion HcompiledClassHash Hresources Htip Hpaymaster Hdeployment HnonceMode HfeeMode

theorem declare_tip_acc (dst : Loc) (h : DeclareTransaction) :
    dst ↦ h ⊢ (structFieldRef DeclareTransaction go!"Tip" dst ↦ h.Tip' ∗
      (∀ value : w64, structFieldRef DeclareTransaction go!"Tip" dst ↦ value -∗
        dst ↦ {h with Tip' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, HclassHash, Hsender, HmaxFee, Hsignature, Hnonce, Hversion, HcompiledClassHash, Hresources, Htip, Hpaymaster, Hdeployment, HnonceMode, HfeeMode, -⟩
  iframe Htip
  iintro %value Htip
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash HclassHash Hsender HmaxFee Hsignature Hnonce Hversion HcompiledClassHash Hresources Htip Hpaymaster Hdeployment HnonceMode HfeeMode

theorem declare_paymaster_acc (dst : Loc) (h : DeclareTransaction) :
    dst ↦ h ⊢ (structFieldRef DeclareTransaction go!"PaymasterData" dst ↦ h.PaymasterData' ∗
      (∀ value : GoSlice, structFieldRef DeclareTransaction go!"PaymasterData" dst ↦ value -∗
        dst ↦ {h with PaymasterData' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, HclassHash, Hsender, HmaxFee, Hsignature, Hnonce, Hversion, HcompiledClassHash, Hresources, Htip, Hpaymaster, Hdeployment, HnonceMode, HfeeMode, -⟩
  iframe Hpaymaster
  iintro %value Hpaymaster
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash HclassHash Hsender HmaxFee Hsignature Hnonce Hversion HcompiledClassHash Hresources Htip Hpaymaster Hdeployment HnonceMode HfeeMode

theorem declare_deployment_acc (dst : Loc) (h : DeclareTransaction) :
    dst ↦ h ⊢ (structFieldRef DeclareTransaction go!"AccountDeploymentData" dst ↦ h.AccountDeploymentData' ∗
      (∀ value : GoSlice, structFieldRef DeclareTransaction go!"AccountDeploymentData" dst ↦ value -∗
        dst ↦ {h with AccountDeploymentData' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, HclassHash, Hsender, HmaxFee, Hsignature, Hnonce, Hversion, HcompiledClassHash, Hresources, Htip, Hpaymaster, Hdeployment, HnonceMode, HfeeMode, -⟩
  iframe Hdeployment
  iintro %value Hdeployment
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash HclassHash Hsender HmaxFee Hsignature Hnonce Hversion HcompiledClassHash Hresources Htip Hpaymaster Hdeployment HnonceMode HfeeMode

theorem declare_nonceMode_acc (dst : Loc) (h : DeclareTransaction) :
    dst ↦ h ⊢ (structFieldRef DeclareTransaction go!"NonceDAMode" dst ↦ h.NonceDAMode' ∗
      (∀ value : w32, structFieldRef DeclareTransaction go!"NonceDAMode" dst ↦ value -∗
        dst ↦ {h with NonceDAMode' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, HclassHash, Hsender, HmaxFee, Hsignature, Hnonce, Hversion, HcompiledClassHash, Hresources, Htip, Hpaymaster, Hdeployment, HnonceMode, HfeeMode, -⟩
  iframe HnonceMode
  iintro %value HnonceMode
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash HclassHash Hsender HmaxFee Hsignature Hnonce Hversion HcompiledClassHash Hresources Htip Hpaymaster Hdeployment HnonceMode HfeeMode

theorem declare_feeMode_acc (dst : Loc) (h : DeclareTransaction) :
    dst ↦ h ⊢ (structFieldRef DeclareTransaction go!"FeeDAMode" dst ↦ h.FeeDAMode' ∗
      (∀ value : w32, structFieldRef DeclareTransaction go!"FeeDAMode" dst ↦ value -∗
        dst ↦ {h with FeeDAMode' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, HclassHash, Hsender, HmaxFee, Hsignature, Hnonce, Hversion, HcompiledClassHash, Hresources, Htip, Hpaymaster, Hdeployment, HnonceMode, HfeeMode, -⟩
  iframe HfeeMode
  iintro %value HfeeMode
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hhash HclassHash Hsender HmaxFee Hsignature Hnonce Hversion HcompiledClassHash Hresources Htip Hpaymaster Hdeployment HnonceMode HfeeMode

theorem declareChildren_hash (h : DeclareTransaction) (old : DeclareValue) (out : Loc) (value : Option (GoArray w64 4)) :
    DeclareChildren h old ∗ FeltResult out value ⊢
      (DeclareChildren {h with TransactionHash' := out} {old with hash := value} : IProp GF) := by
  unfold DeclareChildren
  iintro ⟨⟨%Hscalar, _, HclassHash, Hsender, HmaxFee, Hsignature, Hnonce, Hversion, HcompiledClassHash, Hresources, Hpaymaster, Hdeployment⟩, Hnew⟩
  iframe Hnew HclassHash Hsender HmaxFee Hsignature Hnonce Hversion HcompiledClassHash Hresources Hpaymaster Hdeployment
  ipureintro; exact Hscalar

theorem declareChildren_classHash (h : DeclareTransaction) (old : DeclareValue) (out : Loc) (value : Option (GoArray w64 4)) :
    DeclareChildren h old ∗ FeltResult out value ⊢
      (DeclareChildren {h with ClassHash' := out} {old with classHash := value} : IProp GF) := by
  unfold DeclareChildren
  iintro ⟨⟨%Hscalar, Hhash, _, Hsender, HmaxFee, Hsignature, Hnonce, Hversion, HcompiledClassHash, Hresources, Hpaymaster, Hdeployment⟩, Hnew⟩
  iframe Hhash Hnew Hsender HmaxFee Hsignature Hnonce Hversion HcompiledClassHash Hresources Hpaymaster Hdeployment
  ipureintro; exact Hscalar

theorem declareChildren_sender (h : DeclareTransaction) (old : DeclareValue) (out : Loc) (value : Option (GoArray w64 4)) :
    DeclareChildren h old ∗ FeltResult out value ⊢
      (DeclareChildren {h with SenderAddress' := out} {old with sender := value} : IProp GF) := by
  unfold DeclareChildren
  iintro ⟨⟨%Hscalar, Hhash, HclassHash, _, HmaxFee, Hsignature, Hnonce, Hversion, HcompiledClassHash, Hresources, Hpaymaster, Hdeployment⟩, Hnew⟩
  iframe Hhash HclassHash Hnew HmaxFee Hsignature Hnonce Hversion HcompiledClassHash Hresources Hpaymaster Hdeployment
  ipureintro; exact Hscalar

theorem declareChildren_maxFee (h : DeclareTransaction) (old : DeclareValue) (out : Loc) (value : Option (GoArray w64 4)) :
    DeclareChildren h old ∗ FeltResult out value ⊢
      (DeclareChildren {h with MaxFee' := out} {old with maxFee := value} : IProp GF) := by
  unfold DeclareChildren
  iintro ⟨⟨%Hscalar, Hhash, HclassHash, Hsender, _, Hsignature, Hnonce, Hversion, HcompiledClassHash, Hresources, Hpaymaster, Hdeployment⟩, Hnew⟩
  iframe Hhash HclassHash Hsender Hnew Hsignature Hnonce Hversion HcompiledClassHash Hresources Hpaymaster Hdeployment
  ipureintro; exact Hscalar

theorem declareChildren_signature (h : DeclareTransaction) (old : DeclareValue) (out : GoSlice) (value : Option (List (GoArray w64 4))) :
    DeclareChildren h old ∗ FeltValuesResult out value ⊢
      (DeclareChildren {h with TransactionSignature' := out} {old with signature := value} : IProp GF) := by
  unfold DeclareChildren
  iintro ⟨⟨%Hscalar, Hhash, HclassHash, Hsender, HmaxFee, _, Hnonce, Hversion, HcompiledClassHash, Hresources, Hpaymaster, Hdeployment⟩, Hnew⟩
  iframe Hhash HclassHash Hsender HmaxFee Hnew Hnonce Hversion HcompiledClassHash Hresources Hpaymaster Hdeployment
  ipureintro; exact Hscalar

theorem declareChildren_nonce (h : DeclareTransaction) (old : DeclareValue) (out : Loc) (value : Option (GoArray w64 4)) :
    DeclareChildren h old ∗ FeltResult out value ⊢
      (DeclareChildren {h with Nonce' := out} {old with nonce := value} : IProp GF) := by
  unfold DeclareChildren
  iintro ⟨⟨%Hscalar, Hhash, HclassHash, Hsender, HmaxFee, Hsignature, _, Hversion, HcompiledClassHash, Hresources, Hpaymaster, Hdeployment⟩, Hnew⟩
  iframe Hhash HclassHash Hsender HmaxFee Hsignature Hnew Hversion HcompiledClassHash Hresources Hpaymaster Hdeployment
  ipureintro; exact Hscalar

theorem declareChildren_version (h : DeclareTransaction) (old : DeclareValue) (out : Loc) (value : Option (GoArray w64 4)) :
    DeclareChildren h old ∗ FeltResult out value ⊢
      (DeclareChildren {h with Version' := out} {old with version := value} : IProp GF) := by
  unfold DeclareChildren
  iintro ⟨⟨%Hscalar, Hhash, HclassHash, Hsender, HmaxFee, Hsignature, Hnonce, _, HcompiledClassHash, Hresources, Hpaymaster, Hdeployment⟩, Hnew⟩
  iframe Hhash HclassHash Hsender HmaxFee Hsignature Hnonce Hnew HcompiledClassHash Hresources Hpaymaster Hdeployment
  ipureintro; exact Hscalar

theorem declareChildren_compiledClassHash (h : DeclareTransaction) (old : DeclareValue) (out : Loc) (value : Option (GoArray w64 4)) :
    DeclareChildren h old ∗ FeltResult out value ⊢
      (DeclareChildren {h with CompiledClassHash' := out} {old with compiledClassHash := value} : IProp GF) := by
  unfold DeclareChildren
  iintro ⟨⟨%Hscalar, Hhash, HclassHash, Hsender, HmaxFee, Hsignature, Hnonce, Hversion, _, Hresources, Hpaymaster, Hdeployment⟩, Hnew⟩
  iframe Hhash HclassHash Hsender HmaxFee Hsignature Hnonce Hversion Hnew Hresources Hpaymaster Hdeployment
  ipureintro; exact Hscalar

theorem declareChildren_resources (h : DeclareTransaction) (old : DeclareValue) (out : Loc) (value : Option (GMap w32 ResourceBoundsValue)) :
    DeclareChildren h old ∗ ResourceMapResult out value ⊢
      (DeclareChildren {h with ResourceBounds' := out} {old with resources := value} : IProp GF) := by
  unfold DeclareChildren
  iintro ⟨⟨%Hscalar, Hhash, HclassHash, Hsender, HmaxFee, Hsignature, Hnonce, Hversion, HcompiledClassHash, _, Hpaymaster, Hdeployment⟩, Hnew⟩
  iframe Hhash HclassHash Hsender HmaxFee Hsignature Hnonce Hversion HcompiledClassHash Hnew Hpaymaster Hdeployment
  ipureintro; exact Hscalar

theorem declareChildren_paymaster (h : DeclareTransaction) (old : DeclareValue) (out : GoSlice) (value : Option (List (GoArray w64 4))) :
    DeclareChildren h old ∗ FeltValuesResult out value ⊢
      (DeclareChildren {h with PaymasterData' := out} {old with paymaster := value} : IProp GF) := by
  unfold DeclareChildren
  iintro ⟨⟨%Hscalar, Hhash, HclassHash, Hsender, HmaxFee, Hsignature, Hnonce, Hversion, HcompiledClassHash, Hresources, _, Hdeployment⟩, Hnew⟩
  iframe Hhash HclassHash Hsender HmaxFee Hsignature Hnonce Hversion HcompiledClassHash Hresources Hnew Hdeployment
  ipureintro; exact Hscalar

theorem declareChildren_deployment (h : DeclareTransaction) (old : DeclareValue) (out : GoSlice) (value : Option (List (GoArray w64 4))) :
    DeclareChildren h old ∗ FeltValuesResult out value ⊢
      (DeclareChildren {h with AccountDeploymentData' := out} {old with deployment := value} : IProp GF) := by
  unfold DeclareChildren
  iintro ⟨⟨%Hscalar, Hhash, HclassHash, Hsender, HmaxFee, Hsignature, Hnonce, Hversion, HcompiledClassHash, Hresources, Hpaymaster, _⟩, Hnew⟩
  iframe Hhash HclassHash Hsender HmaxFee Hsignature Hnonce Hversion HcompiledClassHash Hresources Hpaymaster Hnew
  ipureintro; exact Hscalar

theorem declareChildren_tip (h : DeclareTransaction) (old : DeclareValue) (value : w64) :
    DeclareChildren h old ⊢ (DeclareChildren {h with Tip' := value} {old with tip := value} : IProp GF) := by
  unfold DeclareChildren
  iintro ⟨%Hscalar, Hhash, HclassHash, Hsender, HmaxFee, Hsignature, Hnonce, Hversion, HcompiledClassHash, Hresources, Hpaymaster, Hdeployment⟩
  iframe Hhash HclassHash Hsender HmaxFee Hsignature Hnonce Hversion HcompiledClassHash Hresources Hpaymaster Hdeployment
  ipureintro; exact ⟨rfl, Hscalar.2.1, Hscalar.2.2⟩

theorem declareChildren_nonceMode (h : DeclareTransaction) (old : DeclareValue) (value : w32) :
    DeclareChildren h old ⊢ (DeclareChildren {h with NonceDAMode' := value} {old with nonceMode := value} : IProp GF) := by
  unfold DeclareChildren
  iintro ⟨%Hscalar, Hhash, HclassHash, Hsender, HmaxFee, Hsignature, Hnonce, Hversion, HcompiledClassHash, Hresources, Hpaymaster, Hdeployment⟩
  iframe Hhash HclassHash Hsender HmaxFee Hsignature Hnonce Hversion HcompiledClassHash Hresources Hpaymaster Hdeployment
  ipureintro; exact ⟨Hscalar.1, rfl, Hscalar.2.2⟩

theorem declareChildren_feeMode (h : DeclareTransaction) (old : DeclareValue) (value : w32) :
    DeclareChildren h old ⊢ (DeclareChildren {h with FeeDAMode' := value} {old with feeMode := value} : IProp GF) := by
  unfold DeclareChildren
  iintro ⟨%Hscalar, Hhash, HclassHash, Hsender, HmaxFee, Hsignature, Hnonce, Hversion, HcompiledClassHash, Hresources, Hpaymaster, Hdeployment⟩
  iframe Hhash HclassHash Hsender HmaxFee Hsignature Hnonce Hversion HcompiledClassHash Hresources Hpaymaster Hdeployment
  ipureintro; exact ⟨Hscalar.1, Hscalar.2.1, rfl⟩

theorem declareChildren_scalars (h : DeclareTransaction) (value : DeclareValue) :
    DeclareChildren h value ⊢ (⌜h.Tip' = value.tip ∧ h.NonceDAMode' = value.nonceMode ∧ h.FeeDAMode' = value.feeMode⌝ : IProp GF) := by
  unfold DeclareChildren
  iintro ⟨%Hscalar, _⟩
  ipureintro; exact Hscalar

theorem declareChildren_zero : ⊢ (DeclareChildren (zero_val DeclareTransaction) declareZero : IProp GF) := by
  isimp only [DeclareChildren, declareZero, FeltResult, FeltValuesResult, ResourceMapResult]
  ipureintro
  exact ⟨⟨rfl, rfl, rfl⟩, rfl, rfl, rfl, rfl, rfl, rfl, rfl, rfl, rfl, rfl, rfl⟩

theorem declareOwned_zero (dst : Loc) :
    dst ↦ zero_val DeclareTransaction ⊢ (DeclareOwned dst declareZero : IProp GF) := by
  iintro Hdst
  unfold DeclareOwned
  iexists zero_val DeclareTransaction
  iframe Hdst
  iapply declareChildren_zero

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.declare_hash_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.declare_classHash_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.declare_sender_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.declare_maxFee_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.declare_signature_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.declare_nonce_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.declare_version_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.declare_compiledClassHash_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.declare_resources_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.declare_tip_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.declare_paymaster_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.declare_deployment_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.declare_nonceMode_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.declare_feeMode_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.declareChildren_hash
#print axioms Perennial.github_com.NethermindEth.juno.core.declareChildren_classHash
#print axioms Perennial.github_com.NethermindEth.juno.core.declareChildren_sender
#print axioms Perennial.github_com.NethermindEth.juno.core.declareChildren_maxFee
#print axioms Perennial.github_com.NethermindEth.juno.core.declareChildren_signature
#print axioms Perennial.github_com.NethermindEth.juno.core.declareChildren_nonce
#print axioms Perennial.github_com.NethermindEth.juno.core.declareChildren_version
#print axioms Perennial.github_com.NethermindEth.juno.core.declareChildren_compiledClassHash
#print axioms Perennial.github_com.NethermindEth.juno.core.declareChildren_resources
#print axioms Perennial.github_com.NethermindEth.juno.core.declareChildren_paymaster
#print axioms Perennial.github_com.NethermindEth.juno.core.declareChildren_deployment
#print axioms Perennial.github_com.NethermindEth.juno.core.declareChildren_tip
#print axioms Perennial.github_com.NethermindEth.juno.core.declareChildren_nonceMode
#print axioms Perennial.github_com.NethermindEth.juno.core.declareChildren_feeMode
#print axioms Perennial.github_com.NethermindEth.juno.core.declareChildren_scalars
#print axioms Perennial.github_com.NethermindEth.juno.core.declareChildren_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.declareOwned_zero
