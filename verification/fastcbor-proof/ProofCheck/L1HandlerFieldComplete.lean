import ProofCheck.L1HandlerOwnership
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

def L1HandlerFieldFailureError (err : Loc) (pos : w64) (reason : L1HandlerFieldFailure) (kinds : FeltErrorKinds) : IProp GF :=
  match reason with
  | .felt reason => FeltFailureError err pos reason (feltFailureKind reason kinds)
  | .values reason => FeltValuesFailureError err pos reason kinds

def L1HandlerFieldOutcome (c dst : Loc) (old : L1HandlerValue) (v : cursor.Cursor) (xs wire : List w8)
    (d : Nat) (field : L1HandlerField) (kinds : FeltErrorKinds) (result : GoInterface) : IProp GF := iprop(
  (∃ (item rest : List w8) (final : L1HandlerValue), L1HandlerOwned dst final ∗
    c ↦ {v with pos' := v.pos' + W64 item.length} ∗
    ⌜result = interface.nil ∧ wire = item ++ rest ∧ L1HandlerFieldValid d field old item rest final ∧
      sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
      sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝) ∨
  (∃ (offset : Nat) (reason : L1HandlerFieldFailure) (err : Loc), L1HandlerOwned dst old ∗ c ↦ v ∗
    L1HandlerFieldFailureError err (v.pos' + W64 offset) reason kinds ∗
    ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
      L1HandlerFieldRejected d field wire offset reason ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝))


theorem wp_l1Handler_hash_complete (c dst : Loc) (old : L1HandlerValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       L1HandlerOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #(structFieldRef L1HandlerTransaction go!"TransactionHash" dst)))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
       L1HandlerFieldOutcome c dst old v xs wire d .hash kinds result }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  isimp only [L1HandlerOwned] at Howned
  icases Howned with ⟨%h, Hh, Hchildren⟩
  icases l1Handler_hash_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
  ihave Hglobals : (∃ kinds : FeltErrorKinds, FeltErrorGlobals (GF := GF) kinds : IProp GF) $$ []
  · iapply header_initialized_felt_globals
    solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  wp_apply wp_readFelt_complete c (structFieldRef L1HandlerTransaction go!"TransactionHash" dst) h.TransactionHash' v xs wire dq d kinds
    hd hdepth hp hbytes $$ [Hfield Hc Hs] as %result ⟨Hs, Houtcome⟩
  · iframe Hglobals Hfield Hc Hs
  isimp only [FeltOutcome] at Houtcome
  icases Houtcome with (Hsuccess | Hfailure)
  · icases Hsuccess with ⟨%item, %rest, %value, %p, Hfield, Hvalue, Hc, %Hresult⟩
    ihave Hh := Hclose $$ %p Hfield
    ihave Hchildren := l1HandlerChildren_hash h old p value $$ [Hchildren Hvalue]
    · iframe Hchildren Hvalue
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold L1HandlerFieldOutcome
    ileft
    iexists item, rest, ({old with hash := value} : L1HandlerValue)
    iframe Hc
    isplitl [Hh Hchildren]
    · unfold L1HandlerOwned; iexists ({h with TransactionHash' := p} : L1HandlerTransaction); iframe Hh Hchildren
    ipureintro
    exact ⟨Hresult.1, Hresult.2.1, .hash Hresult.2.2.1, Hresult.2.2.2⟩
  · icases Hfailure with ⟨%offset, %reason, %err, Hfield, Hc, Herr, %Hresult⟩
    ihave Hh := Hclose $$ %h.TransactionHash' Hfield
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold L1HandlerFieldOutcome
    iright
    iexists offset, (L1HandlerFieldFailure.felt reason), err
    isimp only [L1HandlerFieldFailureError]
    iframe Hc Herr
    isplitl [Hh Hchildren]
    · unfold L1HandlerOwned; iexists h; iframe Hh Hchildren
    ipureintro
    exact ⟨Hresult.1, .hash Hresult.2.1, Hresult.2.2⟩


theorem wp_l1Handler_contract_complete (c dst : Loc) (old : L1HandlerValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       L1HandlerOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #(structFieldRef L1HandlerTransaction go!"ContractAddress" dst)))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
       L1HandlerFieldOutcome c dst old v xs wire d .contract kinds result }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  isimp only [L1HandlerOwned] at Howned
  icases Howned with ⟨%h, Hh, Hchildren⟩
  icases l1Handler_contract_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
  ihave Hglobals : (∃ kinds : FeltErrorKinds, FeltErrorGlobals (GF := GF) kinds : IProp GF) $$ []
  · iapply header_initialized_felt_globals
    solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  wp_apply wp_readFelt_complete c (structFieldRef L1HandlerTransaction go!"ContractAddress" dst) h.ContractAddress' v xs wire dq d kinds
    hd hdepth hp hbytes $$ [Hfield Hc Hs] as %result ⟨Hs, Houtcome⟩
  · iframe Hglobals Hfield Hc Hs
  isimp only [FeltOutcome] at Houtcome
  icases Houtcome with (Hsuccess | Hfailure)
  · icases Hsuccess with ⟨%item, %rest, %value, %p, Hfield, Hvalue, Hc, %Hresult⟩
    ihave Hh := Hclose $$ %p Hfield
    ihave Hchildren := l1HandlerChildren_contract h old p value $$ [Hchildren Hvalue]
    · iframe Hchildren Hvalue
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold L1HandlerFieldOutcome
    ileft
    iexists item, rest, ({old with contract := value} : L1HandlerValue)
    iframe Hc
    isplitl [Hh Hchildren]
    · unfold L1HandlerOwned; iexists ({h with ContractAddress' := p} : L1HandlerTransaction); iframe Hh Hchildren
    ipureintro
    exact ⟨Hresult.1, Hresult.2.1, .contract Hresult.2.2.1, Hresult.2.2.2⟩
  · icases Hfailure with ⟨%offset, %reason, %err, Hfield, Hc, Herr, %Hresult⟩
    ihave Hh := Hclose $$ %h.ContractAddress' Hfield
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold L1HandlerFieldOutcome
    iright
    iexists offset, (L1HandlerFieldFailure.felt reason), err
    isimp only [L1HandlerFieldFailureError]
    iframe Hc Herr
    isplitl [Hh Hchildren]
    · unfold L1HandlerOwned; iexists h; iframe Hh Hchildren
    ipureintro
    exact ⟨Hresult.1, .contract Hresult.2.1, Hresult.2.2⟩


theorem wp_l1Handler_entry_complete (c dst : Loc) (old : L1HandlerValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       L1HandlerOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #(structFieldRef L1HandlerTransaction go!"EntryPointSelector" dst)))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
       L1HandlerFieldOutcome c dst old v xs wire d .entry kinds result }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  isimp only [L1HandlerOwned] at Howned
  icases Howned with ⟨%h, Hh, Hchildren⟩
  icases l1Handler_entry_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
  ihave Hglobals : (∃ kinds : FeltErrorKinds, FeltErrorGlobals (GF := GF) kinds : IProp GF) $$ []
  · iapply header_initialized_felt_globals
    solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  wp_apply wp_readFelt_complete c (structFieldRef L1HandlerTransaction go!"EntryPointSelector" dst) h.EntryPointSelector' v xs wire dq d kinds
    hd hdepth hp hbytes $$ [Hfield Hc Hs] as %result ⟨Hs, Houtcome⟩
  · iframe Hglobals Hfield Hc Hs
  isimp only [FeltOutcome] at Houtcome
  icases Houtcome with (Hsuccess | Hfailure)
  · icases Hsuccess with ⟨%item, %rest, %value, %p, Hfield, Hvalue, Hc, %Hresult⟩
    ihave Hh := Hclose $$ %p Hfield
    ihave Hchildren := l1HandlerChildren_entry h old p value $$ [Hchildren Hvalue]
    · iframe Hchildren Hvalue
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold L1HandlerFieldOutcome
    ileft
    iexists item, rest, ({old with entry := value} : L1HandlerValue)
    iframe Hc
    isplitl [Hh Hchildren]
    · unfold L1HandlerOwned; iexists ({h with EntryPointSelector' := p} : L1HandlerTransaction); iframe Hh Hchildren
    ipureintro
    exact ⟨Hresult.1, Hresult.2.1, .entry Hresult.2.2.1, Hresult.2.2.2⟩
  · icases Hfailure with ⟨%offset, %reason, %err, Hfield, Hc, Herr, %Hresult⟩
    ihave Hh := Hclose $$ %h.EntryPointSelector' Hfield
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold L1HandlerFieldOutcome
    iright
    iexists offset, (L1HandlerFieldFailure.felt reason), err
    isimp only [L1HandlerFieldFailureError]
    iframe Hc Herr
    isplitl [Hh Hchildren]
    · unfold L1HandlerOwned; iexists h; iframe Hh Hchildren
    ipureintro
    exact ⟨Hresult.1, .entry Hresult.2.1, Hresult.2.2⟩


theorem wp_l1Handler_nonce_complete (c dst : Loc) (old : L1HandlerValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       L1HandlerOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #(structFieldRef L1HandlerTransaction go!"Nonce" dst)))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
       L1HandlerFieldOutcome c dst old v xs wire d .nonce kinds result }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  isimp only [L1HandlerOwned] at Howned
  icases Howned with ⟨%h, Hh, Hchildren⟩
  icases l1Handler_nonce_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
  ihave Hglobals : (∃ kinds : FeltErrorKinds, FeltErrorGlobals (GF := GF) kinds : IProp GF) $$ []
  · iapply header_initialized_felt_globals
    solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  wp_apply wp_readFelt_complete c (structFieldRef L1HandlerTransaction go!"Nonce" dst) h.Nonce' v xs wire dq d kinds
    hd hdepth hp hbytes $$ [Hfield Hc Hs] as %result ⟨Hs, Houtcome⟩
  · iframe Hglobals Hfield Hc Hs
  isimp only [FeltOutcome] at Houtcome
  icases Houtcome with (Hsuccess | Hfailure)
  · icases Hsuccess with ⟨%item, %rest, %value, %p, Hfield, Hvalue, Hc, %Hresult⟩
    ihave Hh := Hclose $$ %p Hfield
    ihave Hchildren := l1HandlerChildren_nonce h old p value $$ [Hchildren Hvalue]
    · iframe Hchildren Hvalue
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold L1HandlerFieldOutcome
    ileft
    iexists item, rest, ({old with nonce := value} : L1HandlerValue)
    iframe Hc
    isplitl [Hh Hchildren]
    · unfold L1HandlerOwned; iexists ({h with Nonce' := p} : L1HandlerTransaction); iframe Hh Hchildren
    ipureintro
    exact ⟨Hresult.1, Hresult.2.1, .nonce Hresult.2.2.1, Hresult.2.2.2⟩
  · icases Hfailure with ⟨%offset, %reason, %err, Hfield, Hc, Herr, %Hresult⟩
    ihave Hh := Hclose $$ %h.Nonce' Hfield
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold L1HandlerFieldOutcome
    iright
    iexists offset, (L1HandlerFieldFailure.felt reason), err
    isimp only [L1HandlerFieldFailureError]
    iframe Hc Herr
    isplitl [Hh Hchildren]
    · unfold L1HandlerOwned; iexists h; iframe Hh Hchildren
    ipureintro
    exact ⟨Hresult.1, .nonce Hresult.2.1, Hresult.2.2⟩


theorem wp_l1Handler_version_complete (c dst : Loc) (old : L1HandlerValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       L1HandlerOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadTransactionVersion)) (Val #c)) (Val #(structFieldRef L1HandlerTransaction go!"Version" dst)))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
       L1HandlerFieldOutcome c dst old v xs wire d .version kinds result }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  isimp only [L1HandlerOwned] at Howned
  icases Howned with ⟨%h, Hh, Hchildren⟩
  icases l1Handler_version_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
  ihave Hglobals : (∃ kinds : FeltErrorKinds, FeltErrorGlobals (GF := GF) kinds : IProp GF) $$ []
  · iapply header_initialized_felt_globals
    solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  wp_apply wp_readTransactionVersion_felt_outcome c (structFieldRef L1HandlerTransaction go!"Version" dst) h.Version' v xs wire dq d kinds
    hd hdepth hp hbytes $$ [Hfield Hc Hs] as %result ⟨Hs, Houtcome⟩
  · iframe Hglobals Hfield Hc Hs
  isimp only [FeltOutcome] at Houtcome
  icases Houtcome with (Hsuccess | Hfailure)
  · icases Hsuccess with ⟨%item, %rest, %value, %p, Hfield, Hvalue, Hc, %Hresult⟩
    ihave Hh := Hclose $$ %p Hfield
    ihave Hchildren := l1HandlerChildren_version h old p value $$ [Hchildren Hvalue]
    · iframe Hchildren Hvalue
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold L1HandlerFieldOutcome
    ileft
    iexists item, rest, ({old with version := value} : L1HandlerValue)
    iframe Hc
    isplitl [Hh Hchildren]
    · unfold L1HandlerOwned; iexists ({h with Version' := p} : L1HandlerTransaction); iframe Hh Hchildren
    ipureintro
    exact ⟨Hresult.1, Hresult.2.1, .version Hresult.2.2.1, Hresult.2.2.2⟩
  · icases Hfailure with ⟨%offset, %reason, %err, Hfield, Hc, Herr, %Hresult⟩
    ihave Hh := Hclose $$ %h.Version' Hfield
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold L1HandlerFieldOutcome
    iright
    iexists offset, (L1HandlerFieldFailure.felt reason), err
    isimp only [L1HandlerFieldFailureError]
    iframe Hc Herr
    isplitl [Hh Hchildren]
    · unfold L1HandlerOwned; iexists h; iframe Hh Hchildren
    ipureintro
    exact ⟨Hresult.1, .version Hresult.2.1, Hresult.2.2⟩

theorem wp_l1Handler_callData_complete (c dst : Loc) (old : L1HandlerValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       L1HandlerOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltValues)) (Val #c)) (Val #(structFieldRef L1HandlerTransaction go!"CallData" dst)))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
       L1HandlerFieldOutcome c dst old v xs wire d .callData kinds result }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  isimp only [L1HandlerOwned] at Howned
  icases Howned with ⟨%h, Hh, Hchildren⟩
  icases l1Handler_callData_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
  wp_apply wp_readFeltValues_complete c (structFieldRef L1HandlerTransaction go!"CallData" dst)
    h.CallData' v xs wire dq d hd hdepth hp hbytes $$ [Hfield Hc Hs]
    as %result ⟨Hs, %outcome, %kinds, Hglobals, %Hspec, Houtcome⟩
  · iframe Hfield Hc Hs
  cases outcome with
  | success value rest =>
    isimp only [FeltValuesReadOwned, FeltValuesSuccess] at Houtcome
    icases Houtcome with ⟨%Hnil, %out, Hfield, Hvalue, %item, Hc, %Hnext⟩
    ihave Hh := Hclose $$ %out Hfield
    ihave Hchildren := l1HandlerChildren_callData h old out value $$ [Hchildren Hvalue]
    · iframe Hchildren Hvalue
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold L1HandlerFieldOutcome
    ileft
    iexists item, rest, ({old with callData := value} : L1HandlerValue)
    iframe Hc
    isplitl [Hh Hchildren]
    · unfold L1HandlerOwned; iexists ({h with CallData' := out} : L1HandlerTransaction); iframe Hh Hchildren
    ipureintro
    refine ⟨Hnil, Hnext.1, ?_, Hnext.2.2⟩
    rw [Hnext.1] at Hspec
    exact .callData Hspec
  | failure offset reason =>
    isimp only [FeltValuesReadOwned, FeltValuesDecoded] at Houtcome
    icases Houtcome with ⟨Hfield, Hc, %err, Herr, %Hresult⟩
    ihave Hh := Hclose $$ %h.CallData' Hfield
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold L1HandlerFieldOutcome
    iright
    iexists offset, (L1HandlerFieldFailure.values reason), err
    isimp only [L1HandlerFieldFailureError]
    iframe Hc Herr
    isplitl [Hh Hchildren]
    · unfold L1HandlerOwned; iexists h; iframe Hh Hchildren
    ipureintro
    refine ⟨Hresult.2.1, ?_, Hresult.2.2⟩
    exact .callData Hspec

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_l1Handler_hash_complete
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_l1Handler_contract_complete
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_l1Handler_entry_complete
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_l1Handler_nonce_complete
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_l1Handler_version_complete
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_l1Handler_callData_complete
