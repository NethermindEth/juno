import ProofCheck.EventFieldComplete

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

def eventFieldReaderName : EventField → GoString
  | .sender => cborReadFelt
  | .keys | .data => cborReadFeltValues

/-- Uniform contract for the three actual field calls in the generated loop. -/
theorem wp_event_field_complete (c dst : Loc) (old : EventValue) (field : EventField) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       EventOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (eventFieldReaderName field))) (Val #c)) (Val #(structFieldRef Event (eventFieldName field) dst)))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
       EventFieldOutcome c dst old v xs wire d field kinds result }} := by
  cases field with
  | sender => exact wp_event_sender_complete c dst old v xs wire dq d hd hdepth hp hbytes
  | keys => exact wp_event_values_complete c dst old true v xs wire dq d hd hdepth hp hbytes
  | data => exact wp_event_values_complete c dst old false v xs wire dq d hd hdepth hp hbytes

/-- Independent successful field decoding determines the exact result of the
    actual call, including its contextual felt-slice guard and unread suffix. -/
theorem wp_event_field_valid (c dst : Loc) (old final : EventValue) (field : EventField) (v : cursor.Cursor)
    (xs item rem : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (valid : EventFieldValid d field old item rem final)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       EventOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (eventFieldReaderName field))) (Val #c)) (Val #(structFieldRef Event (eventFieldName field) dst)))
    {{ RET #interface.nil;
       EventOwned dst final ∗ c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  wp_apply +noauto wp_event_field_complete c dst old field v xs (item ++ rem) dq d hd hdepth hp hbytes
    $$ [Howned Hc Hs] as %result ⟨Hs, %kinds, Hglobals, Hout⟩
  · iframe Howned Hc Hs
  isimp only [EventFieldOutcome] at Hout
  icases Hout with (Hsuccess | Hfailure)
  · icases Hsuccess with ⟨%actual, %rest, %out, Howned, Hc, %Hresult⟩
    obtain ⟨rfl, rfl, rfl⟩ := eventField_prefix_unique d field old final out item actual rem rest valid Hresult.2.2.1 Hresult.2.1
    rcases Hresult with ⟨rfl, _, _, Hnext⟩
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro; exact Hnext
  · icases Hfailure with ⟨%offset, %reason, %err, Howned, Hc, Herr, %Hresult⟩
    exact False.elim (eventField_not_rejected d field old final item rem offset reason valid Hresult.2.1)

/-- Independent rejection determines the owned diagnostic and original Event;
    no field or child allocation is published on a failed call. -/
theorem wp_event_field_rejected (c dst : Loc) (old : EventValue) (field : EventField) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d offset : Nat) (reason : EventFieldFailure)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (failure : EventFieldRejected d field wire offset reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       EventOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (eventFieldReaderName field))) (Val #c)) (Val #(structFieldRef Event (eventFieldName field) dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       EventOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗ EventFieldFailureError err (v.pos' + W64 offset) reason kinds ∗
       ⌜sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  wp_apply +noauto wp_event_field_complete c dst old field v xs wire dq d hd hdepth hp hbytes
    $$ [Howned Hc Hs] as %result ⟨Hs, %kinds, Hglobals, Hout⟩
  · iframe Howned Hc Hs
  isimp only [EventFieldOutcome] at Hout
  icases Hout with (Hsuccess | Hfailure)
  · icases Hsuccess with ⟨%item, %rest, %out, Howned, Hc, %Hresult⟩
    rw [Hresult.2.1] at failure
    exact False.elim (eventField_not_rejected d field old out item rest offset reason Hresult.2.2.1 failure)
  · icases Hfailure with ⟨%actual, %why, %err, Howned, Hc, Herr, %Hresult⟩
    obtain ⟨rfl, rfl⟩ := eventField_rejected_unique d field wire offset actual reason why failure Hresult.2.1
    rcases Hresult with ⟨rfl, _, Hnext⟩
    iapply HΦ
    iframe Howned Hc Hs
    iexists kinds
    iframe Hglobals Herr
    ipureintro; exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_event_field_complete
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_event_field_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_event_field_rejected
