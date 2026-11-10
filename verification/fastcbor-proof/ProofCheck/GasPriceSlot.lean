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

/-- Actual allocation reuse and pointer publication in the slot wrapper.
    The GasPrice child-reader premise is explicit and must be discharged by the
    general map-reader proof before claiming a closed GasPrice decoder contract. -/
theorem wp_gasPriceSlot_existing_success (c dst shared old : Loc) (base : Loc) (prices : GoArray GasPrice 2) (g : GasPrice) (slot : w64)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (R : IProp GF) (Q : GasPrice → IProp GF)
    (hslot : 0 ≤ sint.Z slot ∧ sint.Z slot < 2)
    (hlookup : prices.arr[(sint.Z slot).toNat]? = some g)
    (notNull : wire.head? ≠ some (W8 246)) (notUndefined : wire.head? ≠ some (W8 247))
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire)
    (hread : ∀ target : Loc,
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
         R ∗ target ↦ g ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
        (App (App (Val (@! cborReadGasPrice)) (Val #c)) (Val #target))
      {{ (out : GasPrice), RET #interface.nil; target ↦ out ∗ Q out }}) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       R ∗ dst ↦ old ∗ shared ↦ base ∗ base ↦ prices ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (App (App (Val (@! cborReadGasPriceSlot)) (Val #c)) (Val #dst)) (Val #shared)) (Val #slot))
    {{ RET #interface.nil; ∃ out : GasPrice, dst ↦ arrayIndexRef GasPrice (sint.Z slot) base ∗ shared ↦ base ∗
       base ↦ array.mk 2 (prices.arr.set (sint.Z slot).toNat out) ∗ Q out }} := by
  wp_start as ⟨HR, Hdst, Hshared, Hprices, Hc, Hs⟩
  ihave %Hnn := typedPointsto_not_null _ _ _ $$ Hprices
  wp_auto
  wp_apply wp_null_not_nullish c v xs wire dq notNull notUndefined hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  simp only [Hnn, _root_.decide_false]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true hslot)]
  wp_auto
  icases array_acc base (sint.Z slot) (DFrac.own 1) 2 prices g hslot.1 hlookup $$ Hprices with ⟨Hslot, Hclose⟩
  wp_apply hread (arrayIndexRef GasPrice (sint.Z slot) base) $$ [HR Hslot Hc Hs] as %out ⟨Hslot, HQ⟩
  · iframe HR Hslot Hc Hs
  ihave Hprices := Hclose $$ %out Hslot
  iapply HΦ
  iexists out
  iframe Hdst Hshared Hprices HQ

/-- Actual allocation reuse and failure propagation in the slot wrapper.
    The GasPrice child-reader premise is explicit and must be discharged by the
    general map-reader proof before claiming a closed GasPrice decoder contract. -/
theorem wp_gasPriceSlot_existing_failure (c dst shared old : Loc) (base : Loc) (prices : GoArray GasPrice 2) (g : GasPrice) (slot : w64)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (R : IProp GF) (Q : GasPrice → Loc → IProp GF)
    (hslot : 0 ≤ sint.Z slot ∧ sint.Z slot < 2)
    (hlookup : prices.arr[(sint.Z slot).toNat]? = some g)
    (notNull : wire.head? ≠ some (W8 246)) (notUndefined : wire.head? ≠ some (W8 247))
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire)
    (hread : ∀ target : Loc,
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
         R ∗ target ↦ g ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
        (App (App (Val (@! cborReadGasPrice)) (Val #c)) (Val #target))
      {{ (out : GasPrice) (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err); target ↦ out ∗ Q out err }}) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       R ∗ dst ↦ old ∗ shared ↦ base ∗ base ↦ prices ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (App (App (Val (@! cborReadGasPriceSlot)) (Val #c)) (Val #dst)) (Val #shared)) (Val #slot))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err); ∃ out : GasPrice, dst ↦ old ∗ shared ↦ base ∗
       base ↦ array.mk 2 (prices.arr.set (sint.Z slot).toNat out) ∗ Q out err }} := by
  wp_start as ⟨HR, Hdst, Hshared, Hprices, Hc, Hs⟩
  ihave %Hnn := typedPointsto_not_null _ _ _ $$ Hprices
  wp_auto
  wp_apply wp_null_not_nullish c v xs wire dq notNull notUndefined hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  simp only [Hnn, _root_.decide_false]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true hslot)]
  wp_auto
  icases array_acc base (sint.Z slot) (DFrac.own 1) 2 prices g hslot.1 hlookup $$ Hprices with ⟨Hslot, Hclose⟩
  wp_apply hread (arrayIndexRef GasPrice (sint.Z slot) base) $$ [HR Hslot Hc Hs] as %out %err ⟨Hslot, HQ⟩
  · iframe HR Hslot Hc Hs
  ihave Hprices := Hclose $$ %out Hslot
  iapply HΦ
  iexists out
  iframe Hdst Hshared Hprices HQ

/-- Actual lazy allocation and pointer publication in the slot wrapper.
    The GasPrice child-reader premise is explicit and must be discharged by the
    general map-reader proof before claiming a closed GasPrice decoder contract. -/
theorem wp_gasPriceSlot_new_success (c dst shared old : Loc) (slot : w64)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (R : IProp GF) (Q : GasPrice → IProp GF)
    (hslot : 0 ≤ sint.Z slot ∧ sint.Z slot < 2)
    (notNull : wire.head? ≠ some (W8 246)) (notUndefined : wire.head? ≠ some (W8 247))
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire)
    (hread : ∀ target : Loc,
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
         R ∗ target ↦ zero_val GasPrice ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
        (App (App (Val (@! cborReadGasPrice)) (Val #c)) (Val #target))
      {{ (out : GasPrice), RET #interface.nil; target ↦ out ∗ Q out }}) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       R ∗ dst ↦ old ∗ shared ↦ null ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (App (App (Val (@! cborReadGasPriceSlot)) (Val #c)) (Val #dst)) (Val #shared)) (Val #slot))
    {{ RET #interface.nil; ∃ (base : Loc) (out : GasPrice), dst ↦ arrayIndexRef GasPrice (sint.Z slot) base ∗ shared ↦ base ∗
       base ↦ array.mk 2 ((zero_val (GoArray GasPrice 2)).arr.set (sint.Z slot).toNat out) ∗ Q out }} := by
  wp_start as ⟨HR, Hdst, Hshared, Hc, Hs⟩
  wp_auto
  wp_apply wp_null_not_nullish c v xs wire dq notNull notUndefined hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  let base : Loc := «$r0_ptr»
  irename «$r0» => Hprices
  have hlookup : (zero_val (GoArray GasPrice 2)).arr[(sint.Z slot).toNat]? = some (zero_val GasPrice) := by
    change (List.replicate 2 (zero_val GasPrice))[(sint.Z slot).toNat]? = _
    simp only [List.getElem?_replicate]
    simp [show (sint.Z slot).toNat < 2 by omega]
  rw [ite_eq_left_of_eq_true _ _ (eq_true hslot)]
  wp_auto
  icases array_acc base (sint.Z slot) (DFrac.own 1) 2 (zero_val (GoArray GasPrice 2)) (zero_val GasPrice) hslot.1 hlookup $$ Hprices with ⟨Hslot, Hclose⟩
  wp_apply hread (arrayIndexRef GasPrice (sint.Z slot) base) $$ [HR Hslot Hc Hs] as %out ⟨Hslot, HQ⟩
  · iframe HR Hslot Hc Hs
  ihave Hprices := Hclose $$ %out Hslot
  iapply HΦ
  iexists base, out
  iframe Hdst Hshared Hprices HQ

/-- Actual lazy allocation and failure propagation in the slot wrapper.
    The GasPrice child-reader premise is explicit and must be discharged by the
    general map-reader proof before claiming a closed GasPrice decoder contract. -/
theorem wp_gasPriceSlot_new_failure (c dst shared old : Loc) (slot : w64)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (R : IProp GF) (Q : GasPrice → Loc → IProp GF)
    (hslot : 0 ≤ sint.Z slot ∧ sint.Z slot < 2)
    (notNull : wire.head? ≠ some (W8 246)) (notUndefined : wire.head? ≠ some (W8 247))
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire)
    (hread : ∀ target : Loc,
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
         R ∗ target ↦ zero_val GasPrice ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
        (App (App (Val (@! cborReadGasPrice)) (Val #c)) (Val #target))
      {{ (out : GasPrice) (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err); target ↦ out ∗ Q out err }}) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       R ∗ dst ↦ old ∗ shared ↦ null ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (App (App (Val (@! cborReadGasPriceSlot)) (Val #c)) (Val #dst)) (Val #shared)) (Val #slot))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err); ∃ (base : Loc) (out : GasPrice), dst ↦ old ∗ shared ↦ base ∗
       base ↦ array.mk 2 ((zero_val (GoArray GasPrice 2)).arr.set (sint.Z slot).toNat out) ∗ Q out err }} := by
  wp_start as ⟨HR, Hdst, Hshared, Hc, Hs⟩
  wp_auto
  wp_apply wp_null_not_nullish c v xs wire dq notNull notUndefined hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  let base : Loc := «$r0_ptr»
  irename «$r0» => Hprices
  have hlookup : (zero_val (GoArray GasPrice 2)).arr[(sint.Z slot).toNat]? = some (zero_val GasPrice) := by
    change (List.replicate 2 (zero_val GasPrice))[(sint.Z slot).toNat]? = _
    simp only [List.getElem?_replicate]
    simp [show (sint.Z slot).toNat < 2 by omega]
  rw [ite_eq_left_of_eq_true _ _ (eq_true hslot)]
  wp_auto
  icases array_acc base (sint.Z slot) (DFrac.own 1) 2 (zero_val (GoArray GasPrice 2)) (zero_val GasPrice) hslot.1 hlookup $$ Hprices with ⟨Hslot, Hclose⟩
  wp_apply hread (arrayIndexRef GasPrice (sint.Z slot) base) $$ [HR Hslot Hc Hs] as %out %err ⟨Hslot, HQ⟩
  · iframe HR Hslot Hc Hs
  ihave Hprices := Hclose $$ %out Hslot
  iapply HΦ
  iexists base, out
  iframe Hdst Hshared Hprices HQ

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_gasPriceSlot_existing_success
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_gasPriceSlot_existing_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_gasPriceSlot_new_success
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_gasPriceSlot_new_failure
