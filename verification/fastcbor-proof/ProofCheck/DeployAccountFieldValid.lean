import ProofCheck.DeployAccountOwnership
import ProofCheck.FeltValuesTypedRefine

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

def deployAccountFieldReader : DeployAccountField → GoString
  | .hash => cborReadFelt
  | .salt => cborReadFelt
  | .contract => cborReadFelt
  | .classHash => cborReadFelt
  | .constructorCallData => cborReadFeltValues
  | .version => cborReadTransactionVersion
  | .maxFee => cborReadFelt
  | .signature => cborReadFeltValues
  | .nonce => cborReadFelt
  | .resources => cborReadResourceBoundsMap
  | .tip => cborReadUint64
  | .paymaster => cborReadFeltValues
  | .nonceMode => cborReadDataAvailabilityMode
  | .feeMode => cborReadDataAvailabilityMode

/-- Every successful generated field call preserves all other deployAccount fields and
    owns its exact replacement, including nil/empty and nested allocation shape. -/
theorem wp_deployAccount_field_valid (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old final : DeployAccountValue) (field : DeployAccountField) (v : cursor.Cursor)
    (xs item rest : List w8) (dq : DFrac) (d : Nat)
    (valid : DeployAccountFieldValid d field old item rest final) (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cbortransactions ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       DeployAccountOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (deployAccountFieldReader field))) (Val #c)) (Val #(deployAccountFieldRef (GF := GF) field dst)))
    {{ RET #interface.nil; DeployAccountOwned dst final ∗
       c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  iintro %Φ ⟨#HdeployAccounts, #Hschema, Howned, Hc, Hs⟩ HΦ
  isimp only [DeployAccountOwned] at Howned
  icases Howned with ⟨%h, Hdst, Hchildren⟩
  ihave %Hscalar := deployAccountChildren_scalars h old $$ Hchildren
  cases valid with
  | @hash old item rest value valid =>
    simp only [deployAccountFieldReader, deployAccountFieldName, deployAccountFieldRef]
    icases deployAccount_hash_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFelt_valid c (structFieldRef DeployTransaction go!"TransactionHash" (structFieldRef DeployAccountTransaction go!"DeployTransaction" dst)) h.DeployTransaction'.TransactionHash' v xs item rest value dq d hd depth valid hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := deployAccountChildren_hash h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeployAccountOwned
      iexists {h with DeployTransaction' := {h.DeployTransaction' with TransactionHash' := out}}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @salt old item rest value valid =>
    simp only [deployAccountFieldReader, deployAccountFieldName, deployAccountFieldRef]
    icases deployAccount_salt_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFelt_valid c (structFieldRef DeployTransaction go!"ContractAddressSalt" (structFieldRef DeployAccountTransaction go!"DeployTransaction" dst)) h.DeployTransaction'.ContractAddressSalt' v xs item rest value dq d hd depth valid hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := deployAccountChildren_salt h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeployAccountOwned
      iexists {h with DeployTransaction' := {h.DeployTransaction' with ContractAddressSalt' := out}}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @contract old item rest value valid =>
    simp only [deployAccountFieldReader, deployAccountFieldName, deployAccountFieldRef]
    icases deployAccount_contract_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFelt_valid c (structFieldRef DeployTransaction go!"ContractAddress" (structFieldRef DeployAccountTransaction go!"DeployTransaction" dst)) h.DeployTransaction'.ContractAddress' v xs item rest value dq d hd depth valid hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := deployAccountChildren_contract h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeployAccountOwned
      iexists {h with DeployTransaction' := {h.DeployTransaction' with ContractAddress' := out}}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @classHash old item rest value valid =>
    simp only [deployAccountFieldReader, deployAccountFieldName, deployAccountFieldRef]
    icases deployAccount_classHash_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFelt_valid c (structFieldRef DeployTransaction go!"ClassHash" (structFieldRef DeployAccountTransaction go!"DeployTransaction" dst)) h.DeployTransaction'.ClassHash' v xs item rest value dq d hd depth valid hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := deployAccountChildren_classHash h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeployAccountOwned
      iexists {h with DeployTransaction' := {h.DeployTransaction' with ClassHash' := out}}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @constructorCallData old item rest value valid =>
    simp only [deployAccountFieldReader, deployAccountFieldName, deployAccountFieldRef]
    icases deployAccount_constructorCallData_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFeltValues_valid c (structFieldRef DeployTransaction go!"ConstructorCallData" (structFieldRef DeployAccountTransaction go!"DeployTransaction" dst)) h.DeployTransaction'.ConstructorCallData' v xs item rest value dq d valid hd depth hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := deployAccountChildren_constructorCallData h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeployAccountOwned
      iexists {h with DeployTransaction' := {h.DeployTransaction' with ConstructorCallData' := out}}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @version old item rest value valid =>
    simp only [deployAccountFieldReader, deployAccountFieldName, deployAccountFieldRef]
    icases deployAccount_version_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readTransactionVersion_valid c (structFieldRef DeployTransaction go!"Version" (structFieldRef DeployAccountTransaction go!"DeployTransaction" dst)) h.DeployTransaction'.Version' v xs item rest value dq d hd depth valid hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := deployAccountChildren_version h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeployAccountOwned
      iexists {h with DeployTransaction' := {h.DeployTransaction' with Version' := out}}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @maxFee old item rest value valid =>
    simp only [deployAccountFieldReader, deployAccountFieldName, deployAccountFieldRef]
    icases deployAccount_maxFee_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFelt_valid c (structFieldRef DeployAccountTransaction go!"MaxFee" dst) h.MaxFee' v xs item rest value dq d hd depth valid hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := deployAccountChildren_maxFee h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeployAccountOwned
      iexists {h with MaxFee' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @signature old item rest value valid =>
    simp only [deployAccountFieldReader, deployAccountFieldName, deployAccountFieldRef]
    icases deployAccount_signature_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFeltValues_valid c (structFieldRef DeployAccountTransaction go!"TransactionSignature" dst) h.TransactionSignature' v xs item rest value dq d valid hd depth hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := deployAccountChildren_signature h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeployAccountOwned
      iexists {h with TransactionSignature' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @nonce old item rest value valid =>
    simp only [deployAccountFieldReader, deployAccountFieldName, deployAccountFieldRef]
    icases deployAccount_nonce_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFelt_valid c (structFieldRef DeployAccountTransaction go!"Nonce" dst) h.Nonce' v xs item rest value dq d hd depth valid hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := deployAccountChildren_nonce h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeployAccountOwned
      iexists {h with Nonce' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @resources old item rest value valid =>
    simp only [deployAccountFieldReader, deployAccountFieldName, deployAccountFieldRef]
    icases deployAccount_resources_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readResourceMap_valid validator c (structFieldRef DeployAccountTransaction go!"ResourceBounds" dst) h.ResourceBounds' v xs (item ++ rest) rest value dq d valid hd hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    isimp only [List.length_append, Nat.add_sub_cancel] at Hc
    simp only [List.length_append, Nat.add_sub_cancel] at Hnext
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := deployAccountChildren_resources h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeployAccountOwned
      iexists {h with ResourceBounds' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @tip old item rest value valid =>
    simp only [deployAccountFieldReader, deployAccountFieldName, deployAccountFieldRef]
    icases deployAccount_tip_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    have hread := wp_readScalar_valid (GF := GF) false c (structFieldRef DeployAccountTransaction go!"Tip" dst) h.Tip' v xs item rest value dq (by simpa only [Hscalar.1] using valid) hp hbytes
    simp only [Bool.false_eq_true, ite_false] at hread
    wp_apply hread
      $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %value Hfield
    ihave Hchildren := deployAccountChildren_tip h old value $$ [Hchildren]
    · iframe Hchildren
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeployAccountOwned
      iexists {h with Tip' := value}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @paymaster old item rest value valid =>
    simp only [deployAccountFieldReader, deployAccountFieldName, deployAccountFieldRef]
    icases deployAccount_paymaster_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFeltValues_valid c (structFieldRef DeployAccountTransaction go!"PaymasterData" dst) h.PaymasterData' v xs item rest value dq d valid hd depth hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := deployAccountChildren_paymaster h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeployAccountOwned
      iexists {h with PaymasterData' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @nonceMode old item rest value valid =>
    simp only [deployAccountFieldReader, deployAccountFieldName, deployAccountFieldRef]
    icases deployAccount_nonceMode_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readDAMode_valid c (structFieldRef DeployAccountTransaction go!"NonceDAMode" dst) h.NonceDAMode' v xs item rest value dq (by simpa only [Hscalar.2.1] using valid) hp hbytes
      $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %value Hfield
    ihave Hchildren := deployAccountChildren_nonceMode h old value $$ [Hchildren]
    · iframe Hchildren
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeployAccountOwned
      iexists {h with NonceDAMode' := value}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @feeMode old item rest value valid =>
    simp only [deployAccountFieldReader, deployAccountFieldName, deployAccountFieldRef]
    icases deployAccount_feeMode_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readDAMode_valid c (structFieldRef DeployAccountTransaction go!"FeeDAMode" dst) h.FeeDAMode' v xs item rest value dq (by simpa only [Hscalar.2.2] using valid) hp hbytes
      $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %value Hfield
    ihave Hchildren := deployAccountChildren_feeMode h old value $$ [Hchildren]
    · iframe Hchildren
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeployAccountOwned
      iexists {h with FeeDAMode' := value}
      iframe Hdst Hchildren
    ipureintro; exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_deployAccount_field_valid
