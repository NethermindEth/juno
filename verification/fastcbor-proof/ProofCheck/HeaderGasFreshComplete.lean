import ProofCheck.HeaderGasFresh
import ProofCheck.HeaderGasMapValid
import ProofCheck.HeaderGasRejected

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

/-- A fresh price field starts from zero independently of whether its sibling
    already allocated the shared pair. Both branches use the actual slot reader. -/
theorem wp_headerGas_fresh_map_valid (validator : Utf8ValidatorContract (GF := GF))
    (c dst shared base fields : Loc) (h : Header) (old : HeaderValue) (second : Bool) (seen : w64)
    (v : cursor.Cursor) (f : cursor.Fields) (xs payload body rem : List w8) (head : w8)
    (kind : GoInterface) (qf qo qn dq : DFrac) (d count : Nat) (final : GasPriceValue)
    (freshField : HeaderKeyFresh seen (some (headerGasKey second)))
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (hdepth : d < 32) (valid : GasPairsValid (d + 1) count [] ⟨none, none⟩ body final)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ body ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborGasPriceFields ↦□ fields ∗ globalAddr cursor.ErrUnsupported ↦□ kind ∗
       dst ↦ h ∗ HeaderBody h old ∗ shared ↦ base ∗ HeaderGasReady base h old seen ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames }}
      (App (App (App (App (Val (@! cborReadGasPriceSlot)) (Val #c))
        (Val #(structFieldRef Header (headerGasName second) dst))) (Val #shared)) (Val #(headerGasSlot second)))
    {{ (outBase : Loc), RET #interface.nil;
       dst ↦ headerSetGasPointer second h (arrayIndexRef GasPrice (sint.Z (headerGasSlot second)) outBase) ∗
       HeaderBody (headerSetGasPointer second h (arrayIndexRef GasPrice (sint.Z (headerGasSlot second)) outBase))
         (headerSetGasValue second old (some final)) ∗
       shared ↦ outBase ∗
       HeaderGasReady outBase (headerSetGasPointer second h (arrayIndexRef GasPrice (sint.Z (headerGasSlot second)) outBase))
         (headerSetGasValue second old (some final))
         (headerKeySeen seen (some (headerGasKey second))) ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (body).length)} ∗
       v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + (body).length)) =
           sint.Z v.pos' + 1 + payload.length + (body).length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + (body).length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + (body).length)))
           (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start_folded as ⟨#Hglobal, #Hkind, Hh, Hbody, Hshared, Hready, Hc, Hs, Hfields, Horder, Hnames⟩
  isimp only [HeaderGasReady] at Hready
  icases Hready with (Hnil | Hallocated)
  · icases Hnil with %Hnil
    rcases Hnil with ⟨rfl, hf, hs, vf, vs⟩
    have layout : headerGasZeroSnapshot.layout (GF := GF) null h old := ⟨hf, hs, vf, vs⟩
    wp_apply wp_headerGas_new_map_valid validator c dst shared fields h old second seen v f xs payload body rem head
      kind qf qo qn dq d count final layout enc major hd hdepth valid hp hbytes
      $$ [Hh Hbody Hshared Hc Hs Hfields Horder Hnames]
      as %outBase ⟨Hh, Hbody, Hshared, Hready, Hc, Hs, Hfields, Horder, Hnames, %Hnext⟩
    · iframe Hglobal Hkind Hh Hbody Hshared Hc Hs Hfields Horder Hnames
    wp_end
  · icases Hallocated with ⟨%prices, %snapshot, Hprices, %Hlayout⟩
    have hz := Hlayout.2 second freshField
    ihave %Hg := headerGas_owned_slot base prices snapshot second $$ Hprices
    rcases Hg with ⟨g, hg⟩
    have hvalid : GasPairsValid (d + 1) count [] (snapshot.value second) body final := by
      simpa only [hz.1] using valid
    wp_apply wp_headerGas_existing_map_valid validator c dst shared base fields h old second prices snapshot g seen
      v f xs payload body rem head kind qf qo qn dq d count final Hlayout.1 Hlayout.2 hg enc major hd hdepth hvalid hp hbytes
      $$ [Hh Hbody Hshared Hprices Hc Hs Hfields Horder Hnames]
      with ⟨Hh, Hbody, Hshared, Hready, Hc, Hs, Hfields, Horder, Hnames, %Hnext⟩
    · iframe Hglobal Hkind Hh Hbody Hshared Hprices Hc Hs Hfields Horder Hnames
    wp_end

/-- Failure in a fresh price field preserves the entire public Header value.
    A newly allocated or reused slot may retain private partial contents, which
    remain owned until cleanup. Every rejection is covered, conditional on the validator contract. -/
theorem wp_headerGas_fresh_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst shared base fields : Loc) (h : Header) (old : HeaderValue)
    (second : Bool) (final : GasPriceValue) (seen : w64) (v : cursor.Cursor) (f : cursor.Fields) (xs wire : List w8)
    (kinds : FeltErrorKinds) (qf qo qn dq : DFrac) (d consumed offset : Nat) (entered : Bool)
    (reason : GasPriceFailureReason)
    (freshField : HeaderKeyFresh seen (some (headerGasKey second)))
    (failure : GasRecordRejected d ⟨none, none⟩ wire final consumed offset entered reason)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborGasPriceFields ↦□ fields ∗ FeltErrorGlobals kinds ∗
       dst ↦ h ∗ HeaderBody h old ∗ shared ↦ base ∗ HeaderGasReady base h old seen ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames }}
      (App (App (App (App (Val (@! cborReadGasPriceSlot)) (Val #c))
        (Val #(structFieldRef Header (headerGasName second) dst))) (Val #shared)) (Val #(headerGasSlot second)))
    {{ (err outBase : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ h ∗ HeaderBody h old ∗ shared ↦ outBase ∗ HeaderGasReady outBase h old (headerKeySeen seen (some (headerGasKey second))) ∗
       c ↦ {v with pos' := v.pos' + W64 consumed, depth' := if entered then v.depth' + W64 1 else v.depth'} ∗
       v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames ∗
       GasPriceFailureError err (v.pos' + W64 offset) reason (gasPriceFailureKind reason kinds) ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len ∧ d + (if entered then 1 else 0) ≤ 32⌝ }} := by
  wp_start_folded as ⟨#Hglobal, Hglobals, Hh, Hbody, Hshared, Hready, Hc, Hs, Hfields, Horder, Hnames⟩
  isimp only [HeaderGasReady] at Hready
  icases Hready with (Hnil | Hallocated)
  · icases Hnil with %Hnil
    rcases Hnil with ⟨rfl, hf, hs, vf, vs⟩
    have layout : headerGasZeroSnapshot.layout (GF := GF) null h old := ⟨hf, hs, vf, vs⟩
    wp_apply wp_headerGas_new_rejected validator c dst shared fields h old second final seen v f xs wire
      kinds qf qo qn dq d consumed offset entered reason layout failure hd hdepth hp hbytes
      $$ [Hglobals Hh Hbody Hshared Hc Hs Hfields Horder Hnames]
      as %err %outBase ⟨Hh, Hbody, Hshared, Hready, Hc, Hs, Hfields, Horder, Hnames, Herr, %Hprogress⟩
    · iframe Hglobal Hglobals Hh Hbody Hshared Hc Hs Hfields Horder Hnames
    wp_end
  · icases Hallocated with ⟨%prices, %snapshot, Hprices, %Hlayout⟩
    have hz := Hlayout.2 second freshField
    have hfailure : GasRecordRejected d (snapshot.value second) wire final consumed offset entered reason := by
      simpa only [hz.1] using failure
    have hnone := headerGasLayout_unpublished (GF := GF) snapshot base h old second Hlayout.1 hz.2
    ihave %Hg := headerGas_owned_slot base prices snapshot second $$ Hprices
    rcases Hg with ⟨g, hg⟩
    wp_apply wp_headerGas_existing_rejected validator c dst shared base fields h old second prices snapshot g final seen
      v f xs wire kinds qf qo qn dq d consumed offset entered reason Hlayout.1 Hlayout.2 hg hfailure hd hdepth hp hbytes
      $$ [Hglobals Hh Hbody Hshared Hprices Hc Hs Hfields Horder Hnames]
      as %err ⟨Hh, Hbody, Hshared, Hready, Hc, Hs, Hfields, Horder, Hnames, Herr, %Hprogress⟩
    · iframe Hglobal Hglobals Hh Hbody Hshared Hprices Hc Hs Hfields Horder Hnames
    isimp only [hz.2, Bool.false_eq_true, ite_false, hnone] at Hbody
    isimp only [hz.2, Bool.false_eq_true, ite_false, hnone] at Hready
    wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerGas_fresh_map_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerGas_fresh_rejected
