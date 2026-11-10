import ProofCheck.GasPriceFailureSpec

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

def gasPriceFailureKind (reason : GasPriceFailureReason) (kinds : FeltErrorKinds) : GoInterface :=
  match reason with
  | .inner reason => feltFailureKind reason kinds
  | .duplicate _ => kinds.unsupportedKind
  | .key reason => keyFailureKind reason kinds

/-- One actual-reader contract for the currently specified opening, value, key and
    duplicate failures. Every child proof is discharged; earlier fields remain
    owned, and the cursor/diagnostic/depth are bounded without machine wrap. -/
theorem wp_readGasPrice_failure (c dst fields : Loc) (g : GasPrice) (initial final : GasPriceValue)
    (v : cursor.Cursor) (f : cursor.Fields) (xs wire : List w8)
    (kinds : FeltErrorKinds) (qf qo qn dq : DFrac) (d consumed offset : Nat) (entered : Bool)
    (reason : GasPriceFailureReason)
    (failure : GasPriceFailure d wire initial final consumed offset entered reason)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
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
  have Hbounds := gasPriceFailure_bounds d consumed offset entered wire initial final reason failure hdepth
  have hprogress : sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len ∧
      d + (if entered then 1 else 0) ≤ 32 := by
    exact ⟨by word, by word, by word, by word, Hbounds.2.2⟩
  cases failure with
  | @opening wire reason initial failure =>
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
  | @value count offset head payload tail reason initial entries key enc major depth more fit fresh failure =>
    ihave #Hkind : (feltFailureAddr (GF := GF) reason ↦□ feltFailureKind reason kinds : IProp GF) $$ []
    · iapply feltErrorGlobals_lookup kinds reason
      unfold FeltErrorGlobals SkipErrorGlobals
      iframe Hsyntax Htruncated Hunsupported Hdepth Htype
    wp_apply wp_readGasPrice_map_value_failure c dst fields g initial v f xs payload tail head kinds.unsupportedKind
      (feltFailureKind reason kinds) qf qo qn dq d count offset entries key reason enc major hd depth more fit fresh failure hp hbytes
      $$ [Hdst Hchildren Hc Hs Hfields Horder Hnames]
      as %out %err ⟨Hdst, Hchildren, Hc, Hs, Hfields, Horder, Hnames, Herr, %Hpositions⟩
    · iframe Hglobal Hunsupported Hkind Hdst Hchildren Hc Hs Hfields Horder Hnames
    have hc : v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length + key.bytes.length : Nat) =
        v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length) + W64 key.bytes.length := by word
    have ho : v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length + key.bytes.length + offset : Nat) =
        v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length) + W64 key.bytes.length + W64 offset := by word
    simp only [Int.natCast_add, Int.natCast_one] at hc ho
    isimp only [GasPriceFailureError, gasPriceFailureKind, Int.natCast_add, Int.natCast_one, hc, ho, ite_true] at HΦ
    iapply HΦ
    iframe Hdst Hchildren Hc Hs Hfields Horder Hnames Herr
    ipureintro
    simpa only [Int.natCast_add, Int.natCast_one, hc, ho, ite_true] using hprogress
  | @duplicate count head payload tail wei initial entries key enc major depth more fit beforeDuplicate choice =>
    wp_apply wp_readGasPrice_map_duplicate c dst fields g initial v f xs payload tail head kinds.unsupportedKind qf qo qn dq
      d count entries key wei enc major hd depth more fit beforeDuplicate choice hp hbytes
      $$ [Hdst Hchildren Hc Hs Hfields Horder Hnames]
      as %out %err ⟨%args, Hdst, Hchildren, Hc, Hs, Hfields, Horder, Hnames, Herr, Hargs, %Hpositions⟩
    · iframe Hglobal Hunsupported Hdst Hchildren Hc Hs Hfields Horder Hnames
    have hc : v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length + key.bytes.length : Nat) =
        v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length) + W64 key.bytes.length := by word
    simp only [Int.natCast_add, Int.natCast_one] at hc
    isimp only [GasPriceFailureError, gasPriceFailureKind, Int.natCast_add, Int.natCast_one, hc, ite_true] at HΦ
    iapply HΦ
    iframe Hdst Hchildren Hc Hs Hfields Horder Hnames
    isplitl [Herr Hargs]
    · iexists args
      iframe Herr Hargs
    ipureintro
    simpa only [Int.natCast_add, Int.natCast_one, hc, ite_true] using hprogress
  | @key count head payload tail reason initial entries enc major depth more fit fresh failure =>
    ihave #Hkind : (keyFailureAddr (GF := GF) reason ↦□ keyFailureKind reason kinds : IProp GF) $$ []
    · iapply keyErrorGlobals_lookup kinds reason
      unfold FeltErrorGlobals SkipErrorGlobals
      iframe Hsyntax Htruncated Hunsupported Hdepth Htype
    wp_apply wp_readGasPrice_map_key_failure c dst fields g initial v f xs payload tail head kinds.unsupportedKind
      (keyFailureKind reason kinds) qf qo qn dq d count entries reason enc major hd depth more fit fresh failure hp hbytes
      $$ [Hdst Hchildren Hc Hs Hfields Horder Hnames]
      as %out %err ⟨Hdst, Hchildren, Hc, Hs, Hfields, Horder, Hnames, Herr, %Hpositions⟩
    · iframe Hglobal Hunsupported Hkind Hdst Hchildren Hc Hs Hfields Horder Hnames
    isimp only [GasPriceFailureError, gasPriceFailureKind, Int.natCast_add, Int.natCast_one, ite_true] at HΦ
    iapply HΦ
    iframe Hdst Hchildren Hc Hs Hfields Horder Hnames Herr
    ipureintro
    simpa only [Int.natCast_add, Int.natCast_one, ite_true] using hprogress

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readGasPrice_failure
