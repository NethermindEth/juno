import ProofCheck.LateError
import Perennial.Golang.Theory.String

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 1600000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]



/-- Decode an empty array head, preserving the input byte and cursor scratch space. -/
theorem wp_expect_emptyArray (c : Loc) (data : GoSlice) (dq : DFrac)
    (hdata : data.len = W64 1) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ cursor.Cursor.mk data (W64 0) (W64 0) (zero_val (GoArray w64 4)) ∗ data ↦*{dq} [W8 128] }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"expect")) (Val #(W8 128)))
    {{ RET (PairV #(W64 0) #interface.nil);
       c ↦ cursor.Cursor.mk data (W64 1) (W64 0) (zero_val (GoArray w64 4)) ∗ data ↦*{dq} [W8 128] }} := by
  wp_start as ⟨Hc, Hdata⟩
  wp_auto
  simp only [hdata]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true (by rw [hdata]; decide))]
  wp_apply wp_load_slice_index (t := go.byte) data (sint.Z (W64 0)) [W8 128] dq (W8 128) (by decide)
    $$ [Hdata] with Hdata
  · iframe Hdata; ipureintro; rfl
  simp only [show W8 128 - W8 128 = W8 0 from rfl, show W64 (uint.Z (W8 0)) = W64 0 from rfl]
  wp_end

/-- The four-limb shortcut declines the empty-array encoding without writing the output. -/
theorem wp_shortLimbs_emptyArray (dst : Loc) (data : GoSlice) (dq : DFrac)
    (hdata : data.len = W64 1) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       data ↦*{dq} [W8 128] }}
      (App (App (Val (@! cursor.shortLimbs)) (Val #data)) (Val #dst))
    {{ RET #(W64 0); data ↦*{dq} [W8 128] }} := by
  wp_start as Hdata
  wp_auto
  simp only [hdata]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true (by rw [hdata]; decide))]
  wp_apply wp_load_slice_index (t := go.byte) data (sint.Z (W64 0)) [W8 128] dq (W8 128) (by decide)
    $$ [Hdata] with Hdata
  · iframe Hdata; ipureintro; rfl
  wp_end

/-- The slow limb reader fills all four output words with zero for a shorter empty array. -/
theorem wp_limbsSlow_emptyArray (c dst : Loc) (old : GoArray w64 4) (data : GoSlice) (dq : DFrac)
    (hdata : data.len = W64 1) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       dst ↦ old ∗ c ↦ cursor.Cursor.mk data (W64 0) (W64 0) (zero_val (GoArray w64 4)) ∗ data ↦*{dq} [W8 128] }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"limbsSlow")) (Val #dst))
    {{ RET #interface.nil; dst ↦ (zero_val (GoArray w64 4)) ∗
       c ↦ cursor.Cursor.mk data (W64 1) (W64 0) (zero_val (GoArray w64 4)) ∗ data ↦*{dq} [W8 128] }} := by
  wp_start as ⟨Hdst, Hc, Hdata⟩
  wp_auto
  wp_apply wp_expect_emptyArray c data dq hdata $$ [Hc Hdata] with ⟨Hc, Hdata⟩
  · iframe Hc Hdata
  wp_method_call
  wp_call
  wp_auto
  wp_for
  wp_end

/-- Limbs reaches the real slow decoder after rejecting both shortcuts. -/
theorem wp_limbs_emptyArray (c dst : Loc) (old : GoArray w64 4) (data : GoSlice) (dq : DFrac)
    (hdata : data.len = W64 1) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       dst ↦ old ∗ c ↦ cursor.Cursor.mk data (W64 0) (W64 0) (zero_val (GoArray w64 4)) ∗ data ↦*{dq} [W8 128] }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Limbs")) (Val #dst))
    {{ RET #interface.nil; dst ↦ (zero_val (GoArray w64 4)) ∗
       c ↦ cursor.Cursor.mk data (W64 1) (W64 0) (zero_val (GoArray w64 4)) ∗ data ↦*{dq} [W8 128] }} := by
  wp_start as ⟨Hdst, Hc, Hdata⟩
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hdata
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, Hwf.1, Hwf.2⟩)]
  wp_auto
  simp only [hdata]
  wp_auto
  have hs : slice.slice data w8 (W64 0) (W64 1) = data := by
    rw [← hdata]; exact (slice_slice_trivial data).symm
  rw [hs]
  wp_apply wp_shortLimbs_emptyArray dst data dq hdata $$ [Hdata] with Hdata
  · iframe Hdata
  wp_apply wp_limbsSlow_emptyArray c dst old data dq hdata $$ [Hdst Hc Hdata] with ⟨Hdst, Hc, Hdata⟩
  · iframe Hdst Hc Hdata
  wp_end

/-- The production felt reader allocates a nonnil result and writes through the named-type pointer conversion. -/
theorem wp_readFelt_emptyArray (c dst : Loc) (old : Loc) (data : GoSlice) (dq : DFrac)
    (hdata : data.len = W64 1) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗ dst ↦ old ∗
       c ↦ cursor.Cursor.mk data (W64 0) (W64 0) (zero_val (GoArray w64 4)) ∗ data ↦*{dq} [W8 128] }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ (p : Loc), RET #interface.nil; dst ↦ p ∗ p ↦ (zero_val (GoArray w64 4)) ∗
       c ↦ cursor.Cursor.mk data (W64 1) (W64 0) (zero_val (GoArray w64 4)) ∗ data ↦*{dq} [W8 128] ∗ ⌜p ≠ null⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hdata⟩
  wp_auto
  wp_apply wp_null_other c (cursor.Cursor.mk data (W64 0) (W64 0) (zero_val (GoArray w64 4)))
    [W8 128] (W8 128) dq (by change 0 ≤ sint.Z (W64 0); decide) rfl (by decide) (by decide)
    $$ [Hc Hdata] with ⟨Hc, Hdata⟩
  · iframe Hc Hdata
  wp_apply wp_limbs_emptyArray c «$r0_ptr» (zero_val (GoArray w64 4)) data dq hdata
    $$ [«$r0» Hc Hdata] with ⟨Hv, Hc, Hdata⟩
  · iframe «$r0» Hc Hdata
  ihave %Hnn := typedPointsto_not_null _ _ _ $$ Hv
  iapply HΦ
  iframe Hdst Hv Hc Hdata
  ipureintro; exact Hnn

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_expect_emptyArray
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_shortLimbs_emptyArray
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbsSlow_emptyArray

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbs_emptyArray
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_emptyArray
