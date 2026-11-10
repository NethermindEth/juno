import ProofCheck.GasPriceMapSlot
import ProofCheck.GasPriceMapFailure

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

/-- Actual slot wrapper after a value failure in the map. Earlier fields and
    both shared slots remain owned, while the caller's destination pointer is
    unchanged. This closes the child proof for the specified value failures. -/
theorem wp_gasPriceSlot_existing_map_value_failure (c dst shared old fields : Loc) (slot : w64) (base : Loc) (prices : GoArray GasPrice 2)
    (values : List GasPriceValue) (g : GasPrice) (initial : GasPriceValue)
    (v : cursor.Cursor) (f : cursor.Fields) (xs payload wire : List w8) (head : w8)
    (kind errorKind : GoInterface) (qf qo qn dq : DFrac) (d count offset : Nat)
    (entries : List (GasPriceEntry (d + 1))) (key : GasKeyWire) (reason : FeltFailureReason)
    (hslot : 0 ≤ sint.Z slot ∧ sint.Z slot < 2)
    (hg : prices.arr[(sint.Z slot).toNat]? = some g)
    (hv : values[(sint.Z slot).toNat]? = some initial)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (hdepth : d < 32)
    (more : entries.length < count)
    (fit : 2 * count ≤ (entries.flatMap GasPriceEntry.bytes ++ key.bytes ++ wire).length)
    (fresh : GasEntriesBefore (W64 0) entries key.choice)
    (failure : GasPriceValueFailure (d + 1) key.choice wire offset reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ entries.flatMap GasPriceEntry.bytes ++ key.bytes ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborGasPriceFields ↦□ fields ∗ globalAddr cursor.ErrUnsupported ↦□ kind ∗
       feltFailureAddr (GF := GF) reason ↦□ errorKind ∗
       dst ↦ old ∗ shared ↦ base ∗ GasPricesOwned base prices values ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames }}
      (App (App (App (App (Val (@! cborReadGasPriceSlot)) (Val #c)) (Val #dst)) (Val #shared)) (Val #slot))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err); ∃ (out : GasPrice),
       dst ↦ old ∗ shared ↦ base ∗
       GasPricesOwned base (array.mk 2 (prices.arr.set (sint.Z slot).toNat out))
         (values.set (sint.Z slot).toNat (entries.foldl GasPriceEntry.apply initial)) ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length) + W64 key.bytes.length, depth' := v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames ∗
       FeltFailureError err (v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length) + W64 key.bytes.length + W64 offset) reason errorKind ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length) + W64 key.bytes.length) = sint.Z v.pos' + 1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length + key.bytes.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length) + W64 key.bytes.length) ≤ sint.Z v.buf'.len ∧
         sint.Z (v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length) + W64 key.bytes.length + W64 offset) = sint.Z v.pos' + 1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length + key.bytes.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length) + W64 key.bytes.length + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start as ⟨#Hglobal, #Hkind, #HerrorKind, Hdst, Hshared, Hprices, Hc, Hs, Hfields, Horder, Hnames⟩
  ihave %Hnn := gasPrices_owned_not_null base prices values $$ Hprices
  have notNull : ((head :: payload) ++ entries.flatMap GasPriceEntry.bytes ++ key.bytes ++ wire).head? ≠ some (W8 246) := by
    intro h
    have hh : head = W8 246 := Option.some.inj h
    rw [hh] at major
    contradiction
  have notUndefined : ((head :: payload) ++ entries.flatMap GasPriceEntry.bytes ++ key.bytes ++ wire).head? ≠ some (W8 247) := by
    intro h
    have hh : head = W8 247 := Option.some.inj h
    rw [hh] at major
    contradiction
  wp_auto
  wp_apply wp_null_not_nullish c v xs ((head :: payload) ++ entries.flatMap GasPriceEntry.bytes ++ key.bytes ++ wire) dq
    notNull notUndefined hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  simp only [Hnn, _root_.decide_false]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true hslot)]
  wp_auto
  icases gasPrices_owned_acc base prices values
    (sint.Z slot) g initial hslot.1 hg hv $$ Hprices with ⟨Hslot, Hchildren, Hclose⟩
  wp_apply wp_readGasPrice_map_value_failure c (arrayIndexRef GasPrice (sint.Z slot) base) fields
    g initial v f xs payload wire head kind errorKind qf qo qn dq d count offset entries key reason enc major hd hdepth more fit fresh failure hp hbytes
    $$ [Hslot Hchildren Hc Hs Hfields Horder Hnames]
    as %out %err ⟨Hslot, Hchildren, Hc, Hs, Hfields, Horder, Hnames, Herr, %Hprogress⟩
  · iframe Hglobal Hkind HerrorKind Hslot Hchildren Hc Hs Hfields Horder Hnames
  ihave Hprices := Hclose $$ %out %(entries.foldl GasPriceEntry.apply initial) [Hslot Hchildren]
  · iframe Hslot Hchildren
  iapply HΦ
  iexists out
  iframe Hdst Hshared Hprices Hc Hs Hfields Horder Hnames Herr
  ipureintro
  exact Hprogress

