import ProofCheck.DeclareOwnership
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

def declareFieldReader : DeclareField → GoString
  | .hash => cborReadFelt
  | .classHash => cborReadFelt
  | .sender => cborReadFelt
  | .maxFee => cborReadFelt
  | .signature => cborReadFeltValues
  | .nonce => cborReadFelt
  | .version => cborReadTransactionVersion
  | .compiledClassHash => cborReadFelt
  | .resources => cborReadResourceBoundsMap
  | .tip => cborReadUint64
  | .paymaster => cborReadFeltValues
  | .deployment => cborReadFeltValues
  | .nonceMode => cborReadDataAvailabilityMode
  | .feeMode => cborReadDataAvailabilityMode

/-- Every successful generated field call preserves all other declare fields and
    owns its exact replacement, including nil/empty and nested allocation shape. -/
theorem wp_declare_field_valid (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old final : DeclareValue) (field : DeclareField) (v : cursor.Cursor)
    (xs item rest : List w8) (dq : DFrac) (d : Nat)
    (valid : DeclareFieldValid d field old item rest final) (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cbortransactions ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       DeclareOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (declareFieldReader field))) (Val #c)) (Val #(structFieldRef DeclareTransaction (declareFieldName field) dst)))
    {{ RET #interface.nil; DeclareOwned dst final ∗
       c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  iintro %Φ ⟨#Hdeclares, #Hschema, Howned, Hc, Hs⟩ HΦ
  isimp only [DeclareOwned] at Howned
  icases Howned with ⟨%h, Hdst, Hchildren⟩
  ihave %Hscalar := declareChildren_scalars h old $$ Hchildren
  cases valid with
  | @hash old item rest value valid =>
    simp only [declareFieldReader, declareFieldName]
    icases declare_hash_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFelt_valid c (structFieldRef DeclareTransaction go!"TransactionHash" dst) h.TransactionHash' v xs item rest value dq d hd depth valid hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := declareChildren_hash h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeclareOwned
      iexists {h with TransactionHash' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @classHash old item rest value valid =>
    simp only [declareFieldReader, declareFieldName]
    icases declare_classHash_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFelt_valid c (structFieldRef DeclareTransaction go!"ClassHash" dst) h.ClassHash' v xs item rest value dq d hd depth valid hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := declareChildren_classHash h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeclareOwned
      iexists {h with ClassHash' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @sender old item rest value valid =>
    simp only [declareFieldReader, declareFieldName]
    icases declare_sender_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFelt_valid c (structFieldRef DeclareTransaction go!"SenderAddress" dst) h.SenderAddress' v xs item rest value dq d hd depth valid hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := declareChildren_sender h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeclareOwned
      iexists {h with SenderAddress' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @maxFee old item rest value valid =>
    simp only [declareFieldReader, declareFieldName]
    icases declare_maxFee_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFelt_valid c (structFieldRef DeclareTransaction go!"MaxFee" dst) h.MaxFee' v xs item rest value dq d hd depth valid hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := declareChildren_maxFee h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeclareOwned
      iexists {h with MaxFee' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @signature old item rest value valid =>
    simp only [declareFieldReader, declareFieldName]
    icases declare_signature_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFeltValues_valid c (structFieldRef DeclareTransaction go!"TransactionSignature" dst) h.TransactionSignature' v xs item rest value dq d valid hd depth hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := declareChildren_signature h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeclareOwned
      iexists {h with TransactionSignature' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @nonce old item rest value valid =>
    simp only [declareFieldReader, declareFieldName]
    icases declare_nonce_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFelt_valid c (structFieldRef DeclareTransaction go!"Nonce" dst) h.Nonce' v xs item rest value dq d hd depth valid hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := declareChildren_nonce h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeclareOwned
      iexists {h with Nonce' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @version old item rest value valid =>
    simp only [declareFieldReader, declareFieldName]
    icases declare_version_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readTransactionVersion_valid c (structFieldRef DeclareTransaction go!"Version" dst) h.Version' v xs item rest value dq d hd depth valid hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := declareChildren_version h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeclareOwned
      iexists {h with Version' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @compiledClassHash old item rest value valid =>
    simp only [declareFieldReader, declareFieldName]
    icases declare_compiledClassHash_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFelt_valid c (structFieldRef DeclareTransaction go!"CompiledClassHash" dst) h.CompiledClassHash' v xs item rest value dq d hd depth valid hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := declareChildren_compiledClassHash h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeclareOwned
      iexists {h with CompiledClassHash' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @resources old item rest value valid =>
    simp only [declareFieldReader, declareFieldName]
    icases declare_resources_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readResourceMap_valid validator c (structFieldRef DeclareTransaction go!"ResourceBounds" dst) h.ResourceBounds' v xs (item ++ rest) rest value dq d valid hd hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    isimp only [List.length_append, Nat.add_sub_cancel] at Hc
    simp only [List.length_append, Nat.add_sub_cancel] at Hnext
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := declareChildren_resources h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeclareOwned
      iexists {h with ResourceBounds' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @tip old item rest value valid =>
    simp only [declareFieldReader, declareFieldName]
    icases declare_tip_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    have hread := wp_readScalar_valid (GF := GF) false c (structFieldRef DeclareTransaction go!"Tip" dst) h.Tip' v xs item rest value dq (by simpa only [Hscalar.1] using valid) hp hbytes
    simp only [Bool.false_eq_true, ite_false] at hread
    wp_apply hread
      $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %value Hfield
    ihave Hchildren := declareChildren_tip h old value $$ [Hchildren]
    · iframe Hchildren
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeclareOwned
      iexists {h with Tip' := value}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @paymaster old item rest value valid =>
    simp only [declareFieldReader, declareFieldName]
    icases declare_paymaster_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFeltValues_valid c (structFieldRef DeclareTransaction go!"PaymasterData" dst) h.PaymasterData' v xs item rest value dq d valid hd depth hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := declareChildren_paymaster h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeclareOwned
      iexists {h with PaymasterData' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @deployment old item rest value valid =>
    simp only [declareFieldReader, declareFieldName]
    icases declare_deployment_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFeltValues_valid c (structFieldRef DeclareTransaction go!"AccountDeploymentData" dst) h.AccountDeploymentData' v xs item rest value dq d valid hd depth hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := declareChildren_deployment h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeclareOwned
      iexists {h with AccountDeploymentData' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @nonceMode old item rest value valid =>
    simp only [declareFieldReader, declareFieldName]
    icases declare_nonceMode_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readDAMode_valid c (structFieldRef DeclareTransaction go!"NonceDAMode" dst) h.NonceDAMode' v xs item rest value dq (by simpa only [Hscalar.2.1] using valid) hp hbytes
      $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %value Hfield
    ihave Hchildren := declareChildren_nonceMode h old value $$ [Hchildren]
    · iframe Hchildren
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeclareOwned
      iexists {h with NonceDAMode' := value}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @feeMode old item rest value valid =>
    simp only [declareFieldReader, declareFieldName]
    icases declare_feeMode_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readDAMode_valid c (structFieldRef DeclareTransaction go!"FeeDAMode" dst) h.FeeDAMode' v xs item rest value dq (by simpa only [Hscalar.2.2] using valid) hp hbytes
      $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %value Hfield
    ihave Hchildren := declareChildren_feeMode h old value $$ [Hchildren]
    · iframe Hchildren
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold DeclareOwned
      iexists {h with FeeDAMode' := value}
      iframe Hdst Hchildren
    ipureintro; exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_declare_field_valid
