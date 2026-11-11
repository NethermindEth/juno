import ProofCheck.L1MessageOwnership

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

def L1MessageFieldFailureError (err : Loc) (pos : w64) (reason : L1MessageFieldFailure) (kinds : FeltErrorKinds) : IProp GF :=
  match reason with
  | .felt reason => FeltFailureError err pos reason (feltFailureKind reason kinds)
  | .values reason => FeltValuesFailureError err pos reason kinds
  | .address reason => BytesIntoError err pos 20 kinds reason

def L1MessageFieldOutcome (c dst : Loc) (old : L1MessageValue) (v : cursor.Cursor) (xs wire : List w8)
    (d : Nat) (field : L1MessageField) (kinds : FeltErrorKinds) (result : GoInterface) : IProp GF := iprop(
  (∃ (item rest : List w8) (final : L1MessageValue), L1MessageOwned dst final ∗
    c ↦ {v with pos' := v.pos' + W64 item.length} ∗
    ⌜result = interface.nil ∧ wire = item ++ rest ∧ L1MessageFieldValid d field old item rest final ∧
      sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
      sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝) ∨
  (∃ (offset : Nat) (reason : L1MessageFieldFailure) (err : Loc), L1MessageOwned dst old ∗ c ↦ v ∗
    L1MessageFieldFailureError err (v.pos' + W64 offset) reason kinds ∗
    ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
      L1MessageFieldRejected d field wire offset reason ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝))

/-- The actual recipient branch owns the whole L1ToL2Message on every outcome; the other
    fields and their child allocations remain owned and unchanged. -/
theorem wp_l1Message_recipient_complete (c dst : Loc) (old : L1MessageValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       L1MessageOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #(structFieldRef L1ToL2Message go!"To" dst)))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
       L1MessageFieldOutcome c dst old v xs wire d .recipient kinds result }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  isimp only [L1MessageOwned] at Howned
  icases Howned with ⟨%h, Hh, Hchildren⟩
  icases l1Message_recipient_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
  ihave Hglobals : (∃ kinds : FeltErrorKinds, FeltErrorGlobals (GF := GF) kinds : IProp GF) $$ []
  · iapply header_initialized_felt_globals
    solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  wp_apply wp_readFelt_complete c (structFieldRef L1ToL2Message go!"To" dst) h.To' v xs wire dq d kinds
    hd hdepth hp hbytes $$ [Hfield Hc Hs] as %result ⟨Hs, Houtcome⟩
  · iframe Hglobals Hfield Hc Hs
  isimp only [FeltOutcome] at Houtcome
  icases Houtcome with (Hsuccess | Hfailure)
  · icases Hsuccess with ⟨%item, %rest, %value, %p, Hfield, Hvalue, Hc, %Hresult⟩
    ihave Hh := Hclose $$ %p Hfield
    ihave Hchildren := l1MessageChildren_recipient h old p value $$ [Hchildren Hvalue]
    · iframe Hchildren Hvalue
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold L1MessageFieldOutcome
    ileft
    iexists item, rest, ({old with recipient := value} : L1MessageValue)
    iframe Hc
    isplitl [Hh Hchildren]
    · unfold L1MessageOwned; iexists ({h with To' := p} : L1ToL2Message); iframe Hh Hchildren
    ipureintro
    exact ⟨Hresult.1, Hresult.2.1, .recipient Hresult.2.2.1, Hresult.2.2.2⟩
  · icases Hfailure with ⟨%offset, %reason, %err, Hfield, Hc, Herr, %Hresult⟩
    ihave Hh := Hclose $$ %h.To' Hfield
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold L1MessageFieldOutcome
    iright
    iexists offset, (L1MessageFieldFailure.felt reason), err
    isimp only [L1MessageFieldFailureError]
    iframe Hc Herr
    isplitl [Hh Hchildren]
    · unfold L1MessageOwned; iexists h; iframe Hh Hchildren
    ipureintro
    exact ⟨Hresult.1, .recipient Hresult.2.1, Hresult.2.2⟩

theorem wp_l1Message_nonce_complete (c dst : Loc) (old : L1MessageValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       L1MessageOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #(structFieldRef L1ToL2Message go!"Nonce" dst)))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
       L1MessageFieldOutcome c dst old v xs wire d .nonce kinds result }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  isimp only [L1MessageOwned] at Howned
  icases Howned with ⟨%h, Hh, Hchildren⟩
  icases l1Message_nonce_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
  ihave Hglobals : (∃ kinds : FeltErrorKinds, FeltErrorGlobals (GF := GF) kinds : IProp GF) $$ []
  · iapply header_initialized_felt_globals
    solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  wp_apply wp_readFelt_complete c (structFieldRef L1ToL2Message go!"Nonce" dst) h.Nonce' v xs wire dq d kinds
    hd hdepth hp hbytes $$ [Hfield Hc Hs] as %result ⟨Hs, Houtcome⟩
  · iframe Hglobals Hfield Hc Hs
  isimp only [FeltOutcome] at Houtcome
  icases Houtcome with (Hsuccess | Hfailure)
  · icases Hsuccess with ⟨%item, %rest, %value, %p, Hfield, Hvalue, Hc, %Hresult⟩
    ihave Hh := Hclose $$ %p Hfield
    ihave Hchildren := l1MessageChildren_nonce h old p value $$ [Hchildren Hvalue]
    · iframe Hchildren Hvalue
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold L1MessageFieldOutcome
    ileft
    iexists item, rest, ({old with nonce := value} : L1MessageValue)
    iframe Hc
    isplitl [Hh Hchildren]
    · unfold L1MessageOwned; iexists ({h with Nonce' := p} : L1ToL2Message); iframe Hh Hchildren
    ipureintro
    exact ⟨Hresult.1, Hresult.2.1, .nonce Hresult.2.2.1, Hresult.2.2.2⟩
  · icases Hfailure with ⟨%offset, %reason, %err, Hfield, Hc, Herr, %Hresult⟩
    ihave Hh := Hclose $$ %h.Nonce' Hfield
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold L1MessageFieldOutcome
    iright
    iexists offset, (L1MessageFieldFailure.felt reason), err
    isimp only [L1MessageFieldFailureError]
    iframe Hc Herr
    isplitl [Hh Hchildren]
    · unfold L1MessageOwned; iexists h; iframe Hh Hchildren
    ipureintro
    exact ⟨Hresult.1, .nonce Hresult.2.1, Hresult.2.2⟩

theorem wp_l1Message_selector_complete (c dst : Loc) (old : L1MessageValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       L1MessageOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #(structFieldRef L1ToL2Message go!"Selector" dst)))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
       L1MessageFieldOutcome c dst old v xs wire d .selector kinds result }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  isimp only [L1MessageOwned] at Howned
  icases Howned with ⟨%h, Hh, Hchildren⟩
  icases l1Message_selector_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
  ihave Hglobals : (∃ kinds : FeltErrorKinds, FeltErrorGlobals (GF := GF) kinds : IProp GF) $$ []
  · iapply header_initialized_felt_globals
    solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  wp_apply wp_readFelt_complete c (structFieldRef L1ToL2Message go!"Selector" dst) h.Selector' v xs wire dq d kinds
    hd hdepth hp hbytes $$ [Hfield Hc Hs] as %result ⟨Hs, Houtcome⟩
  · iframe Hglobals Hfield Hc Hs
  isimp only [FeltOutcome] at Houtcome
  icases Houtcome with (Hsuccess | Hfailure)
  · icases Hsuccess with ⟨%item, %rest, %value, %p, Hfield, Hvalue, Hc, %Hresult⟩
    ihave Hh := Hclose $$ %p Hfield
    ihave Hchildren := l1MessageChildren_selector h old p value $$ [Hchildren Hvalue]
    · iframe Hchildren Hvalue
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold L1MessageFieldOutcome
    ileft
    iexists item, rest, ({old with selector := value} : L1MessageValue)
    iframe Hc
    isplitl [Hh Hchildren]
    · unfold L1MessageOwned; iexists ({h with Selector' := p} : L1ToL2Message); iframe Hh Hchildren
    ipureintro
    exact ⟨Hresult.1, Hresult.2.1, .selector Hresult.2.2.1, Hresult.2.2.2⟩
  · icases Hfailure with ⟨%offset, %reason, %err, Hfield, Hc, Herr, %Hresult⟩
    ihave Hh := Hclose $$ %h.Selector' Hfield
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold L1MessageFieldOutcome
    iright
    iexists offset, (L1MessageFieldFailure.felt reason), err
    isimp only [L1MessageFieldFailureError]
    iframe Hc Herr
    isplitl [Hh Hchildren]
    · unfold L1MessageOwned; iexists h; iframe Hh Hchildren
    ipureintro
    exact ⟨Hresult.1, .selector Hresult.2.1, Hresult.2.2⟩

/-- The actual payload branch publishes only after complete success, retaining
    recipient/address ownership and rolling back on every error. -/
theorem wp_l1Message_payload_complete (c dst : Loc) (old : L1MessageValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       L1MessageOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltValues)) (Val #c)) (Val #(structFieldRef L1ToL2Message go!"Payload" dst)))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
       L1MessageFieldOutcome c dst old v xs wire d .payload kinds result }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  isimp only [L1MessageOwned] at Howned
  icases Howned with ⟨%h, Hh, Hchildren⟩
  icases l1Message_payload_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
  wp_apply wp_readFeltValues_complete c (structFieldRef L1ToL2Message go!"Payload" dst)
    h.Payload' v xs wire dq d hd hdepth hp hbytes $$ [Hfield Hc Hs]
    as %result ⟨Hs, %outcome, %kinds, Hglobals, %Hspec, Houtcome⟩
  · iframe Hfield Hc Hs
  cases outcome with
  | success value rest =>
    isimp only [FeltValuesReadOwned, FeltValuesSuccess] at Houtcome
    icases Houtcome with ⟨%Hnil, %out, Hfield, Hvalue, %item, Hc, %Hnext⟩
    ihave Hh := Hclose $$ %out Hfield
    ihave Hchildren := l1MessageChildren_payload h old out value $$ [Hchildren Hvalue]
    · iframe Hchildren Hvalue
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold L1MessageFieldOutcome
    ileft
    iexists item, rest, ({old with payload := value} : L1MessageValue)
    iframe Hc
    isplitl [Hh Hchildren]
    · unfold L1MessageOwned; iexists ({h with Payload' := out} : L1ToL2Message); iframe Hh Hchildren
    ipureintro
    refine ⟨Hnil, Hnext.1, ?_, Hnext.2.2⟩
    rw [Hnext.1] at Hspec
    exact .payload Hspec
  | failure offset reason =>
    isimp only [FeltValuesReadOwned, FeltValuesDecoded] at Houtcome
    icases Houtcome with ⟨Hfield, Hc, %err, Herr, %Hresult⟩
    ihave Hh := Hclose $$ %h.Payload' Hfield
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold L1MessageFieldOutcome
    iright
    iexists offset, (L1MessageFieldFailure.values reason), err
    isimp only [L1MessageFieldFailureError]
    iframe Hc Herr
    isplitl [Hh Hchildren]
    · unfold L1MessageOwned; iexists h; iframe Hh Hchildren
    ipureintro
    refine ⟨Hresult.2.1, ?_, Hresult.2.2⟩
    exact .payload Hspec

/-- The inline address call changes only From, with exact success consumption and
    complete rollback on errors. Null/undefined preserve the prior inline value. -/
theorem wp_l1Message_sender_complete (c dst : Loc) (old : L1MessageValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       L1MessageOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadAddress)) (Val #c)) (Val #(structFieldRef L1ToL2Message go!"From" dst)))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
       L1MessageFieldOutcome c dst old v xs wire d .sender kinds result }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  isimp only [L1MessageOwned] at Howned
  icases Howned with ⟨%h, Hh, Hchildren⟩
  ihave %Haddress : ⌜h.From' = old.sender⌝ $$ [Hchildren]
  · unfold L1MessageChildren
    icases Hchildren with ⟨Haddress, _, _, _, _⟩
    iexact Haddress
  icases l1Message_sender_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
  wp_apply wp_readAddress_complete c (structFieldRef L1ToL2Message go!"From" dst) h.From' v xs wire dq hp hbytes
    $$ [Hfield Hc Hs] as %result ⟨Hs, %kinds, %outcome, Hglobals, %Hspec, Houtcome⟩
  · iframe Hfield Hc Hs
  cases outcome with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := Hspec.1
    isimp only [AddressReadOutcome] at Houtcome
    icases Houtcome with ⟨%consumed, Hfield, Hc, %Hresult⟩
    have count : consumed = item.length := by rw [split, List.length_append] at Hresult; omega
    subst consumed
    ihave Hh := Hclose $$ %value Hfield
    ihave Hchildren := l1MessageChildren_sender h old value $$ Hchildren
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold L1MessageFieldOutcome
    ileft
    iexists item, rest, ({old with sender := value} : L1MessageValue)
    iframe Hc
    isplitl [Hh Hchildren]
    · unfold L1MessageOwned; iexists ({h with From' := value} : L1ToL2Message); iframe Hh Hchildren
    ipureintro
    exact ⟨Hresult.1, split, .sender (by simpa only [Haddress] using valid), Hresult.2.2⟩
  | failure reason =>
    isimp only [AddressReadOutcome] at Houtcome
    icases Houtcome with ⟨Hfield, Hc, %err, %Hresult, Herr⟩
    ihave Hh := Hclose $$ %h.From' Hfield
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold L1MessageFieldOutcome
    iright
    iexists 0, (L1MessageFieldFailure.address reason), err
    isimp only [L1MessageFieldFailureError, Int.natCast_zero, show v.pos' + W64 0 = v.pos' from by word]
    iframe Hc Herr
    isplitl [Hh Hchildren]
    · unfold L1MessageOwned; iexists h; iframe Hh Hchildren
    ipureintro
    exact ⟨Hresult, .sender Hspec.1, by omega, hp.2⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_l1Message_recipient_complete
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_l1Message_payload_complete

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_l1Message_sender_complete

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_l1Message_nonce_complete
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_l1Message_selector_complete
