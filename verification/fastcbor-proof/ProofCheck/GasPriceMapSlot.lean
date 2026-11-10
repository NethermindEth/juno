import ProofCheck.GasPriceMap
import ProofCheck.GasPriceShared

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

theorem gasPrices_owned_not_null (base : Loc) (prices : GoArray GasPrice 2) (values : List GasPriceValue) :
    GasPricesOwned base prices values ⊢ (⌜base ≠ null⌝ : IProp GF) := by
  unfold GasPricesOwned
  iintro ⟨Hprices, Hchildren⟩
  iapply typedPointsto_not_null base prices (DFrac.own 1)
  iexact Hprices

theorem gasPrices_zero_owned (base : Loc) :
    base ↦ (zero_val (GoArray GasPrice 2)) ⊢
      (GasPricesOwned base (zero_val (GoArray GasPrice 2)) [⟨none, none⟩, ⟨none, none⟩] : IProp GF) := by
  iintro Hprices
  unfold GasPricesOwned
  iframe Hprices
  change ⊢ (iprop([∗list] g;value ∈ [zero_val GasPrice, zero_val GasPrice];[GasPriceValue.mk none none, GasPriceValue.mk none none], GasPriceChildren g value) : IProp GF)
  iapply BigSepL2.bigSepL2_cons.2
  isplitl []
  · iapply gasPrice_children_zero
  iapply BigSepL2.bigSepL2_cons.2
  isplitl []
  · iapply gasPrice_children_zero
  iapply BigSepL2.bigSepL2_nil.2
  itrivial

/-- Closed success contract for the actual slot wrapper and map decoder in the
    currently proved key families. No child-reader correctness premise remains;
    both shared slots and their nested felt allocations are owned. -/
