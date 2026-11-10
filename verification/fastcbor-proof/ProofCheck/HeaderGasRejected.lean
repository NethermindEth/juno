import ProofCheck.HeaderSeen
import ProofCheck.GasPriceSlotRejected

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

/-- Failure through an existing shared pair preserves all Header pointers,
    retains the selected partial value in the shared state, and propagates the
    actual owned diagnostic. All rejections are covered, conditional on the
    validator contract. -/
theorem wp_headerGas_existing_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst shared base fields : Loc) (h : Header) (old : HeaderValue)
    (second : Bool) (prices : GoArray GasPrice 2) (snapshot : HeaderGasSnapshot) (g : GasPrice) (final : GasPriceValue) (seen : w64)
    (v : cursor.Cursor) (f : cursor.Fields) (xs wire : List w8)
    (kinds : FeltErrorKinds) (qf qo qn dq : DFrac) (d consumed offset : Nat) (entered : Bool)
    (reason : GasPriceFailureReason)
    (layout : snapshot.layout (GF := GF) base h old) (unseen : HeaderGasUnseen seen snapshot)
    (hg : prices.arr[(sint.Z (headerGasSlot second)).toNat]? = some g)
    (failure : GasRecordRejected d (snapshot.value second) wire final consumed offset entered reason)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborGasPriceFields ↦□ fields ∗ FeltErrorGlobals kinds ∗
       dst ↦ h ∗ HeaderBody h old ∗ shared ↦ base ∗ GasPricesOwned base prices snapshot.values ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames }}
      (App (App (App (App (Val (@! cborReadGasPriceSlot)) (Val #c))
        (Val #(structFieldRef Header (headerGasName second) dst))) (Val #shared)) (Val #(headerGasSlot second)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ h ∗ HeaderBody h (headerSetGasValue second old (if snapshot.published second then some final else none)) ∗
       shared ↦ base ∗ HeaderGasReady base h (headerSetGasValue second old (if snapshot.published second then some final else none))
         (headerKeySeen seen (some (headerGasKey second))) ∗
       c ↦ {v with pos' := v.pos' + W64 consumed, depth' := if entered then v.depth' + W64 1 else v.depth'} ∗
       v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames ∗
       GasPriceFailureError err (v.pos' + W64 offset) reason (gasPriceFailureKind reason kinds) ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len ∧ d + (if entered then 1 else 0) ≤ 32⌝ }} := by
  wp_start_folded as ⟨#Hglobal, Hglobals, Hh, Hbody, Hshared, Hprices, Hc, Hs, Hfields, Horder, Hnames⟩
  icases headerGas_acc dst h second $$ Hh with ⟨Hfield, Hclose⟩
  wp_apply wp_gasPriceSlot_existing_rejected validator c (structFieldRef Header (headerGasName second) dst) shared
    (headerGasPointer second h) fields (headerGasSlot second) base prices snapshot.values g (snapshot.value second) final
    v f xs wire kinds qf qo qn dq d consumed offset entered reason (headerGasSlot_bound second) hg
    (headerGasSnapshot_get snapshot second) failure hd hdepth hp hbytes
    $$ [Hglobals Hfield Hshared Hprices Hc Hs Hfields Horder Hnames]
    as %err ⟨%out, Hfield, Hshared, Hprices, Hc, Hs, Hfields, Horder, Hnames, Herr, %Hprogress⟩
  · iframe Hglobal Hglobals Hfield Hshared Hprices Hc Hs Hfields Horder Hnames
  ihave Hh := Hclose $$ %(headerGasPointer second h) Hfield
  isimp only [headerSetGasPointer_original] at Hh
  isimp only [headerGasSnapshot_set snapshot second final (snapshot.published second)] at Hprices
  ihave Hstate := headerGasReady_snapshot base _ _ _ (headerKeySeen seen (some (headerGasKey second))) (snapshot.update second final (snapshot.published second))
    (headerGasLayout_failure (GF := GF) snapshot base h old second final layout)
    (headerGasUnseen_write seen snapshot second final (snapshot.published second) unseen) $$ Hprices
  iapply HΦ
  isimp only [headerBody_set_gas_value]
  iframe Hh Hbody Hshared Hstate Hc Hs Hfields Horder Hnames Herr
  ipureintro
  exact Hprogress

/-- A failure after lazy allocation publishes neither pointer. The complete
    partial shared allocation remains accounted for until Header cleanup. -/
theorem wp_headerGas_new_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst shared fields : Loc) (h : Header) (old : HeaderValue)
    (second : Bool) (final : GasPriceValue) (seen : w64) (v : cursor.Cursor) (f : cursor.Fields) (xs wire : List w8)
    (kinds : FeltErrorKinds) (qf qo qn dq : DFrac) (d consumed offset : Nat) (entered : Bool)
    (reason : GasPriceFailureReason)
    (layout : headerGasZeroSnapshot.layout (GF := GF) null h old)
    (failure : GasRecordRejected d ⟨none, none⟩ wire final consumed offset entered reason)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborGasPriceFields ↦□ fields ∗ FeltErrorGlobals kinds ∗
       dst ↦ h ∗ HeaderBody h old ∗ shared ↦ null ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames }}
      (App (App (App (App (Val (@! cborReadGasPriceSlot)) (Val #c))
        (Val #(structFieldRef Header (headerGasName second) dst))) (Val #shared)) (Val #(headerGasSlot second)))
    {{ (err base : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ h ∗ HeaderBody h old ∗ shared ↦ base ∗ HeaderGasReady base h old (headerKeySeen seen (some (headerGasKey second))) ∗
       c ↦ {v with pos' := v.pos' + W64 consumed, depth' := if entered then v.depth' + W64 1 else v.depth'} ∗
       v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames ∗
       GasPriceFailureError err (v.pos' + W64 offset) reason (gasPriceFailureKind reason kinds) ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len ∧ d + (if entered then 1 else 0) ≤ 32⌝ }} := by
  wp_start_folded as ⟨#Hglobal, Hglobals, Hh, Hbody, Hshared, Hc, Hs, Hfields, Horder, Hnames⟩
  icases headerGas_acc dst h second $$ Hh with ⟨Hfield, Hclose⟩
  wp_apply wp_gasPriceSlot_new_rejected validator c (structFieldRef Header (headerGasName second) dst) shared
    (headerGasPointer second h) fields (headerGasSlot second) final
    v f xs wire kinds qf qo qn dq d consumed offset entered reason (headerGasSlot_bound second)
    failure hd hdepth hp hbytes $$ [Hglobals Hfield Hshared Hc Hs Hfields Horder Hnames]
    as %err ⟨%base, %out, Hfield, Hshared, Hprices, Hc, Hs, Hfields, Horder, Hnames, Herr, %Hprogress⟩
  · iframe Hglobal Hglobals Hfield Hshared Hc Hs Hfields Horder Hnames
  ihave Hh := Hclose $$ %(headerGasPointer second h) Hfield
  isimp only [headerSetGasPointer_original] at Hh
  have layout' : (headerGasZeroSnapshot.update second final false).layout (GF := GF) base h old := by
    rcases layout with ⟨hf, hs, vf, vs⟩
    cases second <;> simpa [HeaderGasSnapshot.layout, HeaderGasSnapshot.update, headerGasZeroSnapshot] using And.intro hf (And.intro hs (And.intro vf vs))
  have hvalues := headerGasSnapshot_set headerGasZeroSnapshot second final false
  change (([⟨none, none⟩, ⟨none, none⟩] : List GasPriceValue).set
    (sint.Z (headerGasSlot second)).toNat final) =
    (headerGasZeroSnapshot.update second final false).values at hvalues
  isimp only [hvalues] at Hprices
  ihave Hstate := headerGasReady_snapshot base _ h old (headerKeySeen seen (some (headerGasKey second)))
    (headerGasZeroSnapshot.update second final false) layout'
    (headerGasUnseen_write seen headerGasZeroSnapshot second final false (headerGasUnseen_zero seen)) $$ Hprices
  iapply HΦ
  iframe Hh Hbody Hshared Hstate Hc Hs Hfields Horder Hnames Herr
  ipureintro
  exact Hprogress

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerGas_existing_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerGas_new_rejected
