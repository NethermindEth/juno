import ProofCheck.ReceiptValues
import ProofCheck.ExecutionPointerComplete
import ProofCheck.L1MessagePointerComplete
import ProofCheck.EventPointersComplete
import ProofCheck.L2MessagePointersComplete
import ProofCheck.FeeUnitComplete
import ProofCheck.BoolComplete
import ProofCheck.StringComplete

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

/-- Scalars agree with the independent receipt. Each pointer/slice owns its exact
    children, including shared resource slots and pointer-slice record storage. -/
def ReceiptChildren (h : TransactionReceipt) (value : ReceiptValue) : IProp GF := iprop(
  ⌜h.FeeUnit' = value.unit ∧ h.Reverted' = value.reverted ∧ h.RevertReason' = value.reason⌝ ∗
  FeltResult h.Fee' value.fee ∗ EventPointersResult h.Events' value.events ∗
  ExecutionPointerOwned h.ExecutionResources' value.resources ∗ L1MessagePointerOwned h.L1ToL2Message' value.message ∗
  L2MessagePointersResult h.L2ToL1Message' value.messages ∗ FeltResult h.TransactionHash' value.hash)

def ReceiptOwned (dst : Loc) (value : ReceiptValue) : IProp GF := iprop(
  ∃ h : TransactionReceipt, dst ↦ h ∗ ReceiptChildren h value)