theorem wp_gasPriceSlot_existing_map_supported (c dst shared old fields : Loc) (slot : w64) (base : Loc) (prices : GoArray GasPrice 2)
    (values : List GasPriceValue) (g : GasPrice) (initial : GasPriceValue)
    (v : cursor.Cursor) (f : cursor.Fields) (xs payload rem : List w8) (head : w8)
    (kind : GoInterface) (qf qo qn dq : DFrac) (d : Nat) (entries : List (GasPriceEntry (d + 1)))
    (hslot : 0 ≤ sint.Z slot ∧ sint.Z slot < 2)
    (hg : prices.arr[(sint.Z slot).toNat]? = some g)
    (hv : values[(sint.Z slot).toNat]? = some initial)
    (enc : HeadArgument head payload (W64 entries.length)) (major : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (hdepth : d < 32) (fresh : GasEntriesFresh (W64 0) entries)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ entries.flatMap GasPriceEntry.bytes ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr internal.cborschema.cborGasPriceFields ↦□ fields ∗ globalAddr cursor.ErrUnsupported ↦□ kind ∗
       dst ↦ old ∗ shared ↦ base ∗ GasPricesOwned base prices values ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames }}
      (App (App (App (App (Val (@! cborReadGasPriceSlot)) (Val #c)) (Val #dst)) (Val #shared)) (Val #slot))
    {{ RET #interface.nil; ∃ (out : GasPrice),
       dst ↦ arrayIndexRef GasPrice (sint.Z slot) base ∗ shared ↦ base ∗
       GasPricesOwned base (array.mk 2 (prices.arr.set (sint.Z slot).toNat out))
         (values.set (sint.Z slot).toNat (entries.foldl GasPriceEntry.apply initial)) ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length)} ∗
       v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length)) =
           sint.Z v.pos' + 1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length)))
           (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨#Hglobal, #Hkind, Hdst, Hshared, Hprices, Hc, Hs, Hfields, Horder, Hnames⟩
  ihave %Hnn := gasPrices_owned_not_null base prices values $$ Hprices
  have notNull : ((head :: payload) ++ entries.flatMap GasPriceEntry.bytes ++ rem).head? ≠ some (W8 246) := by
    intro h
    have hh : head = W8 246 := Option.some.inj h
    rw [hh] at major
    contradiction
  have notUndefined : ((head :: payload) ++ entries.flatMap GasPriceEntry.bytes ++ rem).head? ≠ some (W8 247) := by
    intro h
    have hh : head = W8 247 := Option.some.inj h
    rw [hh] at major
    contradiction
  wp_auto
  wp_apply wp_null_not_nullish c v xs ((head :: payload) ++ entries.flatMap GasPriceEntry.bytes ++ rem) dq
    notNull notUndefined hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  simp only [Hnn, _root_.decide_false]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true hslot)]
  wp_auto
  icases gasPrices_owned_acc base prices values
    (sint.Z slot) g initial hslot.1 hg hv $$ Hprices with ⟨Hslot, Hchildren, Hclose⟩
  wp_apply wp_readGasPrice_map_supported c (arrayIndexRef GasPrice (sint.Z slot) base) fields
    g initial v f xs payload rem head kind qf qo qn dq d entries enc major hd hdepth fresh hp hbytes
    $$ [Hslot Hchildren Hc Hs Hfields Horder Hnames]
    as %out ⟨Hslot, Hchildren, Hc, Hs, Hfields, Horder, Hnames, %Hprogress⟩
  · iframe Hglobal Hkind Hslot Hchildren Hc Hs Hfields Horder Hnames
  ihave Hprices := Hclose $$ %out %(entries.foldl GasPriceEntry.apply initial) [Hslot Hchildren]
  · iframe Hslot Hchildren
  iapply HΦ
  iexists out
  iframe Hdst Hshared Hprices Hc Hs Hfields Horder Hnames
  ipureintro
  exact Hprogress

/-- Closed success contract for the actual slot wrapper and map decoder in the
    currently proved key families. No child-reader correctness premise remains;
    both shared slots and their nested felt allocations are owned. -/
theorem wp_gasPriceSlot_new_map_supported (c dst shared old fields : Loc) (slot : w64)
    (v : cursor.Cursor) (f : cursor.Fields) (xs payload rem : List w8) (head : w8)
    (kind : GoInterface) (qf qo qn dq : DFrac) (d : Nat) (entries : List (GasPriceEntry (d + 1)))
    (hslot : 0 ≤ sint.Z slot ∧ sint.Z slot < 2)
    (enc : HeadArgument head payload (W64 entries.length)) (major : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (hdepth : d < 32) (fresh : GasEntriesFresh (W64 0) entries)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ entries.flatMap GasPriceEntry.bytes ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr internal.cborschema.cborGasPriceFields ↦□ fields ∗ globalAddr cursor.ErrUnsupported ↦□ kind ∗
       dst ↦ old ∗ shared ↦ null ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames }}
      (App (App (App (App (Val (@! cborReadGasPriceSlot)) (Val #c)) (Val #dst)) (Val #shared)) (Val #slot))
    {{ RET #interface.nil; ∃ (base : Loc) (out : GasPrice),
       dst ↦ arrayIndexRef GasPrice (sint.Z slot) base ∗ shared ↦ base ∗
       GasPricesOwned base (array.mk 2 ((zero_val (GoArray GasPrice 2)).arr.set (sint.Z slot).toNat out))
         (([⟨none, none⟩, ⟨none, none⟩] : List GasPriceValue).set (sint.Z slot).toNat (entries.foldl GasPriceEntry.apply ⟨none, none⟩)) ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length)} ∗
       v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length)) =
           sint.Z v.pos' + 1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length)))
           (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨#Hglobal, #Hkind, Hdst, Hshared, Hc, Hs, Hfields, Horder, Hnames⟩
  have notNull : ((head :: payload) ++ entries.flatMap GasPriceEntry.bytes ++ rem).head? ≠ some (W8 246) := by
    intro h
    have hh : head = W8 246 := Option.some.inj h
    rw [hh] at major
    contradiction
  have notUndefined : ((head :: payload) ++ entries.flatMap GasPriceEntry.bytes ++ rem).head? ≠ some (W8 247) := by
    intro h
    have hh : head = W8 247 := Option.some.inj h
    rw [hh] at major
    contradiction
  wp_auto
  wp_apply wp_null_not_nullish c v xs ((head :: payload) ++ entries.flatMap GasPriceEntry.bytes ++ rem) dq
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
  wp_apply wp_readGasPrice_map_supported c (arrayIndexRef GasPrice (sint.Z slot) base) fields
    (zero_val GasPrice) ⟨none, none⟩ v f xs payload rem head kind qf qo qn dq d entries enc major hd hdepth fresh hp hbytes
    $$ [Hslot Hchildren Hc Hs Hfields Horder Hnames]
    as %out ⟨Hslot, Hchildren, Hc, Hs, Hfields, Horder, Hnames, %Hprogress⟩
  · iframe Hglobal Hkind Hslot Hchildren Hc Hs Hfields Horder Hnames
  ihave Hprices := Hclose $$ %out %(entries.foldl GasPriceEntry.apply (GasPriceValue.mk none none)) [Hslot Hchildren]
  · iframe Hslot Hchildren
  iapply HΦ
  iexists base, out
  iframe Hdst Hshared Hprices Hc Hs Hfields Horder Hnames
  ipureintro
  exact Hprogress

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.gasPrices_owned_not_null
#print axioms Perennial.github_com.NethermindEth.juno.core.gasPrices_zero_owned
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_gasPriceSlot_existing_map_supported
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_gasPriceSlot_new_map_supported
