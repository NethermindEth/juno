import ProofCheck.ReceiptOwnership

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

def receiptFieldReader : ReceiptField → GoString
  | .fee => cborReadFelt
  | .unit => cborReadFeeUnit
  | .events => cborReadEventPointers
  | .resources => cborReadExecutionResourcesPointer
  | .message => cborReadL1ToL2MessagePointer
  | .messages => cborReadL2ToL1MessagePointers
  | .hash => cborReadFelt
  | .reverted => cborReadBool
  | .reason => cborReadString

theorem receiptChildren_scalars (h : TransactionReceipt) (value : ReceiptValue) :
    ReceiptChildren h value ⊢ (⌜h.FeeUnit' = value.unit ∧ h.Reverted' = value.reverted ∧ h.RevertReason' = value.reason⌝ : IProp GF) := by
  unfold ReceiptChildren
  iintro ⟨%Hscalar, _⟩
  ipureintro; exact Hscalar

/-- Every successful generated field call preserves all other receipt fields and
    owns its exact replacement, including nil/empty and nested allocation shape. -/
theorem wp_receipt_field_valid (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old final : ReceiptValue) (field : ReceiptField) (v : cursor.Cursor)
    (xs item rest : List w8) (dq : DFrac) (d : Nat)
    (valid : ReceiptFieldValid d field old item rest final) (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       ReceiptOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (receiptFieldReader field))) (Val #c)) (Val #(structFieldRef TransactionReceipt (receiptFieldName field) dst)))
    {{ RET #interface.nil; ReceiptOwned dst final ∗
       c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  iintro %Φ ⟨#Hreceipts, #Hschema, Howned, Hc, Hs⟩ HΦ
  isimp only [ReceiptOwned] at Howned
  icases Howned with ⟨%h, Hdst, Hchildren⟩
  ihave %Hscalar := receiptChildren_scalars h old $$ Hchildren
  cases valid with
  | @fee old item rest value valid =>
    simp only [receiptFieldReader, receiptFieldName]
    icases receipt_fee_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFelt_valid c (structFieldRef TransactionReceipt go!"Fee" dst) h.Fee' v xs item rest value dq d hd depth valid hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := receiptChildren_fee h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold ReceiptOwned
      iexists {h with Fee' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @unit old item rest value valid =>
    simp only [receiptFieldReader, receiptFieldName]
    icases receipt_unit_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFeeUnit_valid c (structFieldRef TransactionReceipt go!"FeeUnit" dst) h.FeeUnit' v xs item rest value dq (by simpa only [Hscalar.1] using valid) hp hbytes
      $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %value Hfield
    ihave Hchildren := receiptChildren_unit h old value $$ [Hchildren]
    · iframe Hchildren
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold ReceiptOwned
      iexists {h with FeeUnit' := value}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @events old item rest value valid =>
    simp only [receiptFieldReader, receiptFieldName]
    icases receipt_events_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readEventPointers_valid validator c (structFieldRef TransactionReceipt go!"Events" dst) h.Events' v xs (item ++ rest) rest value dq d valid hd hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    isimp only [List.length_append, Nat.add_sub_cancel] at Hc
    simp only [List.length_append, Nat.add_sub_cancel] at Hnext
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := receiptChildren_events h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold ReceiptOwned
      iexists {h with Events' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @resources old item rest value valid =>
    simp only [receiptFieldReader, receiptFieldName]
    icases receipt_resources_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readExecutionPointer_valid validator c (structFieldRef TransactionReceipt go!"ExecutionResources" dst) h.ExecutionResources' v xs item rest value dq d valid hd hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := receiptChildren_resources h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold ReceiptOwned
      iexists {h with ExecutionResources' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @message old item rest value valid =>
    simp only [receiptFieldReader, receiptFieldName]
    icases receipt_message_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readL1MessagePointer_valid validator c (structFieldRef TransactionReceipt go!"L1ToL2Message" dst) h.L1ToL2Message' v xs item rest value dq d valid hd hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := receiptChildren_message h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold ReceiptOwned
      iexists {h with L1ToL2Message' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @messages old item rest value valid =>
    simp only [receiptFieldReader, receiptFieldName]
    icases receipt_messages_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readL2MessagePointers_valid validator c (structFieldRef TransactionReceipt go!"L2ToL1Message" dst) h.L2ToL1Message' v xs (item ++ rest) rest value dq d valid hd hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    isimp only [List.length_append, Nat.add_sub_cancel] at Hc
    simp only [List.length_append, Nat.add_sub_cancel] at Hnext
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := receiptChildren_messages h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold ReceiptOwned
      iexists {h with L2ToL1Message' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @hash old item rest value valid =>
    simp only [receiptFieldReader, receiptFieldName]
    icases receipt_hash_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFelt_valid c (structFieldRef TransactionReceipt go!"TransactionHash" dst) h.TransactionHash' v xs item rest value dq d hd depth valid hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    ihave Hchildren := receiptChildren_hash h old out value $$ [Hchildren Hnew]
    · iframe Hchildren Hnew
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold ReceiptOwned
      iexists {h with TransactionHash' := out}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @reverted old item rest value valid =>
    simp only [receiptFieldReader, receiptFieldName]
    icases receipt_reverted_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readBool_valid c (structFieldRef TransactionReceipt go!"Reverted" dst) h.Reverted' v xs item rest value dq (by simpa only [Hscalar.2.1] using valid) hp hbytes
      $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %value Hfield
    ihave Hchildren := receiptChildren_reverted h old value $$ [Hchildren]
    · iframe Hchildren
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold ReceiptOwned
      iexists {h with Reverted' := value}
      iframe Hdst Hchildren
    ipureintro; exact Hnext
  | @reason old item rest value valid =>
    simp only [receiptFieldReader, receiptFieldName]
    icases receipt_reason_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readString_valid validator c (structFieldRef TransactionReceipt go!"RevertReason" dst) h.RevertReason' v xs item rest value dq (by simpa only [Hscalar.2.2] using valid) hp hbytes
      $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %value Hfield
    ihave Hchildren := receiptChildren_reason h old value $$ [Hchildren]
    · iframe Hchildren
    iapply HΦ
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold ReceiptOwned
      iexists {h with RevertReason' := value}
      iframe Hdst Hchildren
    ipureintro; exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptChildren_scalars
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_receipt_field_valid
