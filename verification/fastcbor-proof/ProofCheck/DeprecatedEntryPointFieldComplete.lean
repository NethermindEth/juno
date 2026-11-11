import ProofCheck.DeprecatedEntryPointOwnership

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

def DeprecatedEntryPointFieldFailureError (err : Loc) (pos : w64) (reason : DeprecatedEntryPointFieldFailure) (kinds : FeltErrorKinds) : IProp GF :=
  match reason with
  | .felt reason => FeltFailureError err pos reason (feltFailureKind reason kinds)

def DeprecatedEntryPointFieldOutcome (c dst : Loc) (old : DeprecatedEntryPointValue) (v : cursor.Cursor) (xs wire : List w8)
    (d : Nat) (field : DeprecatedEntryPointField) (kinds : FeltErrorKinds) (result : GoInterface) : IProp GF := iprop(
  (∃ (item rest : List w8) (final : DeprecatedEntryPointValue), DeprecatedEntryPointOwned dst final ∗
    c ↦ {v with pos' := v.pos' + W64 item.length} ∗
    ⌜result = interface.nil ∧ wire = item ++ rest ∧ DeprecatedEntryPointFieldValid d field old item rest final ∧
      sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
      sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝) ∨
  (∃ (offset : Nat) (reason : DeprecatedEntryPointFieldFailure) (err : Loc), DeprecatedEntryPointOwned dst old ∗ c ↦ v ∗
    DeprecatedEntryPointFieldFailureError err (v.pos' + W64 offset) reason kinds ∗
    ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
      DeprecatedEntryPointFieldRejected d field wire offset reason ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝))

def deprecatedEntryPointFieldReaderName : DeprecatedEntryPointField → GoString
  | .selector => cborReadFelt
  | .offset => cborReadFelt

/-- Both actual field calls preserve the other field and return the complete
    independently specified value/error outcome, using initialized categories. -/
theorem wp_deprecatedEntryPoint_field_complete (c dst : Loc) (old : DeprecatedEntryPointValue) (field : DeprecatedEntryPointField) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       DeprecatedEntryPointOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (deprecatedEntryPointFieldReaderName field))) (Val #c)) (Val #(structFieldRef DeprecatedEntryPoint (deprecatedEntryPointFieldName field) dst)))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
       DeprecatedEntryPointFieldOutcome c dst old v xs wire d field kinds result }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  ihave Hglobals := header_initialized_felt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  cases field with
  | selector =>
    icases deprecatedEntryPoint_selector_acc dst old $$ Howned with ⟨%ptr, Hfield, Hoffset, Hclose⟩
    simp only [deprecatedEntryPointFieldReaderName, deprecatedEntryPointFieldName]
    wp_apply +noauto wp_readFelt_complete c (structFieldRef DeprecatedEntryPoint go!"Selector" dst) ptr v xs wire dq d kinds
      hd hdepth hp hbytes $$ [Hfield Hc Hs] as %result ⟨Hs, Houtcome⟩
    · iframe Hglobals Hfield Hc Hs
    isimp only [FeltOutcome] at Houtcome
    icases Houtcome with (Hsuccess | Hfailure)
    · icases Hsuccess with ⟨%item, %rest, %value, %out, Hfield, Hnew, Hc, %Hresult⟩
      ihave Howned := Hclose $$ %out %value [Hfield Hnew]
      · iframe Hfield Hnew
      iapply HΦ
      iframe Hs
      iexists kinds
      iframe Hglobals
      unfold DeprecatedEntryPointFieldOutcome
      ileft
      iexists item, rest, {old with selector := value}
      iframe Howned Hc
      ipureintro
      exact ⟨Hresult.1, Hresult.2.1, .selector Hresult.2.2.1, Hresult.2.2.2⟩
    · icases Hfailure with ⟨%offset, %reason, %err, Hfield, Hc, Herr, %Hresult⟩
      ihave Howned := Hclose $$ %ptr %old.selector [Hfield Hoffset]
      · iframe Hfield Hoffset
      iapply HΦ
      iframe Hs
      iexists kinds
      iframe Hglobals
      unfold DeprecatedEntryPointFieldOutcome
      iright
      iexists offset, DeprecatedEntryPointFieldFailure.felt reason, err
      isimp only [DeprecatedEntryPointFieldFailureError]
      iframe Howned Hc Herr
      ipureintro
      exact ⟨Hresult.1, .selector Hresult.2.1, Hresult.2.2⟩
  | offset =>
    icases deprecatedEntryPoint_offset_acc dst old $$ Howned with ⟨%ptr, Hfield, Hoffset, Hclose⟩
    simp only [deprecatedEntryPointFieldReaderName, deprecatedEntryPointFieldName]
    wp_apply +noauto wp_readFelt_complete c (structFieldRef DeprecatedEntryPoint go!"Offset" dst) ptr v xs wire dq d kinds
      hd hdepth hp hbytes $$ [Hfield Hc Hs] as %result ⟨Hs, Houtcome⟩
    · iframe Hglobals Hfield Hc Hs
    isimp only [FeltOutcome] at Houtcome
    icases Houtcome with (Hsuccess | Hfailure)
    · icases Hsuccess with ⟨%item, %rest, %value, %out, Hfield, Hnew, Hc, %Hresult⟩
      ihave Howned := Hclose $$ %out %value [Hfield Hnew]
      · iframe Hfield Hnew
      iapply HΦ
      iframe Hs
      iexists kinds
      iframe Hglobals
      unfold DeprecatedEntryPointFieldOutcome
      ileft
      iexists item, rest, {old with offset := value}
      iframe Howned Hc
      ipureintro
      exact ⟨Hresult.1, Hresult.2.1, .offset Hresult.2.2.1, Hresult.2.2.2⟩
    · icases Hfailure with ⟨%offset, %reason, %err, Hfield, Hc, Herr, %Hresult⟩
      ihave Howned := Hclose $$ %ptr %old.offset [Hfield Hoffset]
      · iframe Hfield Hoffset
      iapply HΦ
      iframe Hs
      iexists kinds
      iframe Hglobals
      unfold DeprecatedEntryPointFieldOutcome
      iright
      iexists offset, DeprecatedEntryPointFieldFailure.felt reason, err
      isimp only [DeprecatedEntryPointFieldFailureError]
      iframe Howned Hc Herr
      ipureintro
      exact ⟨Hresult.1, .offset Hresult.2.1, Hresult.2.2⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_deprecatedEntryPoint_field_complete
