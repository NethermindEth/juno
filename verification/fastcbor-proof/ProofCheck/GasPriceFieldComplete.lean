import ProofCheck.GasPriceSlot

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

/-- Decoding either GasPrice amount updates only that pointer field on success,
    owning the resulting felt and exact suffix. Failure preserves the whole
    struct and cursor and owns the precise diagnostic. -/
def GasPriceFieldOutcome (c dst : Loc) (g : GasPrice) (wei : Bool) (v : cursor.Cursor) (xs wire : List w8)
    (d : Nat) (kinds : FeltErrorKinds) (result : GoInterface) : IProp GF := iprop(
  (∃ (item rest : List w8) (value : Option (GoArray w64 4)) (p : Loc),
    dst ↦ gasPriceSet wei g p ∗ FeltResult (GF := GF) p value ∗ c ↦ {v with pos' := v.pos' + W64 item.length} ∗
    ⌜result = interface.nil ∧ wire = item ++ rest ∧ FeltValid d item value ∧
      sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
      sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝) ∨
  (∃ (offset : Nat) (reason : FeltFailureReason) (err : Loc), dst ↦ g ∗ c ↦ v ∗
    FeltFailureError err (v.pos' + W64 offset) reason (feltFailureKind reason kinds) ∗
    ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
      FeltFailure d wire offset reason ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝))

/-- The actual felt call in either GasPrice field branch has a complete
    arbitrary-input outcome. Every child proof is discharged; the enclosing
    GasPrice map dispatch and shared-slot wrapper are separate obligations. -/
theorem wp_gasPrice_field_complete (c dst : Loc) (g : GasPrice) (wei : Bool) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat) (kinds : FeltErrorKinds)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       FeltErrorGlobals kinds ∗ dst ↦ g ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #(structFieldRef GasPrice (gasPriceFieldName wei) dst)))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ GasPriceFieldOutcome c dst g wei v xs wire d kinds result }} := by
  wp_start_folded as ⟨Hglobals, Hdst, Hc, Hs⟩
  rcases felt_classify d wire hdepth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
  · wp_apply wp_gasPrice_field_valid c dst g wei v xs item rest value dq d hd hdepth valid hp (hbytes.trans split)
      $$ [Hdst Hc Hs] as %p ⟨Hdst, Hvalue, Hc, Hs, %Hnext⟩
    · iframe Hdst Hc Hs
    iapply HΦ
    iframe Hs
    unfold GasPriceFieldOutcome
    ileft
    iexists item, rest, value, p
    iframe Hdst Hvalue Hc
    ipureintro
    exact ⟨rfl, split, valid, Hnext⟩
  · ihave #Hkind := feltErrorGlobals_lookup kinds reason $$ Hglobals
    wp_apply wp_gasPrice_field_failure c dst g wei v xs wire dq d offset reason (feltFailureKind reason kinds)
      failure hd hdepth hp hbytes $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr, %Hoffset⟩
    · iframe Hkind Hdst Hc Hs
    iapply HΦ
    iframe Hs
    unfold GasPriceFieldOutcome
    iright
    iexists offset, reason, err
    iframe Hdst Hc Herr
    ipureintro
    exact ⟨rfl, failure, Hoffset⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_gasPrice_field_complete