/-- Actual slot wrapper after a value failure in the map. Earlier fields and
    both shared slots remain owned, while the caller's destination pointer is
    unchanged. This closes the child proof for the specified value failures. -/
theorem wp_gasPriceSlot_new_map_value_failure (c dst shared old fields : Loc) (slot : w64)
    (v : cursor.Cursor) (f : cursor.Fields) (xs payload wire : List w8) (head : w8)
    (kind errorKind : GoInterface) (qf qo qn dq : DFrac) (d count offset : Nat)
    (entries : List (GasPriceEntry (d + 1))) (key : GasKeyWire) (reason : FeltFailureReason)
    (hslot : 0 ≤ sint.Z slot ∧ sint.Z slot < 2)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (hdepth : d < 32)
    (more : entries.length < count)
    (fit : 2 * count ≤ (entries.flatMap GasPriceEntry.bytes ++ key.bytes ++ wire).length)
    (fresh : GasEntriesBefore (W64 0) entries key.choice)
    (failure : GasPriceValueFailure (d + 1) key.choice wire offset reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ entries.flatMap GasPriceEntry.bytes ++ key.bytes ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborGasPriceFields ↦□ fields ∗ globalAddr cursor.ErrUnsupported ↦□ kind ∗
       feltFailureAddr (GF := GF) reason ↦□ errorKind ∗
       dst ↦ old ∗ shared ↦ null ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames }}
      (App (App (App (App (Val (@! cborReadGasPriceSlot)) (Val #c)) (Val #dst)) (Val #shared)) (Val #slot))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err); ∃ (base : Loc) (out : GasPrice),
       dst ↦ old ∗ shared ↦ base ∗
       GasPricesOwned base (array.mk 2 ((zero_val (GoArray GasPrice 2)).arr.set (sint.Z slot).toNat out))
         (([⟨none, none⟩, ⟨none, none⟩] : List GasPriceValue).set (sint.Z slot).toNat (entries.foldl GasPriceEntry.apply ⟨none, none⟩)) ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length) + W64 key.bytes.length, depth' := v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames ∗
       FeltFailureError err (v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length) + W64 key.bytes.length + W64 offset) reason errorKind ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length) + W64 key.bytes.length) = sint.Z v.pos' + 1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length + key.bytes.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length) + W64 key.bytes.length) ≤ sint.Z v.buf'.len ∧
         sint.Z (v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length) + W64 key.bytes.length + W64 offset) = sint.Z v.pos' + 1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length + key.bytes.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length) + W64 key.bytes.length + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start as ⟨#Hglobal, #Hkind, #HerrorKind, Hdst, Hshared, Hc, Hs, Hfields, Horder, Hnames⟩
  have notNull : ((head :: payload) ++ entries.flatMap GasPriceEntry.bytes ++ key.bytes ++ wire).head? ≠ some (W8 246) := by
    intro h
    have hh : head = W8 246 := Option.some.inj h
    rw [hh] at major
    contradiction
  have notUndefined : ((head :: payload) ++ entries.flatMap GasPriceEntry.bytes ++ key.bytes ++ wire).head? ≠ some (W8 247) := by
    intro h
    have hh : head = W8 247 := Option.some.inj h
    rw [hh] at major
    contradiction
  wp_auto
  wp_apply wp_null_not_nullish c v xs ((head :: payload) ++ entries.flatMap GasPriceEntry.bytes ++ key.bytes ++ wire) dq
    notNull notUndefined hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
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
  wp_apply wp_readGasPrice_map_value_failure c (arrayIndexRef GasPrice (sint.Z slot) base) fields
    (zero_val GasPrice) ⟨none, none⟩ v f xs payload wire head kind errorKind qf qo qn dq d count offset entries key reason enc major hd hdepth more fit fresh failure hp hbytes
    $$ [Hslot Hchildren Hc Hs Hfields Horder Hnames]
    as %out %err ⟨Hslot, Hchildren, Hc, Hs, Hfields, Horder, Hnames, Herr, %Hprogress⟩
  · iframe Hglobal Hkind HerrorKind Hslot Hchildren Hc Hs Hfields Horder Hnames
  ihave Hprices := Hclose $$ %out %(entries.foldl GasPriceEntry.apply (GasPriceValue.mk none none)) [Hslot Hchildren]
  · iframe Hslot Hchildren
  iapply HΦ
  iexists base, out
  iframe Hdst Hshared Hprices Hc Hs Hfields Horder Hnames Herr
  ipureintro
  exact Hprogress

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_gasPriceSlot_existing_map_value_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_gasPriceSlot_new_map_value_failure
