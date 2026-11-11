import ProofCheck.DeployFieldComplete

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

def deployFieldReaderName : DeployField → GoString
  | .hash | .salt | .contract | .classHash => cborReadFelt
  | .constructorCallData => cborReadFeltValues
  | .version => cborReadTransactionVersion

/-- Uniform salt for the six actual field calls in the generated loop. -/
theorem wp_deploy_field_complete (c dst : Loc) (old : DeployValue) (field : DeployField) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       DeployOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (deployFieldReaderName field))) (Val #c)) (Val #(structFieldRef DeployTransaction (deployFieldName field) dst)))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
       DeployFieldOutcome c dst old v xs wire d field kinds result }} := by
  cases field with
  | hash => exact wp_deploy_hash_complete c dst old v xs wire dq d hd hdepth hp hbytes
  | salt => exact wp_deploy_salt_complete c dst old v xs wire dq d hd hdepth hp hbytes
  | contract => exact wp_deploy_contract_complete c dst old v xs wire dq d hd hdepth hp hbytes
  | classHash => exact wp_deploy_classHash_complete c dst old v xs wire dq d hd hdepth hp hbytes
  | constructorCallData => exact wp_deploy_constructorCallData_complete c dst old v xs wire dq d hd hdepth hp hbytes
  | version => exact wp_deploy_version_complete c dst old v xs wire dq d hd hdepth hp hbytes

/-- Independent successful field decoding determines the exact result of the
    actual call, including its contextual felt-slice guard and unread suffix. -/
theorem wp_deploy_field_valid (c dst : Loc) (old final : DeployValue) (field : DeployField) (v : cursor.Cursor)
    (xs item rem : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (valid : DeployFieldValid d field old item rem final)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       DeployOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (deployFieldReaderName field))) (Val #c)) (Val #(structFieldRef DeployTransaction (deployFieldName field) dst)))
    {{ RET #interface.nil;
       DeployOwned dst final ∗ c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  wp_apply +noauto wp_deploy_field_complete c dst old field v xs (item ++ rem) dq d hd hdepth hp hbytes
    $$ [Howned Hc Hs] as %result ⟨Hs, %kinds, Hglobals, Hout⟩
  · iframe Howned Hc Hs
  isimp only [DeployFieldOutcome] at Hout
  icases Hout with (Hsuccess | Hfailure)
  · icases Hsuccess with ⟨%actual, %rest, %out, Howned, Hc, %Hresult⟩
    obtain ⟨rfl, rfl, rfl⟩ := deployField_prefix_unique d field old final out item actual rem rest valid Hresult.2.2.1 Hresult.2.1
    rcases Hresult with ⟨rfl, _, _, Hnext⟩
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro; exact Hnext
  · icases Hfailure with ⟨%offset, %reason, %err, Howned, Hc, Herr, %Hresult⟩
    exact False.elim (deployField_not_rejected d field old final item rem offset reason valid Hresult.2.1)

/-- Independent rejection determines the owned diagnostic and original DeployTransaction;
    no field or child allocation is published on a failed call. -/
theorem wp_deploy_field_rejected (c dst : Loc) (old : DeployValue) (field : DeployField) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d offset : Nat) (reason : DeployFieldFailure)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (failure : DeployFieldRejected d field wire offset reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       DeployOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (deployFieldReaderName field))) (Val #c)) (Val #(structFieldRef DeployTransaction (deployFieldName field) dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       DeployOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗ DeployFieldFailureError err (v.pos' + W64 offset) reason kinds ∗
       ⌜sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  wp_apply +noauto wp_deploy_field_complete c dst old field v xs wire dq d hd hdepth hp hbytes
    $$ [Howned Hc Hs] as %result ⟨Hs, %kinds, Hglobals, Hout⟩
  · iframe Howned Hc Hs
  isimp only [DeployFieldOutcome] at Hout
  icases Hout with (Hsuccess | Hfailure)
  · icases Hsuccess with ⟨%item, %rest, %out, Howned, Hc, %Hresult⟩
    rw [Hresult.2.1] at failure
    exact False.elim (deployField_not_rejected d field old out item rest offset reason Hresult.2.2.1 failure)
  · icases Hfailure with ⟨%actual, %why, %err, Howned, Hc, Herr, %Hresult⟩
    obtain ⟨rfl, rfl⟩ := deployField_rejected_unique d field wire offset actual reason why failure Hresult.2.1
    rcases Hresult with ⟨rfl, _, Hnext⟩
    iapply HΦ
    iframe Howned Hc Hs
    iexists kinds
    iframe Hglobals Herr
    ipureintro; exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_deploy_field_complete
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_deploy_field_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_deploy_field_rejected
