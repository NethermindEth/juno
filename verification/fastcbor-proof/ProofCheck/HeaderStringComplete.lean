import ProofCheck.StringComplete

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

/-- Updating ProtocolVersion preserves every other public field and child. -/
theorem headerBody_set_string (h : Header) (old : HeaderValue) (value : GoString) :
    HeaderBody h old ⊢
      (HeaderBody {h with ProtocolVersion' := value} {old with protocolVersion := value} : IProp GF) := by
  unfold HeaderBody
  iintro ⟨%Hscalars, Hchildren⟩
  iframe Hchildren
  ipureintro
  rcases Hscalars with ⟨hn, ht, he, htime, hv, hm⟩
  exact ⟨hn, ht, he, htime, rfl, hm⟩

theorem headerGasReady_set_string (base : Loc) (h : Header) (old : HeaderValue) (value : GoString) (seen : w64) :
    HeaderGasReady (GF := GF) base {h with ProtocolVersion' := value} {old with protocolVersion := value} seen =
      HeaderGasReady base h old seen := rfl

/-- Complete ProtocolVersion field outcomes, retaining every other field and
    the private shared allocation in both success and rejection branches. -/
def HeaderStringOutcome (c dst base : Loc) (h : Header) (old : HeaderValue) (seen : w64)
    (v : cursor.Cursor) (xs wire : List w8) (kinds : FeltErrorKinds) (result : GoInterface) : IProp GF := iprop(
  (∃ (item rest : List w8) (value : GoString), dst ↦ {h with ProtocolVersion' := value} ∗
    HeaderBody {h with ProtocolVersion' := value} {old with protocolVersion := value} ∗
    HeaderGasReady base {h with ProtocolVersion' := value} {old with protocolVersion := value} seen ∗
    c ↦ {v with pos' := v.pos' + W64 item.length} ∗
    ⌜result = interface.nil ∧ wire = item ++ rest ∧ StringValid old.protocolVersion item value ∧
      sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
      sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝) ∨
  (∃ (reason : StringFailureReason) (err : Loc), dst ↦ h ∗ HeaderBody h old ∗ HeaderGasReady base h old seen ∗ c ↦ v ∗
    StringFailureError err v.pos' reason (stringFailureKind reason kinds) ∗
    ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧ StringRejected wire reason⌝))

/-- The actual Header string field call has complete outcomes on arbitrary
    input, conditional on the still-unresolved production validator contract. -/
theorem wp_headerString_complete (validator : Utf8ValidatorContract (GF := GF))
    (c dst base : Loc) (h : Header) (old : HeaderValue) (seen : w64)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (kinds : FeltErrorKinds)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (bytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       FeltErrorGlobals kinds ∗ dst ↦ h ∗ HeaderBody h old ∗ HeaderGasReady base h old seen ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadString)) (Val #c)) (Val #(structFieldRef Header go!"ProtocolVersion" dst)))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ HeaderStringOutcome c dst base h old seen v xs wire kinds result }} := by
  iintro %Φ ⟨#Hpkg, Hglobals, Hh, Hchildren, Hprices, Hc, Hs⟩ HΦ
  isimp only [HeaderBody] at Hchildren
  icases Hchildren with ⟨%Hscalars, Hchildren⟩
  ihave Hbody : HeaderBody h old $$ [Hchildren]
  · unfold HeaderBody
    iframe Hchildren
    ipureintro; exact Hscalars
  have version : h.ProtocolVersion' = old.protocolVersion := Hscalars.2.2.2.2.1
  icases headerString_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
  wp_apply wp_readString_complete validator c (structFieldRef Header go!"ProtocolVersion" dst)
    h.ProtocolVersion' v xs wire dq kinds hp bytes $$ [Hglobals Hfield Hc Hs] as %result ⟨Hs, Hout⟩
  · iframe Hglobals Hfield Hc Hs
  iapply HΦ
  iframe Hs
  unfold StringOutcome HeaderStringOutcome
  icases Hout with (Hsuccess | Hfailure)
  · icases Hsuccess with ⟨%item, %rest, %value, Hfield, Hc, %Hnext⟩
    ihave Hh := Hclose $$ %value Hfield
    ihave Hbody := headerBody_set_string h old value $$ Hbody
    ileft
    iexists item, rest, value
    isimp only [headerGasReady_set_string]
    iframe Hh Hbody Hprices Hc
    ipureintro
    simpa only [version] using Hnext
  · icases Hfailure with ⟨%reason, %err, Hfield, Hc, Herr, %Herror⟩
    ihave Hh := Hclose $$ %h.ProtocolVersion' Hfield
    iright
    iexists reason, err
    iframe Hh Hbody Hprices Hc Herr
    ipureintro; exact Herror

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerBody_set_string
#print axioms Perennial.github_com.NethermindEth.juno.core.headerGasReady_set_string
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerString_complete
