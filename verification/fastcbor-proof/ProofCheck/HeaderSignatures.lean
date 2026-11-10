import ProofCheck.HeaderSeen
import ProofCheck.HeaderOwnership
import ProofCheck.SignaturesComplete

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

/-- Borrow the actual signature slice header, retaining every other field. -/
theorem headerSignatures_acc (dst : Loc) (h : Header) :
    dst ↦ h ⊢ (iprop(
      structFieldRef Header go!"Signatures" dst ↦ h.Signatures' ∗
      (∀ p : GoSlice, structFieldRef Header go!"Signatures" dst ↦ p -∗ dst ↦ {h with Signatures' := p})) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup _ _ _ $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split _ _ _ $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, Hparent, Hnumber, Hroot, Hsequencer, Htransactions, Hevents,
    Htime, Hversion, Heth, Hsignatures, Hstrk, Hmode, HdataGas, Hl2Gas, -⟩
  iframe Hsignatures
  iintro %p Hvalue
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe

/-- Publish the new nested value without changing any other application field. -/
theorem headerBody_set_signatures (h : Header) (old : HeaderValue)
    (p : GoSlice) (value : Option (List SignatureValue)) :
    HeaderBody h old ∗ SignaturesResult p value ⊢
      (HeaderBody {h with Signatures' := p} {old with signatures := value} : IProp GF) := by
  unfold HeaderBody
  iintro ⟨⟨%Hscalars, Hhash, Hparent, Hroot, Hsequencer, Heth, Hsignatures, Hstrk⟩, Hvalue⟩
  iframe
  ipureintro
  exact Hscalars

theorem headerGasState_set_signatures (base : Loc) (h : Header) (old : HeaderValue)
    (p : GoSlice) (value : Option (List SignatureValue)) :
    HeaderGasState (GF := GF) base {h with Signatures' := p} {old with signatures := value} =
      HeaderGasState base h old := by rfl

theorem headerGasReady_set_signatures (base : Loc) (h : Header) (old : HeaderValue)
    (p : GoSlice) (value : Option (List SignatureValue)) (seen : w64) :
    HeaderGasReady (GF := GF) base {h with Signatures' := p} {old with signatures := value} seen =
      HeaderGasReady base h old seen := by rfl

/-- The Header signature branch preserves the other fields and private shared
    prices. On failure the old signature value remains owned, even when parsing
    failed inside a partially constructed nested signature. -/
def HeaderSignaturesOutcome (c dst base : Loc) (h : Header) (old : HeaderValue) (seen : w64) (v : cursor.Cursor) (xs wire : List w8)
    (d : Nat) (kinds : FeltErrorKinds) (result : GoInterface) : IProp GF := iprop(
  (∃ (item rest : List w8) (value : Option (List SignatureValue)) (p : GoSlice),
    dst ↦ {h with Signatures' := p} ∗
    HeaderBody {h with Signatures' := p} {old with signatures := value} ∗
    HeaderGasReady base {h with Signatures' := p} {old with signatures := value} seen ∗ c ↦ {v with pos' := v.pos' + W64 item.length} ∗
    ⌜result = interface.nil ∧ wire = item ++ rest ∧ SignaturesValid d item value ∧
      sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
      sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝) ∨
  (∃ (consumed offset : Nat) (extraDepth : Nat) (reason : FeltFailureReason) (err : Loc), dst ↦ h ∗ HeaderBody h old ∗ HeaderGasReady base h old seen ∗
    c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
    FeltFailureError err (v.pos' + W64 offset) reason (feltFailureKind reason kinds) ∗
    ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
      SignaturesFailure d wire consumed offset extraDepth reason ∧
      sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝))

/-- Compose the already proved arbitrary-input nested reader at its actual
    Header destination. No child-reader correctness premise is assumed. -/
theorem wp_headerSignatures_complete (c dst base : Loc) (h : Header) (old : HeaderValue) (seen : w64)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (d : Nat) (kinds : FeltErrorKinds)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       FeltErrorGlobals kinds ∗ dst ↦ h ∗ HeaderBody h old ∗ HeaderGasReady base h old seen ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadSignatures)) (Val #c)) (Val #(structFieldRef Header go!"Signatures" dst)))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ HeaderSignaturesOutcome c dst base h old seen v xs wire d kinds result }} := by
  wp_start_folded as ⟨Hglobals, Hh, Hchildren, Hprices, Hc, Hs⟩
  icases headerSignatures_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
  wp_apply wp_readSignatures_complete c (structFieldRef Header go!"Signatures" dst) h.Signatures'
    v xs wire dq d kinds hd hdepth hp hbytes $$ [Hglobals Hfield Hc Hs] as %result ⟨Hs, Hout⟩
  · iframe Hglobals Hfield Hc Hs
  iapply HΦ
  iframe Hs
  unfold SignaturesOutcome HeaderSignaturesOutcome
  icases Hout with (Hsuccess | Hfailure)
  · icases Hsuccess with ⟨%item, %rest, %value, %p, Hfield, Hvalue, Hc, %Hnext⟩
    ihave Hh := Hclose $$ %p Hfield
    ihave Hchildren := headerBody_set_signatures h old p value $$ [Hchildren Hvalue]
    · iframe Hchildren Hvalue
    ileft
    iexists item, rest, value, p
    isimp only [headerGasReady_set_signatures]
    iframe Hh Hchildren Hprices Hc
    ipureintro
    exact Hnext
  · icases Hfailure with ⟨%consumed, %offset, %extraDepth, %reason, %err, Hfield, Hc, Herr, %Herror⟩
    ihave Hh := Hclose $$ %h.Signatures' Hfield
    iright
    iexists consumed, offset, extraDepth, reason, err
    iframe Hh Hchildren Hprices Hc Herr
    ipureintro
    exact Herror

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerSignatures_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.headerBody_set_signatures
#print axioms Perennial.github_com.NethermindEth.juno.core.headerGasState_set_signatures
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerSignatures_complete
#print axioms Perennial.github_com.NethermindEth.juno.core.headerGasReady_set_signatures