theorem receipt_fee_acc (dst : Loc) (h : TransactionReceipt) :
    dst ↦ h ⊢ (structFieldRef TransactionReceipt go!"Fee" dst ↦ h.Fee' ∗
      (∀ value : Loc, structFieldRef TransactionReceipt go!"Fee" dst ↦ value -∗
        dst ↦ {h with Fee' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hfee, Hunit, Hevents, Hresources, Hmessage, Hmessages, Hhash, Hreverted, Hreason, -⟩
  iframe Hfee
  iintro %value Hfee
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hfee Hunit Hevents Hresources Hmessage Hmessages Hhash Hreverted Hreason

theorem receipt_unit_acc (dst : Loc) (h : TransactionReceipt) :
    dst ↦ h ⊢ (structFieldRef TransactionReceipt go!"FeeUnit" dst ↦ h.FeeUnit' ∗
      (∀ value : w8, structFieldRef TransactionReceipt go!"FeeUnit" dst ↦ value -∗
        dst ↦ {h with FeeUnit' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hfee, Hunit, Hevents, Hresources, Hmessage, Hmessages, Hhash, Hreverted, Hreason, -⟩
  iframe Hunit
  iintro %value Hunit
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hfee Hunit Hevents Hresources Hmessage Hmessages Hhash Hreverted Hreason

theorem receipt_events_acc (dst : Loc) (h : TransactionReceipt) :
    dst ↦ h ⊢ (structFieldRef TransactionReceipt go!"Events" dst ↦ h.Events' ∗
      (∀ value : GoSlice, structFieldRef TransactionReceipt go!"Events" dst ↦ value -∗
        dst ↦ {h with Events' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hfee, Hunit, Hevents, Hresources, Hmessage, Hmessages, Hhash, Hreverted, Hreason, -⟩
  iframe Hevents
  iintro %value Hevents
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hfee Hunit Hevents Hresources Hmessage Hmessages Hhash Hreverted Hreason

theorem receipt_resources_acc (dst : Loc) (h : TransactionReceipt) :
    dst ↦ h ⊢ (structFieldRef TransactionReceipt go!"ExecutionResources" dst ↦ h.ExecutionResources' ∗
      (∀ value : Loc, structFieldRef TransactionReceipt go!"ExecutionResources" dst ↦ value -∗
        dst ↦ {h with ExecutionResources' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hfee, Hunit, Hevents, Hresources, Hmessage, Hmessages, Hhash, Hreverted, Hreason, -⟩
  iframe Hresources
  iintro %value Hresources
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hfee Hunit Hevents Hresources Hmessage Hmessages Hhash Hreverted Hreason

theorem receipt_message_acc (dst : Loc) (h : TransactionReceipt) :
    dst ↦ h ⊢ (structFieldRef TransactionReceipt go!"L1ToL2Message" dst ↦ h.L1ToL2Message' ∗
      (∀ value : Loc, structFieldRef TransactionReceipt go!"L1ToL2Message" dst ↦ value -∗
        dst ↦ {h with L1ToL2Message' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hfee, Hunit, Hevents, Hresources, Hmessage, Hmessages, Hhash, Hreverted, Hreason, -⟩
  iframe Hmessage
  iintro %value Hmessage
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hfee Hunit Hevents Hresources Hmessage Hmessages Hhash Hreverted Hreason

theorem receipt_messages_acc (dst : Loc) (h : TransactionReceipt) :
    dst ↦ h ⊢ (structFieldRef TransactionReceipt go!"L2ToL1Message" dst ↦ h.L2ToL1Message' ∗
      (∀ value : GoSlice, structFieldRef TransactionReceipt go!"L2ToL1Message" dst ↦ value -∗
        dst ↦ {h with L2ToL1Message' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hfee, Hunit, Hevents, Hresources, Hmessage, Hmessages, Hhash, Hreverted, Hreason, -⟩
  iframe Hmessages
  iintro %value Hmessages
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hfee Hunit Hevents Hresources Hmessage Hmessages Hhash Hreverted Hreason

theorem receipt_hash_acc (dst : Loc) (h : TransactionReceipt) :
    dst ↦ h ⊢ (structFieldRef TransactionReceipt go!"TransactionHash" dst ↦ h.TransactionHash' ∗
      (∀ value : Loc, structFieldRef TransactionReceipt go!"TransactionHash" dst ↦ value -∗
        dst ↦ {h with TransactionHash' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hfee, Hunit, Hevents, Hresources, Hmessage, Hmessages, Hhash, Hreverted, Hreason, -⟩
  iframe Hhash
  iintro %value Hhash
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hfee Hunit Hevents Hresources Hmessage Hmessages Hhash Hreverted Hreason

theorem receipt_reverted_acc (dst : Loc) (h : TransactionReceipt) :
    dst ↦ h ⊢ (structFieldRef TransactionReceipt go!"Reverted" dst ↦ h.Reverted' ∗
      (∀ value : Bool, structFieldRef TransactionReceipt go!"Reverted" dst ↦ value -∗
        dst ↦ {h with Reverted' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hfee, Hunit, Hevents, Hresources, Hmessage, Hmessages, Hhash, Hreverted, Hreason, -⟩
  iframe Hreverted
  iintro %value Hreverted
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hfee Hunit Hevents Hresources Hmessage Hmessages Hhash Hreverted Hreason

theorem receipt_reason_acc (dst : Loc) (h : TransactionReceipt) :
    dst ↦ h ⊢ (structFieldRef TransactionReceipt go!"RevertReason" dst ↦ h.RevertReason' ∗
      (∀ value : GoString, structFieldRef TransactionReceipt go!"RevertReason" dst ↦ value -∗
        dst ↦ {h with RevertReason' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hfee, Hunit, Hevents, Hresources, Hmessage, Hmessages, Hhash, Hreverted, Hreason, -⟩
  iframe Hreason
  iintro %value Hreason
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hfee Hunit Hevents Hresources Hmessage Hmessages Hhash Hreverted Hreason

theorem receiptChildren_fee (h : TransactionReceipt) (old : ReceiptValue) (out : Loc) (value : Option (GoArray w64 4)) :
    ReceiptChildren h old ∗ FeltResult out value ⊢
      (ReceiptChildren {h with Fee' := out} {old with fee := value} : IProp GF) := by
  unfold ReceiptChildren
  iintro ⟨⟨%Hscalar, Hfee, Hevents, Hresources, Hmessage, Hmessages, Hhash⟩, Hnew⟩
  iframe Hnew Hevents Hresources Hmessage Hmessages Hhash
  ipureintro; exact Hscalar

theorem receiptChildren_unit (h : TransactionReceipt) (old : ReceiptValue) (value : w8) :
    ReceiptChildren h old ⊢ (ReceiptChildren {h with FeeUnit' := value} {old with unit := value} : IProp GF) := by
  unfold ReceiptChildren
  iintro ⟨%Hscalar, Hfee, Hevents, Hresources, Hmessage, Hmessages, Hhash⟩
  iframe Hfee Hevents Hresources Hmessage Hmessages Hhash
  ipureintro; exact ⟨rfl, Hscalar.2.1, Hscalar.2.2⟩

theorem receiptChildren_events (h : TransactionReceipt) (old : ReceiptValue) (out : GoSlice) (value : Option (List (Option EventValue))) :
    ReceiptChildren h old ∗ EventPointersResult out value ⊢
      (ReceiptChildren {h with Events' := out} {old with events := value} : IProp GF) := by
  unfold ReceiptChildren
  iintro ⟨⟨%Hscalar, Hfee, Hevents, Hresources, Hmessage, Hmessages, Hhash⟩, Hnew⟩
  iframe Hnew Hfee Hresources Hmessage Hmessages Hhash
  ipureintro; exact Hscalar

theorem receiptChildren_resources (h : TransactionReceipt) (old : ReceiptValue) (out : Loc) (value : Option ExecutionValue) :
    ReceiptChildren h old ∗ ExecutionPointerOwned out value ⊢
      (ReceiptChildren {h with ExecutionResources' := out} {old with resources := value} : IProp GF) := by
  unfold ReceiptChildren
  iintro ⟨⟨%Hscalar, Hfee, Hevents, Hresources, Hmessage, Hmessages, Hhash⟩, Hnew⟩
  iframe Hnew Hfee Hevents Hmessage Hmessages Hhash
  ipureintro; exact Hscalar

theorem receiptChildren_message (h : TransactionReceipt) (old : ReceiptValue) (out : Loc) (value : Option L1MessageValue) :
    ReceiptChildren h old ∗ L1MessagePointerOwned out value ⊢
      (ReceiptChildren {h with L1ToL2Message' := out} {old with message := value} : IProp GF) := by
  unfold ReceiptChildren
  iintro ⟨⟨%Hscalar, Hfee, Hevents, Hresources, Hmessage, Hmessages, Hhash⟩, Hnew⟩
  iframe Hnew Hfee Hevents Hresources Hmessages Hhash
  ipureintro; exact Hscalar

theorem receiptChildren_messages (h : TransactionReceipt) (old : ReceiptValue) (out : GoSlice) (value : Option (List (Option L2MessageValue))) :
    ReceiptChildren h old ∗ L2MessagePointersResult out value ⊢
      (ReceiptChildren {h with L2ToL1Message' := out} {old with messages := value} : IProp GF) := by
  unfold ReceiptChildren
  iintro ⟨⟨%Hscalar, Hfee, Hevents, Hresources, Hmessage, Hmessages, Hhash⟩, Hnew⟩
  iframe Hnew Hfee Hevents Hresources Hmessage Hhash
  ipureintro; exact Hscalar

theorem receiptChildren_hash (h : TransactionReceipt) (old : ReceiptValue) (out : Loc) (value : Option (GoArray w64 4)) :
    ReceiptChildren h old ∗ FeltResult out value ⊢
      (ReceiptChildren {h with TransactionHash' := out} {old with hash := value} : IProp GF) := by
  unfold ReceiptChildren
  iintro ⟨⟨%Hscalar, Hfee, Hevents, Hresources, Hmessage, Hmessages, Hhash⟩, Hnew⟩
  iframe Hnew Hfee Hevents Hresources Hmessage Hmessages
  ipureintro; exact Hscalar

theorem receiptChildren_reverted (h : TransactionReceipt) (old : ReceiptValue) (value : Bool) :
    ReceiptChildren h old ⊢ (ReceiptChildren {h with Reverted' := value} {old with reverted := value} : IProp GF) := by
  unfold ReceiptChildren
  iintro ⟨%Hscalar, Hfee, Hevents, Hresources, Hmessage, Hmessages, Hhash⟩
  iframe Hfee Hevents Hresources Hmessage Hmessages Hhash
  ipureintro; exact ⟨Hscalar.1, rfl, Hscalar.2.2⟩

theorem receiptChildren_reason (h : TransactionReceipt) (old : ReceiptValue) (value : GoString) :
    ReceiptChildren h old ⊢ (ReceiptChildren {h with RevertReason' := value} {old with reason := value} : IProp GF) := by
  unfold ReceiptChildren
  iintro ⟨%Hscalar, Hfee, Hevents, Hresources, Hmessage, Hmessages, Hhash⟩
  iframe Hfee Hevents Hresources Hmessage Hmessages Hhash
  ipureintro; exact ⟨Hscalar.1, Hscalar.2.1, rfl⟩

theorem receiptChildren_zero : ⊢ (ReceiptChildren (zero_val TransactionReceipt) receiptZero : IProp GF) := by
  isimp only [ReceiptChildren, receiptZero, FeltResult, EventPointersResult, ExecutionPointerOwned,
    L1MessagePointerOwned, L2MessagePointersResult]
  ipureintro
  exact ⟨⟨rfl, rfl, rfl⟩, rfl, rfl, rfl, rfl, rfl, rfl⟩

theorem receiptOwned_zero (dst : Loc) :
    dst ↦ zero_val TransactionReceipt ⊢ (ReceiptOwned dst receiptZero : IProp GF) := by
  iintro Hdst
  unfold ReceiptOwned
  iexists zero_val TransactionReceipt
  iframe Hdst
  iapply receiptChildren_zero

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receipt_fee_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.receipt_unit_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.receipt_events_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.receipt_resources_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.receipt_message_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.receipt_messages_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.receipt_hash_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.receipt_reverted_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.receipt_reason_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptChildren_fee
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptChildren_unit
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptChildren_events
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptChildren_resources
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptChildren_message
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptChildren_messages
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptChildren_hash
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptChildren_reverted
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptChildren_reason
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptChildren_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptOwned_zero
