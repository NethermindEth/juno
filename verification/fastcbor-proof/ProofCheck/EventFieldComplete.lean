import ProofCheck.EventOwnership

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

def EventFieldFailureError (err : Loc) (pos : w64) (reason : EventFieldFailure) (kinds : FeltErrorKinds) : IProp GF :=
  match reason with
  | .felt reason => FeltFailureError err pos reason (feltFailureKind reason kinds)
  | .values reason => FeltValuesFailureError err pos reason kinds

def EventFieldOutcome (c dst : Loc) (old : EventValue) (v : cursor.Cursor) (xs wire : List w8)
    (d : Nat) (field : EventField) (kinds : FeltErrorKinds) (result : GoInterface) : IProp GF := iprop(
  (∃ (item rest : List w8) (final : EventValue), EventOwned dst final ∗
    c ↦ {v with pos' := v.pos' + W64 item.length} ∗
    ⌜result = interface.nil ∧ wire = item ++ rest ∧ EventFieldValid d field old item rest final ∧
      sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
      sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝) ∨
  (∃ (offset : Nat) (reason : EventFieldFailure) (err : Loc), EventOwned dst old ∗ c ↦ v ∗
    EventFieldFailureError err (v.pos' + W64 offset) reason kinds ∗
    ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
      EventFieldRejected d field wire offset reason ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝))

/-- The actual sender branch owns the whole Event on every outcome; the two
    slice allocations are retained and never accessed by this branch. -/
theorem wp_event_sender_complete (c dst : Loc) (old : EventValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       EventOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #(structFieldRef Event go!"From" dst)))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
       EventFieldOutcome c dst old v xs wire d .sender kinds result }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  isimp only [EventOwned] at Howned
  icases Howned with ⟨%h, Hh, Hchildren⟩
  icases event_sender_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
  ihave Hglobals : (∃ kinds : FeltErrorKinds, FeltErrorGlobals (GF := GF) kinds : IProp GF) $$ []
  · iapply header_initialized_felt_globals
    solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  wp_apply wp_readFelt_complete c (structFieldRef Event go!"From" dst) h.From' v xs wire dq d kinds
    hd hdepth hp hbytes $$ [Hfield Hc Hs] as %result ⟨Hs, Houtcome⟩
  · iframe Hglobals Hfield Hc Hs
  isimp only [FeltOutcome] at Houtcome
  icases Houtcome with (Hsuccess | Hfailure)
  · icases Hsuccess with ⟨%item, %rest, %value, %p, Hfield, Hvalue, Hc, %Hresult⟩
    ihave Hh := Hclose $$ %p Hfield
    ihave Hchildren := eventChildren_sender h old p value $$ [Hchildren Hvalue]
    · iframe Hchildren Hvalue
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold EventFieldOutcome
    ileft
    iexists item, rest, ({old with sender := value} : EventValue)
    iframe Hc
    isplitl [Hh Hchildren]
    · unfold EventOwned; iexists ({h with From' := p} : Event); iframe Hh Hchildren
    ipureintro
    exact ⟨Hresult.1, Hresult.2.1, .sender Hresult.2.2.1, Hresult.2.2.2⟩
  · icases Hfailure with ⟨%offset, %reason, %err, Hfield, Hc, Herr, %Hresult⟩
    ihave Hh := Hclose $$ %h.From' Hfield
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold EventFieldOutcome
    iright
    iexists offset, (EventFieldFailure.felt reason), err
    isimp only [EventFieldFailureError]
    iframe Hc Herr
    isplitl [Hh Hchildren]
    · unfold EventOwned; iexists h; iframe Hh Hchildren
    ipureintro
    exact ⟨Hresult.1, .sender Hresult.2.1, Hresult.2.2⟩

/-- Either actual slice-field branch publishes only after complete success,
    retaining sender/other-slice ownership and rolling back on every error. -/
theorem wp_event_values_complete (c dst : Loc) (old : EventValue) (keys : Bool) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       EventOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltValues)) (Val #c)) (Val #(structFieldRef Event (eventValuesFieldName keys) dst)))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
       EventFieldOutcome c dst old v xs wire d (if keys then .keys else .data) kinds result }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  isimp only [EventOwned] at Howned
  icases Howned with ⟨%h, Hh, Hchildren⟩
  icases event_values_acc dst h keys $$ Hh with ⟨Hfield, Hclose⟩
  wp_apply wp_readFeltValues_complete c (structFieldRef Event (eventValuesFieldName keys) dst)
    (eventValuesField keys h) v xs wire dq d hd hdepth hp hbytes $$ [Hfield Hc Hs]
    as %result ⟨Hs, %outcome, %kinds, Hglobals, %Hspec, Houtcome⟩
  · iframe Hfield Hc Hs
  cases outcome with
  | success value rest =>
    isimp only [FeltValuesReadOwned, FeltValuesSuccess] at Houtcome
    icases Houtcome with ⟨%Hnil, %out, Hfield, Hvalue, %item, Hc, %Hnext⟩
    ihave Hh := Hclose $$ %out Hfield
    ihave Hchildren := eventChildren_values h old keys out value $$ [Hchildren Hvalue]
    · iframe Hchildren Hvalue
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold EventFieldOutcome
    ileft
    iexists item, rest, (eventValueSetValues keys old value)
    iframe Hc
    isplitl [Hh Hchildren]
    · unfold EventOwned; iexists (eventSetValues keys h out); iframe Hh Hchildren
    ipureintro
    refine ⟨Hnil, Hnext.1, ?_, Hnext.2.2⟩
    rw [Hnext.1] at Hspec
    cases keys
    · exact .data Hspec
    · exact .keys Hspec
  | failure offset reason =>
    isimp only [FeltValuesReadOwned, FeltValuesDecoded] at Houtcome
    icases Houtcome with ⟨Hfield, Hc, %err, Herr, %Hresult⟩
    ihave Hh := Hclose $$ %(eventValuesField keys h) Hfield
    isimp only [eventSetValues_original] at Hh
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold EventFieldOutcome
    iright
    iexists offset, (EventFieldFailure.values reason), err
    isimp only [EventFieldFailureError]
    iframe Hc Herr
    isplitl [Hh Hchildren]
    · unfold EventOwned; iexists h; iframe Hh Hchildren
    ipureintro
    refine ⟨Hresult.2.1, ?_, Hresult.2.2⟩
    cases keys
    · exact .data Hspec
    · exact .keys Hspec

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_event_sender_complete
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_event_values_complete
