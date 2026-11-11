import ProofCheck.DeployOwnership
import ProofCheck.TransactionVersionFeltOutcome

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 4096

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

local instance (kinds : FeltErrorKinds) : Persistent (FeltErrorGlobals (GF := GF) kinds) := by
  unfold FeltErrorGlobals SkipErrorGlobals
  infer_instance

def DeployFieldFailureError (err : Loc) (pos : w64) (reason : DeployFieldFailure) (kinds : FeltErrorKinds) : IProp GF :=
  match reason with
  | .felt reason => FeltFailureError err pos reason (feltFailureKind reason kinds)
  | .values reason => FeltValuesFailureError err pos reason kinds

def DeployFieldOutcome (c dst : Loc) (old : DeployValue) (v : cursor.Cursor) (xs wire : List w8)
    (d : Nat) (field : DeployField) (kinds : FeltErrorKinds) (result : GoInterface) : IProp GF := iprop(
  (∃ (item rest : List w8) (final : DeployValue), DeployOwned dst final ∗
    c ↦ {v with pos' := v.pos' + W64 item.length} ∗
    ⌜result = interface.nil ∧ wire = item ++ rest ∧ DeployFieldValid d field old item rest final ∧
      sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
      sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝) ∨
  (∃ (offset : Nat) (reason : DeployFieldFailure) (err : Loc), DeployOwned dst old ∗ c ↦ v ∗
    DeployFieldFailureError err (v.pos' + W64 offset) reason kinds ∗
    ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
      DeployFieldRejected d field wire offset reason ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝))


theorem wp_deploy_hash_complete (c dst : Loc) (old : DeployValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       DeployOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #(structFieldRef DeployTransaction go!"TransactionHash" dst)))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
       DeployFieldOutcome c dst old v xs wire d .hash kinds result }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  isimp only [DeployOwned] at Howned
  icases Howned with ⟨%h, Hh, Hchildren⟩
  icases deploy_hash_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
  ihave Hglobals : (∃ kinds : FeltErrorKinds, FeltErrorGlobals (GF := GF) kinds : IProp GF) $$ []
  · iapply header_initialized_felt_globals
    solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  wp_apply wp_readFelt_complete c (structFieldRef DeployTransaction go!"TransactionHash" dst) h.TransactionHash' v xs wire dq d kinds
    hd hdepth hp hbytes $$ [Hfield Hc Hs] as %result ⟨Hs, Houtcome⟩
  · iframe Hglobals Hfield Hc Hs
  isimp only [FeltOutcome] at Houtcome
  icases Houtcome with (Hsuccess | Hfailure)
  · icases Hsuccess with ⟨%item, %rest, %value, %p, Hfield, Hvalue, Hc, %Hresult⟩
    ihave Hh := Hclose $$ %p Hfield
    ihave Hchildren := deployChildren_hash h old p value $$ [Hchildren Hvalue]
    · iframe Hchildren Hvalue
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold DeployFieldOutcome
    ileft
    iexists item, rest, ({old with hash := value} : DeployValue)
    iframe Hc
    isplitl [Hh Hchildren]
    · unfold DeployOwned; iexists ({h with TransactionHash' := p} : DeployTransaction); iframe Hh Hchildren
    ipureintro
    exact ⟨Hresult.1, Hresult.2.1, .hash Hresult.2.2.1, Hresult.2.2.2⟩
  · icases Hfailure with ⟨%offset, %reason, %err, Hfield, Hc, Herr, %Hresult⟩
    ihave Hh := Hclose $$ %h.TransactionHash' Hfield
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold DeployFieldOutcome
    iright
    iexists offset, (DeployFieldFailure.felt reason), err
    isimp only [DeployFieldFailureError]
    iframe Hc Herr
    isplitl [Hh Hchildren]
    · unfold DeployOwned; iexists h; iframe Hh Hchildren
    ipureintro
    exact ⟨Hresult.1, .hash Hresult.2.1, Hresult.2.2⟩


theorem wp_deploy_salt_complete (c dst : Loc) (old : DeployValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       DeployOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #(structFieldRef DeployTransaction go!"ContractAddressSalt" dst)))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
       DeployFieldOutcome c dst old v xs wire d .salt kinds result }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  isimp only [DeployOwned] at Howned
  icases Howned with ⟨%h, Hh, Hchildren⟩
  icases deploy_salt_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
  ihave Hglobals : (∃ kinds : FeltErrorKinds, FeltErrorGlobals (GF := GF) kinds : IProp GF) $$ []
  · iapply header_initialized_felt_globals
    solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  wp_apply wp_readFelt_complete c (structFieldRef DeployTransaction go!"ContractAddressSalt" dst) h.ContractAddressSalt' v xs wire dq d kinds
    hd hdepth hp hbytes $$ [Hfield Hc Hs] as %result ⟨Hs, Houtcome⟩
  · iframe Hglobals Hfield Hc Hs
  isimp only [FeltOutcome] at Houtcome
  icases Houtcome with (Hsuccess | Hfailure)
  · icases Hsuccess with ⟨%item, %rest, %value, %p, Hfield, Hvalue, Hc, %Hresult⟩
    ihave Hh := Hclose $$ %p Hfield
    ihave Hchildren := deployChildren_salt h old p value $$ [Hchildren Hvalue]
    · iframe Hchildren Hvalue
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold DeployFieldOutcome
    ileft
    iexists item, rest, ({old with salt := value} : DeployValue)
    iframe Hc
    isplitl [Hh Hchildren]
    · unfold DeployOwned; iexists ({h with ContractAddressSalt' := p} : DeployTransaction); iframe Hh Hchildren
    ipureintro
    exact ⟨Hresult.1, Hresult.2.1, .salt Hresult.2.2.1, Hresult.2.2.2⟩
  · icases Hfailure with ⟨%offset, %reason, %err, Hfield, Hc, Herr, %Hresult⟩
    ihave Hh := Hclose $$ %h.ContractAddressSalt' Hfield
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold DeployFieldOutcome
    iright
    iexists offset, (DeployFieldFailure.felt reason), err
    isimp only [DeployFieldFailureError]
    iframe Hc Herr
    isplitl [Hh Hchildren]
    · unfold DeployOwned; iexists h; iframe Hh Hchildren
    ipureintro
    exact ⟨Hresult.1, .salt Hresult.2.1, Hresult.2.2⟩


theorem wp_deploy_contract_complete (c dst : Loc) (old : DeployValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       DeployOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #(structFieldRef DeployTransaction go!"ContractAddress" dst)))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
       DeployFieldOutcome c dst old v xs wire d .contract kinds result }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  isimp only [DeployOwned] at Howned
  icases Howned with ⟨%h, Hh, Hchildren⟩
  icases deploy_contract_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
  ihave Hglobals : (∃ kinds : FeltErrorKinds, FeltErrorGlobals (GF := GF) kinds : IProp GF) $$ []
  · iapply header_initialized_felt_globals
    solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  wp_apply wp_readFelt_complete c (structFieldRef DeployTransaction go!"ContractAddress" dst) h.ContractAddress' v xs wire dq d kinds
    hd hdepth hp hbytes $$ [Hfield Hc Hs] as %result ⟨Hs, Houtcome⟩
  · iframe Hglobals Hfield Hc Hs
  isimp only [FeltOutcome] at Houtcome
  icases Houtcome with (Hsuccess | Hfailure)
  · icases Hsuccess with ⟨%item, %rest, %value, %p, Hfield, Hvalue, Hc, %Hresult⟩
    ihave Hh := Hclose $$ %p Hfield
    ihave Hchildren := deployChildren_contract h old p value $$ [Hchildren Hvalue]
    · iframe Hchildren Hvalue
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold DeployFieldOutcome
    ileft
    iexists item, rest, ({old with contract := value} : DeployValue)
    iframe Hc
    isplitl [Hh Hchildren]
    · unfold DeployOwned; iexists ({h with ContractAddress' := p} : DeployTransaction); iframe Hh Hchildren
    ipureintro
    exact ⟨Hresult.1, Hresult.2.1, .contract Hresult.2.2.1, Hresult.2.2.2⟩
  · icases Hfailure with ⟨%offset, %reason, %err, Hfield, Hc, Herr, %Hresult⟩
    ihave Hh := Hclose $$ %h.ContractAddress' Hfield
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold DeployFieldOutcome
    iright
    iexists offset, (DeployFieldFailure.felt reason), err
    isimp only [DeployFieldFailureError]
    iframe Hc Herr
    isplitl [Hh Hchildren]
    · unfold DeployOwned; iexists h; iframe Hh Hchildren
    ipureintro
    exact ⟨Hresult.1, .contract Hresult.2.1, Hresult.2.2⟩


theorem wp_deploy_classHash_complete (c dst : Loc) (old : DeployValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       DeployOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #(structFieldRef DeployTransaction go!"ClassHash" dst)))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
       DeployFieldOutcome c dst old v xs wire d .classHash kinds result }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  isimp only [DeployOwned] at Howned
  icases Howned with ⟨%h, Hh, Hchildren⟩
  icases deploy_classHash_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
  ihave Hglobals : (∃ kinds : FeltErrorKinds, FeltErrorGlobals (GF := GF) kinds : IProp GF) $$ []
  · iapply header_initialized_felt_globals
    solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  wp_apply wp_readFelt_complete c (structFieldRef DeployTransaction go!"ClassHash" dst) h.ClassHash' v xs wire dq d kinds
    hd hdepth hp hbytes $$ [Hfield Hc Hs] as %result ⟨Hs, Houtcome⟩
  · iframe Hglobals Hfield Hc Hs
  isimp only [FeltOutcome] at Houtcome
  icases Houtcome with (Hsuccess | Hfailure)
  · icases Hsuccess with ⟨%item, %rest, %value, %p, Hfield, Hvalue, Hc, %Hresult⟩
    ihave Hh := Hclose $$ %p Hfield
    ihave Hchildren := deployChildren_classHash h old p value $$ [Hchildren Hvalue]
    · iframe Hchildren Hvalue
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold DeployFieldOutcome
    ileft
    iexists item, rest, ({old with classHash := value} : DeployValue)
    iframe Hc
    isplitl [Hh Hchildren]
    · unfold DeployOwned; iexists ({h with ClassHash' := p} : DeployTransaction); iframe Hh Hchildren
    ipureintro
    exact ⟨Hresult.1, Hresult.2.1, .classHash Hresult.2.2.1, Hresult.2.2.2⟩
  · icases Hfailure with ⟨%offset, %reason, %err, Hfield, Hc, Herr, %Hresult⟩
    ihave Hh := Hclose $$ %h.ClassHash' Hfield
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold DeployFieldOutcome
    iright
    iexists offset, (DeployFieldFailure.felt reason), err
    isimp only [DeployFieldFailureError]
    iframe Hc Herr
    isplitl [Hh Hchildren]
    · unfold DeployOwned; iexists h; iframe Hh Hchildren
    ipureintro
    exact ⟨Hresult.1, .classHash Hresult.2.1, Hresult.2.2⟩


theorem wp_deploy_version_complete (c dst : Loc) (old : DeployValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       DeployOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadTransactionVersion)) (Val #c)) (Val #(structFieldRef DeployTransaction go!"Version" dst)))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
       DeployFieldOutcome c dst old v xs wire d .version kinds result }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  isimp only [DeployOwned] at Howned
  icases Howned with ⟨%h, Hh, Hchildren⟩
  icases deploy_version_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
  ihave Hglobals : (∃ kinds : FeltErrorKinds, FeltErrorGlobals (GF := GF) kinds : IProp GF) $$ []
  · iapply header_initialized_felt_globals
    solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  wp_apply wp_readTransactionVersion_felt_outcome c (structFieldRef DeployTransaction go!"Version" dst) h.Version' v xs wire dq d kinds
    hd hdepth hp hbytes $$ [Hfield Hc Hs] as %result ⟨Hs, Houtcome⟩
  · iframe Hglobals Hfield Hc Hs
  isimp only [FeltOutcome] at Houtcome
  icases Houtcome with (Hsuccess | Hfailure)
  · icases Hsuccess with ⟨%item, %rest, %value, %p, Hfield, Hvalue, Hc, %Hresult⟩
    ihave Hh := Hclose $$ %p Hfield
    ihave Hchildren := deployChildren_version h old p value $$ [Hchildren Hvalue]
    · iframe Hchildren Hvalue
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold DeployFieldOutcome
    ileft
    iexists item, rest, ({old with version := value} : DeployValue)
    iframe Hc
    isplitl [Hh Hchildren]
    · unfold DeployOwned; iexists ({h with Version' := p} : DeployTransaction); iframe Hh Hchildren
    ipureintro
    exact ⟨Hresult.1, Hresult.2.1, .version Hresult.2.2.1, Hresult.2.2.2⟩
  · icases Hfailure with ⟨%offset, %reason, %err, Hfield, Hc, Herr, %Hresult⟩
    ihave Hh := Hclose $$ %h.Version' Hfield
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold DeployFieldOutcome
    iright
    iexists offset, (DeployFieldFailure.felt reason), err
    isimp only [DeployFieldFailureError]
    iframe Hc Herr
    isplitl [Hh Hchildren]
    · unfold DeployOwned; iexists h; iframe Hh Hchildren
    ipureintro
    exact ⟨Hresult.1, .version Hresult.2.1, Hresult.2.2⟩

theorem wp_deploy_constructorCallData_complete (c dst : Loc) (old : DeployValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       DeployOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltValues)) (Val #c)) (Val #(structFieldRef DeployTransaction go!"ConstructorCallData" dst)))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
       DeployFieldOutcome c dst old v xs wire d .constructorCallData kinds result }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  isimp only [DeployOwned] at Howned
  icases Howned with ⟨%h, Hh, Hchildren⟩
  icases deploy_constructorCallData_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
  wp_apply wp_readFeltValues_complete c (structFieldRef DeployTransaction go!"ConstructorCallData" dst)
    h.ConstructorCallData' v xs wire dq d hd hdepth hp hbytes $$ [Hfield Hc Hs]
    as %result ⟨Hs, %outcome, %kinds, Hglobals, %Hspec, Houtcome⟩
  · iframe Hfield Hc Hs
  cases outcome with
  | success value rest =>
    isimp only [FeltValuesReadOwned, FeltValuesSuccess] at Houtcome
    icases Houtcome with ⟨%Hnil, %out, Hfield, Hvalue, %item, Hc, %Hnext⟩
    ihave Hh := Hclose $$ %out Hfield
    ihave Hchildren := deployChildren_constructorCallData h old out value $$ [Hchildren Hvalue]
    · iframe Hchildren Hvalue
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold DeployFieldOutcome
    ileft
    iexists item, rest, ({old with constructorCallData := value} : DeployValue)
    iframe Hc
    isplitl [Hh Hchildren]
    · unfold DeployOwned; iexists ({h with ConstructorCallData' := out} : DeployTransaction); iframe Hh Hchildren
    ipureintro
    refine ⟨Hnil, Hnext.1, ?_, Hnext.2.2⟩
    rw [Hnext.1] at Hspec
    exact .constructorCallData Hspec
  | failure offset reason =>
    isimp only [FeltValuesReadOwned, FeltValuesDecoded] at Houtcome
    icases Houtcome with ⟨Hfield, Hc, %err, Herr, %Hresult⟩
    ihave Hh := Hclose $$ %h.ConstructorCallData' Hfield
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold DeployFieldOutcome
    iright
    iexists offset, (DeployFieldFailure.values reason), err
    isimp only [DeployFieldFailureError]
    iframe Hc Herr
    isplitl [Hh Hchildren]
    · unfold DeployOwned; iexists h; iframe Hh Hchildren
    ipureintro
    refine ⟨Hresult.2.1, ?_, Hresult.2.2⟩
    exact .constructorCallData Hspec

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_deploy_hash_complete
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_deploy_salt_complete
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_deploy_contract_complete
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_deploy_classHash_complete
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_deploy_version_complete
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_deploy_constructorCallData_complete
