import ProofCheck.HeaderOwnership
import ProofCheck.StringFailure

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

/-- Borrow ProtocolVersion while retaining the other fourteen fields. -/
theorem headerString_acc (dst : Loc) (h : Header) :
    dst ↦ h ⊢ (iprop(
      structFieldRef Header go!"ProtocolVersion" dst ↦ h.ProtocolVersion' ∗
      (∀ value : GoString, structFieldRef Header go!"ProtocolVersion" dst ↦ value -∗
        dst ↦ {h with ProtocolVersion' := value})) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup _ _ _ $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split _ _ _ $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, Hparent, Hnumber, Hroot, Hsequencer, Htransactions, Hevents,
    Htime, Hversion, Heth, Hsignatures, Hstrk, Hmode, HdataGas, Hl2Gas, -⟩
  iframe Hversion
  iintro %value Hvalue
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe

/-- Null/undefined preserve the old protocol string and every other field,
    consuming exactly one byte without changing the nesting depth. -/
theorem wp_headerString_null (c dst base : Loc) (h : Header) (old : HeaderValue)
    (v : cursor.Cursor) (xs rem : List w8) (head : w8) (dq : DFrac)
    (hn : head = W8 246 ∨ head = W8 247)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       dst ↦ h ∗ HeaderBody h old ∗ HeaderGasState base h old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadString)) (Val #c)) (Val #(structFieldRef Header go!"ProtocolVersion" dst)))
    {{ RET #interface.nil; dst ↦ h ∗ HeaderBody h old ∗ HeaderGasState base h old ∗
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 1) = sint.Z v.pos' + 1 ∧ sint.Z (v.pos' + W64 1) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start_folded as ⟨Hh, Hbody, Hprices, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs [head] rem v.pos' v.buf'.len hp Hlen.1 hbytes
  simp only [List.length_singleton, Int.natCast_one] at Hnext
  have hi := input_suffix_head xs rem (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  icases headerString_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
  wp_apply wp_readString_null_at c (structFieldRef Header go!"ProtocolVersion" dst) h.ProtocolVersion'
    v xs head dq hp.1 hi hn $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs⟩
  · iframe Hfield Hc Hs
  ihave Hh := Hclose $$ %h.ProtocolVersion' Hfield
  wp_end

/-- Specified errors before UTF-8 validation preserve the complete Header and
    all child ownership, with the exact owned diagnostic at the original offset.
    Successful text and invalid UTF-8 still require the validator contract. -/
theorem wp_headerString_failure (c dst base : Loc) (h : Header) (old : HeaderValue)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (reason : StringFailureReason) (kind : GoInterface)
    (failure : StringFailure wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       stringFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ h ∗ HeaderBody h old ∗ HeaderGasState base h old ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadString)) (Val #c)) (Val #(structFieldRef Header go!"ProtocolVersion" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ h ∗ HeaderBody h old ∗ HeaderGasState base h old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       StringFailureError err v.pos' reason kind }} := by
  wp_start_folded as ⟨#Hkind, Hh, Hbody, Hprices, Hc, Hs⟩
  icases headerString_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
  wp_apply wp_readString_failure c (structFieldRef Header go!"ProtocolVersion" dst) h.ProtocolVersion'
    v xs wire dq reason kind failure hp hbytes $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr⟩
  · iframe Hkind Hfield Hc Hs
  ihave Hh := Hclose $$ %h.ProtocolVersion' Hfield
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerString_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerString_null
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerString_failure
