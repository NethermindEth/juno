import ProofCheck.ReceiptOwnership
import ProofCheck.ReceiptEventsOwnership
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

theorem executionPointerOwned_null_iff (ptr : Loc) (value : Option ExecutionValue) :
    ExecutionPointerOwned ptr value ⊢ (⌜ptr = null ↔ value = none⌝ : IProp GF) := by
  cases value with
  | none => isimp only [ExecutionPointerOwned]; iintro %h; ipureintro; simp [h]
  | some value =>
    isimp only [ExecutionPointerOwned, ExecutionOwned]
    iintro ⟨%availability, %gas, Hdst, _⟩
    ihave %hn := typedPointsto_not_null ptr (executionConcrete availability gas value) (DFrac.own 1) $$ Hdst
    ipureintro; simp [hn]

theorem l1MessagePointerOwned_null_iff (ptr : Loc) (value : Option L1MessageValue) :
    L1MessagePointerOwned ptr value ⊢ (⌜ptr = null ↔ value = none⌝ : IProp GF) := by
  cases value with
  | none => isimp only [L1MessagePointerOwned]; iintro %h; ipureintro; simp [h]
  | some value =>
    isimp only [L1MessagePointerOwned, L1MessageOwned]
    iintro ⟨%h, Hdst, _⟩
    ihave %hn := typedPointsto_not_null ptr h (DFrac.own 1) $$ Hdst
    ipureintro; simp [hn]

theorem l2MessagePointersResult_nil_iff (out : GoSlice) (values : Option (List (Option L2MessageValue))) :
    L2MessagePointersResult out values ⊢ (⌜out = slice.nil ↔ values = none⌝ : IProp GF) := by
  cases values with
  | none => isimp only [L2MessagePointersResult]; iintro %h; ipureintro; simp [h]
  | some values =>
    isimp only [L2MessagePointersResult, L2MessagePointersOwned]
    iintro ⟨%ptrs, %Hnn, _, _⟩
    ipureintro
    have hnil : out ≠ slice.nil := fun eq => Hnn (by simp [eq, slice.nil])
    simp [hnil]

theorem receiptValue_zero_iff (value : ReceiptValue) :
    value = receiptZero ↔ value.fee = none ∧ value.unit = W8 0 ∧ value.events = none ∧
      value.resources = none ∧ value.message = none ∧ value.messages = none ∧ value.hash = none ∧
      value.reverted = false ∧ value.reason = [] := by
  cases value
  simp [receiptZero]

theorem receiptChildren_zero_iff (h : TransactionReceipt) (value : ReceiptValue) :
    ReceiptChildren h value ⊢ (⌜h = zero_val TransactionReceipt ↔ value = receiptZero⌝ : IProp GF) := by
  unfold ReceiptChildren
  iintro ⟨%Hscalars, Hfee, Hevents, Hresources, Hmessage, Hmessages, Hhash⟩
  ihave %Hfee := feltResult_null_iff h.Fee' value.fee $$ Hfee
  ihave %Hevents := eventPointersResult_nil_iff h.Events' value.events $$ Hevents
  ihave %Hresources := executionPointerOwned_null_iff h.ExecutionResources' value.resources $$ Hresources
  ihave %Hmessage := l1MessagePointerOwned_null_iff h.L1ToL2Message' value.message $$ Hmessage
  ihave %Hmessages := l2MessagePointersResult_nil_iff h.L2ToL1Message' value.messages $$ Hmessages
  ihave %Hhash := feltResult_null_iff h.TransactionHash' value.hash $$ Hhash
  ipureintro
  rw [receiptValue_zero_iff]
  have hz : zero_val TransactionReceipt = TransactionReceipt.mk null (W8 0) slice.nil null null slice.nil null false [] := rfl
  rw [hz]
  cases h
  simp only [TransactionReceipt.mk.injEq]
  simp_all

