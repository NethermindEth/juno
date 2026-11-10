import ProofCheck.HeaderFeltFields

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

/-- A felt-field call either updates exactly one application value and pointer,
    or leaves all Header fields and non-GasPrice children unchanged with an owned
    diagnostic. The private GasPrice state is preserved alongside the body ownership. -/
def HeaderFeltOutcome (c dst base : Loc) (h : Header) (old : HeaderValue) (field : HeaderFeltField)
    (v : cursor.Cursor) (xs wire : List w8) (d : Nat) (kinds : FeltErrorKinds) (result : GoInterface) : IProp GF := iprop(
  (∃ (item rest : List w8) (value : Option (GoArray w64 4)) (p : Loc),
    dst ↦ headerSetFeltPointer field h p ∗
    HeaderBody (headerSetFeltPointer field h p) (headerSetFeltValue field old value) ∗
    HeaderGasState base (headerSetFeltPointer field h p) (headerSetFeltValue field old value) ∗
    c ↦ {v with pos' := v.pos' + W64 item.length} ∗
    ⌜result = interface.nil ∧ wire = item ++ rest ∧ FeltValid d item value ∧
      sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
      sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝) ∨
  (∃ (offset : Nat) (reason : FeltFailureReason) (err : Loc),
    dst ↦ h ∗ HeaderBody h old ∗ HeaderGasState base h old ∗ c ↦ v ∗
    FeltFailureError err (v.pos' + W64 offset) reason (feltFailureKind reason kinds) ∗
    ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
      FeltFailure d wire offset reason ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝))

/-- Every arbitrary input has a specified outcome for the actual felt call in
    any of Header's six felt branches. The enclosing map dispatch, other field
    kinds and public API composition remain separate proof obligations. -/
theorem wp_headerFelt_complete (c dst base : Loc) (h : Header) (old : HeaderValue) (field : HeaderFeltField)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (d : Nat) (kinds : FeltErrorKinds)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       FeltErrorGlobals kinds ∗ dst ↦ h ∗ HeaderBody h old ∗ HeaderGasState base h old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #(structFieldRef Header (headerFeltName field) dst)))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ HeaderFeltOutcome c dst base h old field v xs wire d kinds result }} := by
  wp_start_folded as ⟨Hglobals, Hh, Hchildren, Hprices, Hc, Hs⟩
  rcases felt_classify d wire hdepth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
  · wp_apply wp_headerFelt_valid c dst h old field v xs item rest value dq d hd hdepth valid hp (hbytes.trans split)
      $$ [Hh Hchildren Hc Hs] as %p ⟨Hh, Hchildren, Hc, Hs, %Hnext⟩
    · iframe Hh Hchildren Hc Hs
    iapply HΦ
    iframe Hs
    unfold HeaderFeltOutcome
    ileft
    iexists item, rest, value, p
    isimp only [headerGasState_set_felt]
    iframe Hh Hchildren Hprices Hc
    ipureintro
    exact ⟨trivial, split, valid, Hnext⟩
  · ihave #Hkind := feltErrorGlobals_lookup kinds reason $$ Hglobals
    wp_apply wp_headerFelt_failure c dst h old field v xs wire dq d offset reason (feltFailureKind reason kinds)
      failure hd hdepth hp hbytes $$ [Hh Hchildren Hc Hs] as %err ⟨Hh, Hchildren, Hc, Hs, Herr, %Hoffset⟩
    · iframe Hkind Hh Hchildren Hc Hs
    iapply HΦ
    iframe Hs
    unfold HeaderFeltOutcome
    iright
    iexists offset, reason, err
    iframe Hh Hchildren Hprices Hc Herr
    ipureintro
    exact ⟨rfl, failure, Hoffset⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerFelt_complete
