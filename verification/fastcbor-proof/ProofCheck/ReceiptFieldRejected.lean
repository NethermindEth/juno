import ProofCheck.ReceiptFieldValid

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

local instance (kinds : FeltErrorKinds) : Persistent (FeltErrorGlobals (GF := GF) kinds) := by
  unfold FeltErrorGlobals SkipErrorGlobals
  infer_instance

/-- Structured failures own their actual diagnostic and private pointer-record
    state. FeeUnit overflow instead returns the initialized bare sentinel; its
    supplied boundary is intentionally not interpreted as a diagnostic offset. -/
def ReceiptFieldError (result : GoInterface) (pos : w64) : ReceiptFieldFailure → IProp GF
  | .unit reason => iprop(∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗ FeeUnitError result pos kinds reason)
  | .felt reason => iprop(∃ err : Loc, ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗ FeltFailureError err pos reason (feltFailureKind reason kinds))
  | .events reason => iprop(∃ err : Loc, ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      EventPointersFailureOwned err pos reason)
  | .resources retained reason => iprop(∃ err : Loc, ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      (∃ temporary availability gas : Loc, ExecutionOwned temporary availability gas retained) ∗ ExecutionFailureOwned err pos reason)
  | .message retained reason => iprop(∃ err : Loc, ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      (∃ temporary : Loc, L1MessageOwned temporary retained) ∗ L1MessageFailureOwned err pos reason)
  | .messages reason => iprop(∃ err : Loc, ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      L2MessagePointersFailureOwned err pos reason)
  | .boolean reason => iprop(∃ err : Loc, ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗ BoolFailureError err pos reason (expectedFailureKind (boolExpectedReason reason) kinds))
  | .string reason => iprop(∃ err : Loc, ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗ StringFailureError err pos reason (stringFailureKind reason kinds))

theorem receiptFieldError_nonNil (result : GoInterface) (pos : w64) (reason : ReceiptFieldFailure) :
    ReceiptFieldError (GF := GF) result pos reason ⊢ ⌜result ≠ interface.nil⌝ := by
  cases reason with
  | unit reason =>
    isimp only [ReceiptFieldError]
    iintro ⟨%kinds, _, Herr⟩
    iapply feeUnitError_nonNil result pos kinds reason
    iexact Herr
  | felt | events | resources | message | messages | boolean | string =>
    isimp only [ReceiptFieldError]
    iintro ⟨%err, %Heq, _⟩
    ipureintro; rw [Heq]; simp

/-- Unsigned parse failures consume zero. Overflow ignores the position because
    its actual error is bare; moving to the consumed boundary invents no offset. -/
theorem feeUnitError_boundary (result : GoInterface) (pos : w64) (kinds : FeltErrorKinds)
    (wire : List w8) (consumed : Nat) (reason : FeeUnitFailureReason)
    (failure : FeeUnitRejected wire consumed reason) :
    FeeUnitError (GF := GF) result pos kinds reason ⊢ FeeUnitError result (pos + W64 consumed) kinds reason := by
  cases failure with
  | expected =>
    iintro Herr
    isimp only [Int.natCast_zero, show pos + W64 0 = pos from by word]
    iexact Herr
  | overflow =>
    iintro Herr
    isimp only [FeeUnitError] at Herr
    isimp only [FeeUnitError]
    iexact Herr