/-- Every actual field participates in the guard, including nil slice headers. -/
theorem wp_zeroReceipt (dst : Loc) (h : TransactionReceipt) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗ dst ↦ h }}
      (App (Val (@! cborZeroTransactionReceipt)) (Val #dst))
    {{ (result : Bool), RET #result; dst ↦ h ∗ ⌜result = true ↔ h = zero_val TransactionReceipt⌝ }} := by
  wp_start as Hdst
  wp_auto
  by_cases h0 : h.Fee' ≠ null
  · have hn : h ≠ zero_val TransactionReceipt := by
      intro heq
      subst h
      exact h0 rfl
    simp only [h0, _root_.decide_false]
    wp_auto
    iapply HΦ
    iframe Hdst
    ipureintro; simp [hn]
  have h0' : h.Fee' = null := Classical.not_not.mp h0
  simp only [h0', _root_.decide_true]
  wp_auto
  by_cases h1 : h.FeeUnit' ≠ W8 0
  · have hn : h ≠ zero_val TransactionReceipt := by
      intro heq
      subst h
      exact h1 rfl
    simp only [h1, _root_.decide_false]
    wp_auto
    iapply HΦ
    iframe Hdst
    ipureintro; simp [hn]
  have h1' : h.FeeUnit' = W8 0 := Classical.not_not.mp h1
  simp only [h1', _root_.decide_true]
  wp_auto
  by_cases h2 : h.Events' ≠ slice.nil
  · have hn : h ≠ zero_val TransactionReceipt := by
      intro heq
      subst h
      exact h2 rfl
    simp only [h2, _root_.decide_false]
    wp_auto
    iapply HΦ
    iframe Hdst
    ipureintro; simp [hn]
  have h2' : h.Events' = slice.nil := Classical.not_not.mp h2
  simp only [h2', _root_.decide_true]
  wp_auto
  by_cases h3 : h.ExecutionResources' ≠ null
  · have hn : h ≠ zero_val TransactionReceipt := by
      intro heq
      subst h
      exact h3 rfl
    simp only [h3, _root_.decide_false]
    wp_auto
    iapply HΦ
    iframe Hdst
    ipureintro; simp [hn]
  have h3' : h.ExecutionResources' = null := Classical.not_not.mp h3
  simp only [h3', _root_.decide_true]
  wp_auto
  by_cases h4 : h.L1ToL2Message' ≠ null
  · have hn : h ≠ zero_val TransactionReceipt := by
      intro heq
      subst h
      exact h4 rfl
    simp only [h4, _root_.decide_false]
    wp_auto
    iapply HΦ
    iframe Hdst
    ipureintro; simp [hn]
  have h4' : h.L1ToL2Message' = null := Classical.not_not.mp h4
  simp only [h4', _root_.decide_true]
  wp_auto
  by_cases h5 : h.L2ToL1Message' ≠ slice.nil
  · have hn : h ≠ zero_val TransactionReceipt := by
      intro heq
      subst h
      exact h5 rfl
    simp only [h5, _root_.decide_false]
    wp_auto
    iapply HΦ
    iframe Hdst
    ipureintro; simp [hn]
  have h5' : h.L2ToL1Message' = slice.nil := Classical.not_not.mp h5
  simp only [h5', _root_.decide_true]
  wp_auto
  by_cases h6 : h.TransactionHash' ≠ null
  · have hn : h ≠ zero_val TransactionReceipt := by
      intro heq
      subst h
      exact h6 rfl
    simp only [h6, _root_.decide_false]
    wp_auto
    iapply HΦ
    iframe Hdst
    ipureintro; simp [hn]
  have h6' : h.TransactionHash' = null := Classical.not_not.mp h6
  simp only [h6', _root_.decide_true]
  wp_auto
  by_cases h7 : h.Reverted' ≠ false
  · have hn : h ≠ zero_val TransactionReceipt := by
      intro heq
      subst h
      exact h7 rfl
    simp only [h7, _root_.decide_false]
    wp_auto
    iapply HΦ
    iframe Hdst
    ipureintro; simp [hn]
  have h7' : h.Reverted' = false := Classical.not_not.mp h7
  simp only [h7', _root_.decide_true]
  wp_auto
  iapply HΦ
  iframe Hdst
  ipureintro
  have hz : zero_val TransactionReceipt = TransactionReceipt.mk null (W8 0) slice.nil null null slice.nil null false [] := rfl
  rw [hz]
  cases h
  simp_all

theorem wp_zeroReceipt_owned (dst : Loc) (value : ReceiptValue) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗
       ReceiptOwned dst value }}
      (App (Val (@! cborZeroTransactionReceipt)) (Val #dst))
    {{ (result : Bool), RET #result; ReceiptOwned dst value ∗ ⌜result = true ↔ value = receiptZero⌝ }} := by
  wp_start_folded as Howned
  isimp only [ReceiptOwned] at Howned
  icases Howned with ⟨%h, Hh, Hchildren⟩
  ihave %Hguard := receiptChildren_zero_iff h value $$ Hchildren
  wp_apply wp_zeroReceipt dst h $$ [Hh] as %result ⟨Hh, %Hresult⟩
  · iframe Hh
  iapply HΦ
  isplitl [Hh Hchildren]
  · unfold ReceiptOwned
    iexists h
    iframe Hh Hchildren
  · ipureintro; exact Hresult.trans Hguard

/-- Whole-struct cleanup clears any partially decoded receipt. Child
    allocations become unreachable, as allowed by the garbage-collected model. -/
theorem wp_clearReceipt_owned (dst : Loc) (value : ReceiptValue) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗
       ReceiptOwned dst value }}
      (App (Val (@! cborClearTransactionReceipt)) (Val #dst))
    {{ RET #(); ReceiptOwned dst receiptZero }} := by
  wp_start as Howned
  isimp only [ReceiptOwned] at Howned
  icases Howned with ⟨%h, Hh, Hchildren⟩
  wp_auto
  iapply HΦ
  unfold ReceiptOwned
  iexists zero_val TransactionReceipt
  iframe Hh
  iapply receiptChildren_zero

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.executionPointerOwned_null_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessagePointerOwned_null_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessagePointersResult_nil_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptValue_zero_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptChildren_zero_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_zeroReceipt
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_zeroReceipt_owned
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_clearReceipt_owned
