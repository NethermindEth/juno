import ProofCheck.GasPriceFields

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

/-- Borrow one shared struct and its nested felt ownership; restoring it with
    a new value updates exactly that slot and its specification entry. -/
theorem gasPrices_owned_acc (base : Loc) (prices : GoArray GasPrice 2) (values : List GasPriceValue)
    (i : Int) (g : GasPrice) (value : GasPriceValue) (hi : 0 ≤ i)
    (hg : prices.arr[i.toNat]? = some g) (hv : values[i.toNat]? = some value) :
    GasPricesOwned base prices values ⊢ (iprop(
      arrayIndexRef GasPrice i base ↦ g ∗ GasPriceChildren g value ∗
      (∀ (g' : GasPrice) (value' : GasPriceValue),
        arrayIndexRef GasPrice i base ↦ g' ∗ GasPriceChildren g' value' -∗
        GasPricesOwned base (array.mk 2 (prices.arr.set i.toNat g')) (values.set i.toNat value'))) : IProp GF) := by
  unfold GasPricesOwned
  iintro ⟨Hprices, Hchildren⟩
  icases array_acc base i (DFrac.own 1) 2 prices g hi hg $$ Hprices with ⟨Hslot, Hclose⟩
  icases BigSepL2.bigSepL2_insert_acc hg hv $$ Hchildren with ⟨Hchild, HchildrenClose⟩
  iframe Hslot Hchild
  iintro %g' %value' ⟨Hslot, Hchild⟩
  ihave Hprices := Hclose $$ %g' Hslot
  ihave Hchildren := HchildrenClose $$ %g' %value' Hchild
  iframe Hprices Hchildren

/-- Zero-initialized GasPrice has nil amounts and requires no child allocation. -/
theorem gasPrice_children_zero : ⊢ (GasPriceChildren (zero_val GasPrice) ⟨none, none⟩ : IProp GF) := by
  unfold GasPriceChildren FeltResult
  ipureintro
  exact ⟨rfl, rfl⟩

/-- Ownership produced by the actual Go allocation instruction used by the
    lazy shared allocation, including both zeroed structs and nil children. -/
theorem wp_newGasPrices :
    {{ (True : IProp GF) }}
      (App (Val (GoInstruction (GoAlloc (go.ArrayType 2 GasPrice.ty))))
        (App (Val (GoInstruction (GoZeroVal (go.ArrayType 2 GasPrice.ty)))) (Val #())))
    {{ (base : Loc), RET #base; GasPricesOwned base (zero_val (GoArray GasPrice 2)) [⟨none, none⟩, ⟨none, none⟩] ∗ ⌜base ≠ null⌝ }} := by
  wp_start
  wp_auto
  wp_alloc base as Hprices
  ihave %Hnn := typedPointsto_not_null _ _ _ $$ Hprices
  iapply HΦ
  iframe %Hnn
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

/-- Null/undefined bypass shared allocation and slot indexing altogether. The
    shared pointer and any existing allocations remain unchanged. -/
theorem wp_gasPriceSlot_null (c dst shared base old : Loc) (slot : w64) (v : cursor.Cursor)
    (xs : List w8) (head : w8) (dq : DFrac) (hp : 0 ≤ sint.Z v.pos')
    (hi : xs[(sint.Z v.pos').toNat]? = some head) (hn : head = W8 246 ∨ head = W8 247) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       dst ↦ old ∗ shared ↦ base ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (App (App (Val (@! cborReadGasPriceSlot)) (Val #c)) (Val #dst)) (Val #shared)) (Val #slot))
    {{ RET #interface.nil; dst ↦ null ∗ shared ↦ base ∗
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hdst, Hshared, Hc, Hs⟩
  wp_auto
  wp_apply wp_null_at c v xs head dq hp hi hn $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.gasPrices_owned_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.gasPrice_children_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_newGasPrices
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_gasPriceSlot_null
