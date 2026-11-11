import ProofCheck.ReceiptEventsValues
import ProofCheck.ReceiptStatusOwnership
import ProofCheck.EventPointersComplete

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

/-- The actual projection owns its Event slice and transaction-hash felt
    separately. The embedded discarded skeleton stays in the struct cell. -/
def ReceiptEventsChildren (h : receiptEventsProjection) (value : ReceiptEventsValue) : IProp GF := iprop(
  EventPointersResult h.Events' value.events ∗ FeltResult h.TransactionHash' value.hash)

def ReceiptEventsOwned (dst : Loc) (value : ReceiptEventsValue) : IProp GF := iprop(
  ∃ h : receiptEventsProjection, dst ↦ h ∗ ReceiptEventsChildren h value)

theorem eventPointersResult_null_iff (out : GoSlice) (values : Option (List (Option EventValue))) :
    EventPointersResult out values ⊢ (⌜out.ptr = null ↔ values = none⌝ : IProp GF) := by
  cases values with
  | none => isimp only [EventPointersResult]; iintro %h; ipureintro; simp [h, slice.nil]
  | some values =>
    isimp only [EventPointersResult, EventPointersOwned]
    iintro ⟨%ptrs, %Hnn, _, _⟩
    ipureintro; simp [Hnn]

theorem eventPointersResult_nil_iff (out : GoSlice) (values : Option (List (Option EventValue))) :
    EventPointersResult out values ⊢ (⌜out = slice.nil ↔ values = none⌝ : IProp GF) := by
  cases values with
  | none => isimp only [EventPointersResult]; iintro %h; ipureintro; simp [h]
  | some values =>
    isimp only [EventPointersResult, EventPointersOwned]
    iintro ⟨%ptrs, %Hnn, _, _⟩
    ipureintro
    have hnil : out ≠ slice.nil := fun eq => Hnn (by simp [eq, slice.nil])
    simp [hnil]

theorem receiptEventsValue_zero_iff (value : ReceiptEventsValue) :
    value = receiptEventsZero ↔ value.events = none ∧ value.hash = none := by
  cases value
  simp [receiptEventsZero]

theorem receiptEventsChildren_zero :
    ⊢ (ReceiptEventsChildren (zero_val receiptEventsProjection) receiptEventsZero : IProp GF) := by
  isimp only [ReceiptEventsChildren, receiptEventsZero, EventPointersResult, FeltResult]
  ipureintro; exact ⟨rfl, rfl⟩

/-- Null pointer headers correspond exactly to absent application fields;
    allocated empty slices and allocated zero felts do not pass the zero guard. -/
theorem receiptEventsChildren_guard (h : receiptEventsProjection) (value : ReceiptEventsValue) :
    ReceiptEventsChildren h value ⊢
      (⌜(h.Events' = slice.nil ∧ h.TransactionHash' = null) ↔ value = receiptEventsZero⌝ : IProp GF) := by
  unfold ReceiptEventsChildren
  iintro ⟨Hevents, Hhash⟩
  ihave %Hevents := eventPointersResult_nil_iff h.Events' value.events $$ Hevents
  ihave %Hhash := feltResult_null_iff h.TransactionHash' value.hash $$ Hhash
  ipureintro
  rw [receiptEventsValue_zero_iff, Hevents, Hhash]

