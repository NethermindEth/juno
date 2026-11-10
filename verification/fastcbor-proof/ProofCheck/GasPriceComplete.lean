import ProofCheck.GasPriceRecordRejected

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

/-- Complete record outcomes, including ownership of successful or partially
    decoded children, exact suffixes, retained depth and owned diagnostics. -/
def GasPriceOutcome (c dst : Loc) (initial : GasPriceValue) (v : cursor.Cursor)
    (xs wire : List w8) (d : Nat) (kinds : FeltErrorKinds) (result : GoInterface) : IProp GF := iprop(
  (∃ (item rest : List w8) (final : GasPriceValue) (out : GasPrice),
    dst ↦ out ∗ GasPriceChildren out final ∗ c ↦ {v with pos' := v.pos' + W64 item.length} ∗
    ⌜result = interface.nil ∧ wire = item ++ rest ∧ GasRecordValid d initial item final ∧
      sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
      sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝) ∨
  (∃ (consumed offset : Nat) (entered : Bool) (final : GasPriceValue)
      (reason : GasPriceFailureReason) (out : GasPrice) (err : Loc),
    dst ↦ out ∗ GasPriceChildren out final ∗
    c ↦ {v with pos' := v.pos' + W64 consumed, depth' := if entered then v.depth' + W64 1 else v.depth'} ∗
    GasPriceFailureError err (v.pos' + W64 offset) reason (gasPriceFailureKind reason kinds) ∗
    ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
      GasRecordRejected d initial wire final consumed offset entered reason ∧
      sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len ∧ d + (if entered then 1 else 0) ≤ 32⌝))

/-- Every input to the actual GasPrice reader has the independent specified
    outcome. This is conditional composition until Utf8ValidatorContract is
    discharged; no record-decoder correctness is assumed. -/
theorem wp_readGasPrice_complete (validator : Utf8ValidatorContract (GF := GF))
    (c dst fields : Loc) (g : GasPrice) (initial : GasPriceValue)
    (v : cursor.Cursor) (f : cursor.Fields) (xs wire : List w8)
    (kinds : FeltErrorKinds) (qf qo qn dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (bytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborGasPriceFields ↦□ fields ∗ FeltErrorGlobals kinds ∗
       dst ↦ g ∗ GasPriceChildren g initial ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames }}
      (App (App (Val (@! cborReadGasPrice)) (Val #c)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames ∗
       GasPriceOutcome c dst initial v xs wire d kinds result }} := by
  iintro %Φ ⟨#Hpkg, #Hglobal, Hglobals, Hdst, Hchildren, Hc, Hs, Hfields, Horder, Hnames⟩ HΦ
  rcases gasRecord_classify d initial wire hdepth with
    ⟨item, rest, final, split, valid⟩ | ⟨final, consumed, offset, entered, reason, failure⟩
  · isimp only [FeltErrorGlobals, SkipErrorGlobals] at Hglobals
    icases Hglobals with ⟨⟨_, _, #Hkind, _⟩, _⟩
    wp_apply wp_readGasPrice_valid validator c dst fields g initial final v f xs item rest
      kinds.unsupportedKind qf qo qn dq d valid hd hp (bytes.trans split)
      $$ [Hdst Hchildren Hc Hs Hfields Horder Hnames]
      as %out ⟨Hdst, Hchildren, Hc, Hs, Hfields, Horder, Hnames, %Hnext⟩
    · iframe Hglobal Hkind Hdst Hchildren Hc Hs Hfields Horder Hnames
    iapply HΦ
    iframe Hs Hfields Horder Hnames
    unfold GasPriceOutcome
    ileft
    iexists item, rest, final, out
    iframe Hdst Hchildren Hc
    ipureintro
    exact ⟨rfl, split, valid, Hnext⟩
  · wp_apply wp_readGasPrice_rejected validator c dst fields g initial final v f xs wire
      kinds qf qo qn dq d consumed offset entered reason failure hd hdepth hp bytes
      $$ [Hglobals Hdst Hchildren Hc Hs Hfields Horder Hnames]
      as %out %err ⟨Hdst, Hchildren, Hc, Hs, Hfields, Horder, Hnames, Herr, %Hpositions⟩
    · iframe Hglobal Hglobals Hdst Hchildren Hc Hs Hfields Horder Hnames
    iapply HΦ
    iframe Hs Hfields Horder Hnames
    unfold GasPriceOutcome
    iright
    iexists consumed, offset, entered, final, reason, out, err
    iframe Hdst Hchildren Hc Herr
    ipureintro
    exact ⟨rfl, failure, Hpositions⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readGasPrice_complete
