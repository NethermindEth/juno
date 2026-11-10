import ProofCheck.GasPriceFailureSound
import ProofCheck.GasPriceMapSlot

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

/-- Actual slot wrapper for every currently specified GasPrice failure. The
    old destination pointer and other shared slot are preserved; the selected
    slot owns exactly the partial value specified by the failure relation. -/
theorem wp_gasPriceSlot_existing_specified_failure (c dst shared old fields : Loc) (slot : w64) (base : Loc) (prices : GoArray GasPrice 2)
    (values : List GasPriceValue) (g : GasPrice) (initial final : GasPriceValue)
    (v : cursor.Cursor) (f : cursor.Fields) (xs wire : List w8)
    (kinds : FeltErrorKinds) (qf qo qn dq : DFrac) (d consumed offset : Nat) (entered : Bool)
    (reason : GasPriceFailureReason)
    (hslot : 0 ≤ sint.Z slot ∧ sint.Z slot < 2)
    (hg : prices.arr[(sint.Z slot).toNat]? = some g)
    (hv : values[(sint.Z slot).toNat]? = some initial)
    (failure : GasPriceFailure d wire initial final consumed offset entered reason)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr internal.cborschema.cborGasPriceFields ↦□ fields ∗ FeltErrorGlobals kinds ∗
       dst ↦ old ∗ shared ↦ base ∗ GasPricesOwned base prices values ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames }}
      (App (App (App (App (Val (@! cborReadGasPriceSlot)) (Val #c)) (Val #dst)) (Val #shared)) (Val #slot))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err); ∃ (out : GasPrice),
       dst ↦ old ∗ shared ↦ base ∗
       GasPricesOwned base (array.mk 2 (prices.arr.set (sint.Z slot).toNat out))
         (values.set (sint.Z slot).toNat final) ∗
       c ↦ {v with pos' := v.pos' + W64 consumed, depth' := if entered then v.depth' + W64 1 else v.depth'} ∗
       v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames ∗
       GasPriceFailureError err (v.pos' + W64 offset) reason (gasPriceFailureKind reason kinds) ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len ∧
         d + (if entered then 1 else 0) ≤ 32⌝ }} := by
  wp_start as ⟨#Hglobal, Hglobals, Hdst, Hshared, Hprices, Hc, Hs, Hfields, Horder, Hnames⟩
  ihave %Hnn := gasPrices_owned_not_null base prices values $$ Hprices
  have hn := gasPriceFailure_not_nullish d consumed offset entered wire initial final reason failure
  wp_auto
  wp_apply wp_null_not_nullish c v xs wire dq hn.1 hn.2 hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  simp only [Hnn, _root_.decide_false]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true hslot)]
  wp_auto
  icases gasPrices_owned_acc base prices values
    (sint.Z slot) g initial hslot.1 hg hv $$ Hprices with ⟨Hslot, Hchildren, Hclose⟩
  wp_apply wp_readGasPrice_failure c (arrayIndexRef GasPrice (sint.Z slot) base) fields
    g initial final v f xs wire kinds qf qo qn dq d consumed offset entered reason failure hd hdepth hp hbytes
    $$ [Hglobals Hslot Hchildren Hc Hs Hfields Horder Hnames]
    as %out %err ⟨Hslot, Hchildren, Hc, Hs, Hfields, Horder, Hnames, Herr, %Hprogress⟩
  · iframe Hglobal Hglobals Hslot Hchildren Hc Hs Hfields Horder Hnames
  ihave Hprices := Hclose $$ %out %final [Hslot Hchildren]
  · iframe Hslot Hchildren
  iapply HΦ
  iexists out
  iframe Hdst Hshared Hprices Hc Hs Hfields Horder Hnames Herr
  ipureintro
  exact Hprogress

/-- Actual slot wrapper for every currently specified GasPrice failure. The
    old destination pointer and other shared slot are preserved; the selected
    slot owns exactly the partial value specified by the failure relation. -/
theorem wp_gasPriceSlot_new_specified_failure (c dst shared old fields : Loc) (slot : w64) (final : GasPriceValue)
    (v : cursor.Cursor) (f : cursor.Fields) (xs wire : List w8)
    (kinds : FeltErrorKinds) (qf qo qn dq : DFrac) (d consumed offset : Nat) (entered : Bool)
    (reason : GasPriceFailureReason)
    (hslot : 0 ≤ sint.Z slot ∧ sint.Z slot < 2)
    (failure : GasPriceFailure d wire ⟨none, none⟩ final consumed offset entered reason)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr internal.cborschema.cborGasPriceFields ↦□ fields ∗ FeltErrorGlobals kinds ∗
       dst ↦ old ∗ shared ↦ null ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames }}
      (App (App (App (App (Val (@! cborReadGasPriceSlot)) (Val #c)) (Val #dst)) (Val #shared)) (Val #slot))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err); ∃ (base : Loc) (out : GasPrice),
       dst ↦ old ∗ shared ↦ base ∗
       GasPricesOwned base (array.mk 2 ((zero_val (GoArray GasPrice 2)).arr.set (sint.Z slot).toNat out))
         (([⟨none, none⟩, ⟨none, none⟩] : List GasPriceValue).set (sint.Z slot).toNat final) ∗
       c ↦ {v with pos' := v.pos' + W64 consumed, depth' := if entered then v.depth' + W64 1 else v.depth'} ∗
       v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames ∗
       GasPriceFailureError err (v.pos' + W64 offset) reason (gasPriceFailureKind reason kinds) ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len ∧
         d + (if entered then 1 else 0) ≤ 32⌝ }} := by
  wp_start as ⟨#Hglobal, Hglobals, Hdst, Hshared, Hc, Hs, Hfields, Horder, Hnames⟩
  have hn := gasPriceFailure_not_nullish d consumed offset entered wire (GasPriceValue.mk none none) final reason failure
  wp_auto
  wp_apply wp_null_not_nullish c v xs wire dq hn.1 hn.2 hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  let base : Loc := «$r0_ptr»
  irename «$r0» => Hprices
  ihave Hprices := gasPrices_zero_owned base $$ Hprices
  have hg : (zero_val (GoArray GasPrice 2)).arr[(sint.Z slot).toNat]? = some (zero_val GasPrice) := by
    change (List.replicate 2 (zero_val GasPrice))[(sint.Z slot).toNat]? = _
    simp only [List.getElem?_replicate]
    simp [show (sint.Z slot).toNat < 2 by omega]
  have hv : ([⟨none, none⟩, ⟨none, none⟩] : List GasPriceValue)[(sint.Z slot).toNat]? = some ⟨none, none⟩ := by
    change (List.replicate 2 (GasPriceValue.mk none none))[(sint.Z slot).toNat]? = _
    simp only [List.getElem?_replicate]
    simp [show (sint.Z slot).toNat < 2 by omega]
  rw [ite_eq_left_of_eq_true _ _ (eq_true hslot)]
  wp_auto
  icases gasPrices_owned_acc base (zero_val (GoArray GasPrice 2)) [⟨none, none⟩, ⟨none, none⟩]
    (sint.Z slot) (zero_val GasPrice) ⟨none, none⟩ hslot.1 hg hv $$ Hprices with ⟨Hslot, Hchildren, Hclose⟩
  wp_apply wp_readGasPrice_failure c (arrayIndexRef GasPrice (sint.Z slot) base) fields
    (zero_val GasPrice) ⟨none, none⟩ final v f xs wire kinds qf qo qn dq d consumed offset entered reason failure hd hdepth hp hbytes
    $$ [Hglobals Hslot Hchildren Hc Hs Hfields Horder Hnames]
    as %out %err ⟨Hslot, Hchildren, Hc, Hs, Hfields, Horder, Hnames, Herr, %Hprogress⟩
  · iframe Hglobal Hglobals Hslot Hchildren Hc Hs Hfields Horder Hnames
  ihave Hprices := Hclose $$ %out %final [Hslot Hchildren]
  · iframe Hslot Hchildren
  iapply HΦ
  iexists base, out
  iframe Hdst Hshared Hprices Hc Hs Hfields Horder Hnames Herr
  ipureintro
  exact Hprogress

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_gasPriceSlot_existing_specified_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_gasPriceSlot_new_specified_failure