theorem wp_receipt_field_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : ReceiptValue) (field : ReceiptField) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d consumed offset extraDepth : Nat) (reason : ReceiptFieldFailure)
    (failure : ReceiptFieldRejected d field wire consumed offset extraDepth reason)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       ReceiptOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (receiptFieldReader field))) (Val #c)) (Val #(structFieldRef TransactionReceipt (receiptFieldName field) dst)))
    {{ (result : GoInterface), RET #result; ReceiptOwned dst old ∗
       c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
       v.buf' ↦*{dq} xs ∗ ReceiptFieldError result (v.pos' + W64 offset) reason ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  iintro %Φ ⟨#Hreceipts, #Hschema, Howned, Hc, Hs⟩ HΦ
  isimp only [ReceiptOwned] at Howned
  icases Howned with ⟨%h, Hdst, Hchildren⟩
  ihave Hglobals := header_initialized_felt_globals (GF := GF) $$ Hschema
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  cases failure with
  | @fee wire offset why failure =>
    simp only [receiptFieldReader, receiptFieldName]
    icases receipt_fee_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    ihave #Hkind := feltErrorGlobals_lookup kinds why $$ Hglobals
    wp_apply wp_readFelt_failure_bounded c (structFieldRef TransactionReceipt go!"Fee" dst) h.Fee' v xs wire dq d offset why (feltFailureKind why kinds) failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hkind Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.Fee' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold ReceiptOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [ReceiptFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iexists kinds
      iframe Hglobals Herr
    ipureintro; exact ⟨trivial, Hnext.1, by rw [Hnext.1]; omega, Hnext.2⟩
  | @hash wire offset why failure =>
    simp only [receiptFieldReader, receiptFieldName]
    icases receipt_hash_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    ihave #Hkind := feltErrorGlobals_lookup kinds why $$ Hglobals
    wp_apply wp_readFelt_failure_bounded c (structFieldRef TransactionReceipt go!"TransactionHash" dst) h.TransactionHash' v xs wire dq d offset why (feltFailureKind why kinds) failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hkind Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.TransactionHash' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold ReceiptOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [ReceiptFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iexists kinds
      iframe Hglobals Herr
    ipureintro; exact ⟨trivial, Hnext.1, by rw [Hnext.1]; omega, Hnext.2⟩
  | @events wire consumed offset extraDepth why failure =>
    simp only [receiptFieldReader, receiptFieldName]
    icases receipt_events_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readEventPointers_rejected validator c (structFieldRef TransactionReceipt go!"Events" dst) h.Events' v xs wire dq d consumed offset extraDepth why failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.Events' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold ReceiptOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [ReceiptFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iexact Herr
    ipureintro; exact ⟨Hnext.1, Hnext.2.1, Hnext.2.2.1, Hnext.2.2.2.1⟩
  | @messages wire consumed offset extraDepth why failure =>
    simp only [receiptFieldReader, receiptFieldName]
    icases receipt_messages_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readL2MessagePointers_rejected validator c (structFieldRef TransactionReceipt go!"L2ToL1Message" dst) h.L2ToL1Message' v xs wire dq d consumed offset extraDepth why failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.L2ToL1Message' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold ReceiptOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [ReceiptFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iexact Herr
    ipureintro; exact ⟨Hnext.1, Hnext.2.1, Hnext.2.2.1, Hnext.2.2.2.1⟩
  | @resources wire retained consumed offset extraDepth why failure =>
    simp only [receiptFieldReader, receiptFieldName]
    icases receipt_resources_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readExecutionPointer_rejected validator c (structFieldRef TransactionReceipt go!"ExecutionResources" dst) h.ExecutionResources' v xs wire retained dq d consumed offset extraDepth why failure hd hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hprivate, Hc, Hs, Herr, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.ExecutionResources' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold ReceiptOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr Hprivate]
    · isimp only [ReceiptFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iframe Hprivate Herr
    ipureintro; exact Hnext
  | @message wire retained consumed offset entered why failure =>
    simp only [receiptFieldReader, receiptFieldName]
    icases receipt_message_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readL1MessagePointer_rejected validator c (structFieldRef TransactionReceipt go!"L1ToL2Message" dst) h.L1ToL2Message' v xs wire retained dq d consumed offset entered why failure hd hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hprivate, Hc, Hs, Herr, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.L1ToL2Message' Hfield
    have hdep : (if entered then v.depth' + W64 1 else v.depth') = v.depth' + W64 (if entered then 1 else 0 : Nat) := by
      cases entered <;> simp <;> word
    isimp only [hdep] at Hc
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold ReceiptOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr Hprivate]
    · isimp only [ReceiptFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iframe Hprivate Herr
    ipureintro; exact Hnext
  | @reverted wire why failure =>
    simp only [receiptFieldReader, receiptFieldName]
    icases receipt_reverted_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    ihave #Hkind := feltErrorGlobals_lookup kinds (.expected (W8 224) (boolExpectedReason why)) $$ Hglobals
    isimp only [feltFailureAddr, feltFailureKind] at Hkind
    wp_apply wp_readBool_rejected c (structFieldRef TransactionReceipt go!"Reverted" dst) h.Reverted' v xs wire dq why (expectedFailureKind (boolExpectedReason why) kinds) failure hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr⟩
    · iframe Hkind Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.Reverted' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold ReceiptOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [ReceiptFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iexists kinds
      iframe Hglobals Herr
    ipureintro; exact ⟨trivial, trivial, Int.le_refl _, hp.2⟩
  | @reason wire why failure =>
    simp only [receiptFieldReader, receiptFieldName]
    icases receipt_reason_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    ihave #Hkind := stringErrorGlobals_lookup kinds why $$ Hglobals
    wp_apply wp_readString_rejected validator c (structFieldRef TransactionReceipt go!"RevertReason" dst) h.RevertReason' v xs wire dq why (stringFailureKind why kinds) failure hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr⟩
    · iframe Hkind Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.RevertReason' Hfield
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold ReceiptOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [ReceiptFieldError]
      iexists err
      isplitr
      · ipureintro; rfl
      iexists kinds
      iframe Hglobals Herr
    ipureintro; exact ⟨trivial, trivial, Int.le_refl _, hp.2⟩
  | @unit wire consumed why failure =>
    simp only [receiptFieldReader, receiptFieldName]
    icases receipt_unit_acc dst h $$ Hdst with ⟨Hfield, Hclose⟩
    wp_apply wp_readFeeUnit_rejected c (structFieldRef TransactionReceipt go!"FeeUnit" dst) h.FeeUnit' v xs wire dq consumed why kinds failure hp hbytes
      $$ [Hfield Hc Hs] as %result ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hglobals Hfield Hc Hs
    ihave Hdst := Hclose $$ %h.FeeUnit' Hfield
    ihave Herr := feeUnitError_boundary result v.pos' kinds wire consumed why failure $$ Herr
    iapply HΦ
    try isimp only [Int.natCast_zero, Int.add_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word]
    iframe Hc Hs
    isplitl [Hdst Hchildren]
    · unfold ReceiptOwned; iexists h; iframe Hdst Hchildren
    isplitl [Herr]
    · isimp only [ReceiptFieldError]
      iexists kinds
      iframe Hglobals Herr
    ipureintro; exact ⟨Hnext.1, Hnext.1, Int.le_refl _, Hnext.2⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptFieldError_nonNil
#print axioms Perennial.github_com.NethermindEth.juno.core.feeUnitError_boundary
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_receipt_field_rejected