/-- Borrow the Events header, retaining the hash and empty skeleton cells. -/
theorem receiptEventsSlice_acc (dst : Loc) (h : receiptEventsProjection) :
    dst ↦ h ⊢ (iprop(structFieldRef receiptEventsProjection go!"Events" dst ↦ h.Events' ∗
      (∀ value : GoSlice, structFieldRef receiptEventsProjection go!"Events" dst ↦ value -∗
        dst ↦ {h with Events' := value})) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hskeleton, Hfield, Hhash, -⟩
  iframe Hfield
  iintro %value Hfield
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hskeleton Hfield Hhash

/-- Borrow the hash pointer without consuming Event slice ownership. -/
theorem receiptEventsHash_acc (dst : Loc) (h : receiptEventsProjection) :
    dst ↦ h ⊢ (iprop(structFieldRef receiptEventsProjection go!"TransactionHash" dst ↦ h.TransactionHash' ∗
      (∀ value : Loc, structFieldRef receiptEventsProjection go!"TransactionHash" dst ↦ value -∗
        dst ↦ {h with TransactionHash' := value})) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hskeleton, Hevents, Hfield, -⟩
  iframe Hfield
  iintro %value Hfield
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hskeleton Hevents Hfield

theorem receiptEventsChildren_events (h : receiptEventsProjection) (old : ReceiptEventsValue)
    (out : GoSlice) (value : Option (List (Option EventValue))) :
    ReceiptEventsChildren h old ∗ EventPointersResult out value ⊢
      (ReceiptEventsChildren {h with Events' := out} {old with events := value} : IProp GF) := by
  unfold ReceiptEventsChildren
  iintro ⟨⟨_, Hhash⟩, Hevents⟩
  iframe Hevents Hhash

theorem receiptEventsChildren_hash (h : receiptEventsProjection) (old : ReceiptEventsValue)
    (p : Loc) (value : Option (GoArray w64 4)) :
    ReceiptEventsChildren h old ∗ FeltResult p value ⊢
      (ReceiptEventsChildren {h with TransactionHash' := p} {old with hash := value} : IProp GF) := by
  unfold ReceiptEventsChildren
  iintro ⟨⟨Hevents, _⟩, Hhash⟩
  iframe Hevents Hhash

/-- The actual guard checks headers only and preserves every concrete cell. -/
theorem wp_zeroReceiptEvents (dst : Loc) (h : receiptEventsProjection) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ∗ dst ↦ h }}
      (App (Val (@! cborZeroReceiptEventsProjection)) (Val #dst))
    {{ (result : Bool), RET #result; dst ↦ h ∗
       ⌜result = true ↔ h.Events' = slice.nil ∧ h.TransactionHash' = null⌝ }} := by
  wp_start as Hdst
  wp_auto
  by_cases he : h.Events' = slice.nil
  · simp only [he, _root_.decide_true]
    wp_auto
    iapply HΦ
    iframe Hdst
    ipureintro; simp [he]
  · simp only [he, _root_.decide_false]
    wp_auto
    iapply HΦ
    iframe Hdst
    ipureintro; simp [he]

theorem wp_zeroReceiptEvents_owned (dst : Loc) (value : ReceiptEventsValue) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ∗
       ReceiptEventsOwned dst value }}
      (App (Val (@! cborZeroReceiptEventsProjection)) (Val #dst))
    {{ (result : Bool), RET #result; ReceiptEventsOwned dst value ∗ ⌜result = true ↔ value = receiptEventsZero⌝ }} := by
  wp_start_folded as Howned
  isimp only [ReceiptEventsOwned] at Howned
  icases Howned with ⟨%h, Hh, Hchildren⟩
  ihave %Hguard := receiptEventsChildren_guard h value $$ Hchildren
  wp_apply wp_zeroReceiptEvents dst h $$ [Hh] as %result ⟨Hh, %Hresult⟩
  · iframe Hh
  iapply HΦ
  isplitl [Hh Hchildren]
  · unfold ReceiptEventsOwned
    iexists h
    iframe Hh Hchildren
  · ipureintro; exact Hresult.trans Hguard

/-- Whole-struct cleanup clears any partially decoded projection. Child
    allocations become unreachable, as allowed by the garbage-collected model. -/
theorem wp_clearReceiptEvents_owned (dst : Loc) (value : ReceiptEventsValue) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ∗
       ReceiptEventsOwned dst value }}
      (App (Val (@! cborClearReceiptEventsProjection)) (Val #dst))
    {{ RET #(); ReceiptEventsOwned dst receiptEventsZero }} := by
  wp_start as Howned
  isimp only [ReceiptEventsOwned] at Howned
  icases Howned with ⟨%h, Hh, Hchildren⟩
  wp_auto
  iapply HΦ
  unfold ReceiptEventsOwned
  iexists zero_val receiptEventsProjection
  iframe Hh
  iapply receiptEventsChildren_zero

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.eventPointersResult_null_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.eventPointersResult_nil_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsValue_zero_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsChildren_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsChildren_guard
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsSlice_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsHash_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsChildren_events
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsChildren_hash
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_zeroReceiptEvents
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_zeroReceiptEvents_owned
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_clearReceiptEvents_owned
