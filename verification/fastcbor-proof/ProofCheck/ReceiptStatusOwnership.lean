import ProofCheck.ReceiptStatusValues
import ProofCheck.HeaderProjectionOwnership

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

theorem discardedReceiptSkeleton_eq_zero (value : discardedReceiptSkeleton) : value = zero_val discardedReceiptSkeleton := by
  cases value
  congr 1 <;> apply discardedCBOR_eq_zero

/-- This is the actual translated Go struct, including its embedded skeleton.
    Strings are immutable values in the pinned semantics. -/
def receiptStatusLayout (value : ReceiptStatusValue) : receiptExecutionStatusProjection :=
  ⟨zero_val discardedReceiptSkeleton, value.reverted, value.reason⟩

def ReceiptStatusOwned (dst : Loc) (value : ReceiptStatusValue) : IProp GF :=
  dst ↦ receiptStatusLayout value

/-- Every concrete initial destination is represented, not just zero values or
    strings that happen to be valid UTF-8. The discarded skeleton has no state. -/
theorem receiptStatusLayout_covers (h : receiptExecutionStatusProjection) :
    h = receiptStatusLayout ⟨h.Reverted', h.RevertReason'⟩ := by
  cases h with
  | mk skeleton reverted reason =>
    have hs := discardedReceiptSkeleton_eq_zero skeleton
    subst skeleton
    rfl

theorem receiptStatusLayout_zero : receiptStatusLayout receiptStatusZero = zero_val receiptExecutionStatusProjection := by rfl

theorem receiptStatusValue_zero_iff (value : ReceiptStatusValue) :
    value = receiptStatusZero ↔ value.reverted = false ∧ value.reason = [] := by
  cases value
  simp [receiptStatusZero]

/-- Borrow the Boolean cell and reconstruct the complete concrete struct after
    the helper call, retaining the string and embedded skeleton. -/
theorem receiptStatusReverted_acc (dst : Loc) (h : receiptExecutionStatusProjection) :
    dst ↦ h ⊢ (structFieldRef receiptExecutionStatusProjection go!"Reverted" dst ↦ h.Reverted' ∗
      (∀ value : Bool, structFieldRef receiptExecutionStatusProjection go!"Reverted" dst ↦ value -∗
        dst ↦ {h with Reverted' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hskeleton, Hfield, Hreason, -⟩
  iframe Hfield
  iintro %value Hvalue
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe

/-- Borrow the string cell while retaining the Boolean and skeleton. -/
theorem receiptStatusReason_acc (dst : Loc) (h : receiptExecutionStatusProjection) :
    dst ↦ h ⊢ (structFieldRef receiptExecutionStatusProjection go!"RevertReason" dst ↦ h.RevertReason' ∗
      (∀ value : GoString, structFieldRef receiptExecutionStatusProjection go!"RevertReason" dst ↦ value -∗
        dst ↦ {h with RevertReason' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hskeleton, Hreverted, Hfield, -⟩
  iframe Hfield
  iintro %value Hvalue
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe

/-- The actual zero guard preserves the destination and accepts exactly the
    abstract zero value, without requiring ownership of an input buffer. -/
theorem wp_zeroReceiptStatus_owned (dst : Loc) (value : ReceiptStatusValue) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ∗
       ReceiptStatusOwned dst value }}
      (App (Val (@! cborZeroReceiptExecutionStatusProjection)) (Val #dst))
    {{ (result : Bool), RET #result; ReceiptStatusOwned dst value ∗ ⌜result = true ↔ value = receiptStatusZero⌝ }} := by
  wp_start as Hdst
  isimp only [ReceiptStatusOwned, receiptStatusLayout] at Hdst
  wp_auto
  cases hrev : value.reverted
  all_goals try simp only [hrev, _root_.decide_true, _root_.decide_false]
  all_goals
    wp_auto
    iapply HΦ
    isimp only [ReceiptStatusOwned, receiptStatusLayout, hrev]
    iframe Hdst
    ipureintro
    simp [receiptStatusValue_zero_iff, hrev]

/-- The actual whole-struct clear works for every old Boolean and string. -/
theorem wp_clearReceiptStatus_owned (dst : Loc) (value : ReceiptStatusValue) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ∗
       ReceiptStatusOwned dst value }}
      (App (Val (@! cborClearReceiptExecutionStatusProjection)) (Val #dst))
    {{ RET #(); ReceiptStatusOwned dst receiptStatusZero }} := by
  wp_start as Hdst
  isimp only [ReceiptStatusOwned, receiptStatusLayout] at Hdst
  wp_auto
  wp_end
  isimp only [ReceiptStatusOwned, receiptStatusLayout_zero]
  iexact Hdst

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.discardedReceiptSkeleton_eq_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusLayout_covers
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusLayout_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusValue_zero_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusReverted_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusReason_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_zeroReceiptStatus_owned
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_clearReceiptStatus_owned
