import ProofCheck.InvokeOwnership
import ProofCheck.FeltValuesTypedRefine
import ProofCheck.PlainFeltValuesTypedRefine

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

def invokeFieldReader : InvokeField → GoString
  | .hash => cborReadFelt
  | .callData => cborReadFeltValues
  | .signature => cborReadFeltValues
  | .maxFee => cborReadFelt
  | .contract => cborReadFelt
  | .version => cborReadTransactionVersion
  | .entry => cborReadFelt
  | .nonce => cborReadFelt
  | .sender => cborReadFelt
  | .resources => cborReadResourceBoundsMap
  | .tip => cborReadUint64
  | .paymaster => cborReadFeltValues
  | .deployment => cborReadFeltValues
  | .nonceMode => cborReadDataAvailabilityMode
  | .feeMode => cborReadDataAvailabilityMode
  | .proofFacts => cborReadPlainFeltValues

/-- Every successful generated field call preserves all other invoke fields and
    owns its exact replacement, including nil/empty and nested allocation shape. -/
theorem wp_invoke_field_valid (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old final : InvokeValue) (field : InvokeField) (v : cursor.Cursor)
    (xs item rest : List w8) (dq : DFrac) (d : Nat)
    (valid : InvokeFieldValid d field old item rest final) (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cbortransactions ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       InvokeOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (invokeFieldReader field))) (Val #c)) (Val #(structFieldRef InvokeTransaction (invokeFieldName field) dst)))
    {{ RET #interface.nil; InvokeOwned dst final ∗
       c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  iintro %Φ ⟨#Hinvokes, #Hschema, Howned, Hc, Hs⟩ HΦ
  isimp only [InvokeOwned] at Howned
  icases Howned with ⟨%h, Hdst, Hchildren⟩
  ihave %Hscalar := invokeChildren_scalars h old $$ Hchildren
  cases valid with
  | @hash old item rest value valid =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_hash_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFelt_valid c (structFieldRef InvokeTransaction go!"TransactionHash" dst) h.TransactionHash' v xs item rest value dq d hd depth valid hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := invokeChildren_hash h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned
      iexists {h with TransactionHash' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @callData old item rest value valid =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_callData_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFeltValues_valid c (structFieldRef InvokeTransaction go!"CallData" dst) h.CallData' v xs item rest value dq d valid hd depth hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := invokeChildren_callData h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned
      iexists {h with CallData' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @signature old item rest value valid =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_signature_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFeltValues_valid c (structFieldRef InvokeTransaction go!"TransactionSignature" dst) h.TransactionSignature' v xs item rest value dq d valid hd depth hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := invokeChildren_signature h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned
      iexists {h with TransactionSignature' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @maxFee old item rest value valid =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_maxFee_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFelt_valid c (structFieldRef InvokeTransaction go!"MaxFee" dst) h.MaxFee' v xs item rest value dq d hd depth valid hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := invokeChildren_maxFee h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned
      iexists {h with MaxFee' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @contract old item rest value valid =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_contract_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFelt_valid c (structFieldRef InvokeTransaction go!"ContractAddress" dst) h.ContractAddress' v xs item rest value dq d hd depth valid hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := invokeChildren_contract h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned
      iexists {h with ContractAddress' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @version old item rest value valid =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_version_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readTransactionVersion_valid c (structFieldRef InvokeTransaction go!"Version" dst) h.Version' v xs item rest value dq d hd depth valid hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := invokeChildren_version h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned
      iexists {h with Version' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @entry old item rest value valid =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_entry_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFelt_valid c (structFieldRef InvokeTransaction go!"EntryPointSelector" dst) h.EntryPointSelector' v xs item rest value dq d hd depth valid hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := invokeChildren_entry h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned
      iexists {h with EntryPointSelector' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @nonce old item rest value valid =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_nonce_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFelt_valid c (structFieldRef InvokeTransaction go!"Nonce" dst) h.Nonce' v xs item rest value dq d hd depth valid hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := invokeChildren_nonce h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned
      iexists {h with Nonce' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @sender old item rest value valid =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_sender_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFelt_valid c (structFieldRef InvokeTransaction go!"SenderAddress" dst) h.SenderAddress' v xs item rest value dq d hd depth valid hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := invokeChildren_sender h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned
      iexists {h with SenderAddress' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @resources old item rest value valid =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_resources_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readResourceMap_valid validator c (structFieldRef InvokeTransaction go!"ResourceBounds" dst) h.ResourceBounds' v xs (item ++ rest) rest value dq d valid hd hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    isimp only [List.length_append, Nat.add_sub_cancel] at Hc
    simp only [List.length_append, Nat.add_sub_cancel] at Hnext
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := invokeChildren_resources h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned
      iexists {h with ResourceBounds' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @tip old item rest value valid =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_tip_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    have hread := wp_readScalar_valid (GF := GF) false c (structFieldRef InvokeTransaction go!"Tip" dst) h.Tip' v xs item rest value dq (by simpa only [Hscalar.1] using valid) hp hbytes
    simp only [Bool.false_eq_true, ite_false] at hread
    wp_apply hread
      $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %value Hfield
    ihave Hchildren := invokeChildren_tip h old value $$ [Hchildren]
    · iframe Hchildren
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned
      iexists {h with Tip' := value}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @paymaster old item rest value valid =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_paymaster_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFeltValues_valid c (structFieldRef InvokeTransaction go!"PaymasterData" dst) h.PaymasterData' v xs item rest value dq d valid hd depth hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := invokeChildren_paymaster h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned
      iexists {h with PaymasterData' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @deployment old item rest value valid =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_deployment_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFeltValues_valid c (structFieldRef InvokeTransaction go!"AccountDeploymentData" dst) h.AccountDeploymentData' v xs item rest value dq d valid hd depth hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := invokeChildren_deployment h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned
      iexists {h with AccountDeploymentData' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @nonceMode old item rest value valid =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_nonceMode_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readDAMode_valid c (structFieldRef InvokeTransaction go!"NonceDAMode" dst) h.NonceDAMode' v xs item rest value dq (by simpa only [Hscalar.2.1] using valid) hp hbytes
      $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %value Hfield
    ihave Hchildren := invokeChildren_nonceMode h old value $$ [Hchildren]
    · iframe Hchildren
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned
      iexists {h with NonceDAMode' := value}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @feeMode old item rest value valid =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_feeMode_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readDAMode_valid c (structFieldRef InvokeTransaction go!"FeeDAMode" dst) h.FeeDAMode' v xs item rest value dq (by simpa only [Hscalar.2.2] using valid) hp hbytes
      $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %value Hfield
    ihave Hchildren := invokeChildren_feeMode h old value $$ [Hchildren]
    · iframe Hchildren
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned
      iexists {h with FeeDAMode' := value}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @proofFacts old item rest value valid =>
    simp only [invokeFieldReader, invokeFieldName]
    icases invoke_proofFacts_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readPlainFeltValues_valid c (structFieldRef InvokeTransaction go!"ProofFacts" dst) h.ProofFacts' v xs item rest value dq d valid hd depth hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := invokeChildren_proofFacts h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold InvokeOwned
      iexists {h with ProofFacts' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_invoke_field_valid
