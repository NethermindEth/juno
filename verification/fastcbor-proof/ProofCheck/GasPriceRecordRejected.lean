import ProofCheck.GasPriceMapRejected
import ProofCheck.GasPriceRecordComplete

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

/-- Every complete independent record rejection is realized by the generated
    reader. The only unresolved helper premise is production UTF-8 validation. -/
theorem wp_readGasPrice_rejected (validator : Utf8ValidatorContract (GF := GF)) (c dst fields : Loc) (g : GasPrice) (initial final : GasPriceValue)
    (v : cursor.Cursor) (f : cursor.Fields) (xs wire : List w8)
    (kinds : FeltErrorKinds) (qf qo qn dq : DFrac) (d consumed offset : Nat) (entered : Bool)
    (reason : GasPriceFailureReason)
    (failure : GasRecordRejected d initial wire final consumed offset entered reason)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborGasPriceFields ↦□ fields ∗ FeltErrorGlobals kinds ∗
       dst ↦ g ∗ GasPriceChildren g initial ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames }}
      (App (App (Val (@! cborReadGasPrice)) (Val #c)) (Val #dst))
    {{ (out : GasPrice) (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ out ∗ GasPriceChildren out final ∗
       c ↦ {v with pos' := v.pos' + W64 consumed, depth' := if entered then v.depth' + W64 1 else v.depth'} ∗
       v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames ∗
       GasPriceFailureError err (v.pos' + W64 offset) reason (gasPriceFailureKind reason kinds) ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len ∧
         d + (if entered then 1 else 0) ≤ 32⌝ }} := by
  wp_start_folded as ⟨#Hglobal, Hglobals, Hdst, Hchildren, Hc, Hs, Hfields, Horder, Hnames⟩
  isimp only [FeltErrorGlobals, SkipErrorGlobals] at Hglobals
  icases Hglobals with ⟨⟨#Hsyntax, #Htruncated, #Hunsupported, #Hdepth⟩, #Htype⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hwhole := input_consumption xs wire [] v.pos' v.buf'.len hp Hlen.1
    (by simpa only [List.append_nil] using hbytes)
  have Hbounds := gasRecordRejected_bounds d consumed offset entered initial final wire reason failure hdepth
  have hprogress : sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len ∧
      d + (if entered then 1 else 0) ≤ 32 := by
    exact ⟨by word, by word, by word, by word, Hbounds.2.2⟩
  cases failure with
  | @opening wire reason failure =>
    ihave #Hkind : (feltFailureAddr (GF := GF) reason ↦□ feltFailureKind reason kinds : IProp GF) $$ []
    · iapply feltErrorGlobals_lookup kinds reason
      unfold FeltErrorGlobals SkipErrorGlobals
      iframe Hsyntax Htruncated Hunsupported Hdepth Htype
    wp_apply wp_readGasPrice_open_failure c dst fields g v xs wire dq d reason (feltFailureKind reason kinds)
      failure hd hp hbytes $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe Hglobal Hkind Hdst Hc Hs
    have hz : v.pos' + W64 0 = v.pos' := by word
    isimp only [GasPriceFailureError, gasPriceFailureKind, Int.natCast_zero, hz, Bool.false_eq_true, ite_false] at HΦ
    iapply HΦ
    iframe Hdst Hchildren Hc Hs Hfields Horder Hnames Herr
    ipureintro
    simpa only [Int.natCast_zero, hz, Bool.false_eq_true, ite_false] using hprogress
  | @map head payload body argument count consumed offset final reason enc major countEq depth fit failure =>
    have argumentEq : argument = W64 count := by word
    subst argument
    wp_apply wp_readGasPrice_map_rejected validator c dst fields g initial v f xs payload body head
      kinds qf qo qn dq d count consumed offset final reason enc major hd depth fit failure hp hbytes
      $$ [Hdst Hchildren Hc Hs Hfields Horder Hnames]
      as %out %err ⟨Hdst, Hchildren, Hc, Hs, Hfields, Horder, Hnames, Herr, %Hpositions⟩
    · iframe Hglobal Hdst Hchildren Hc Hs Hfields Horder Hnames
      unfold FeltErrorGlobals SkipErrorGlobals
      iframe Hsyntax Htruncated Hunsupported Hdepth Htype
    isimp only [Int.natCast_add, Int.natCast_one, ite_true] at HΦ
    iapply HΦ
    iframe Hdst Hchildren Hc Hs Hfields Horder Hnames Herr
    ipureintro
    simpa only [Int.natCast_add, Int.natCast_one, ite_true] using hprogress

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readGasPrice_rejected
