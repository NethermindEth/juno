import ProofCheck.ExecutionPointerSpec
import ProofCheck.L1MessagePointerSpec
import ProofCheck.EventPointersUnique
import ProofCheck.L2MessagePointersUnique
import ProofCheck.FeeUnitSpec
import ProofCheck.ReceiptStatusValues

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- The complete application receipt, independent of concrete pointer addresses. -/
structure ReceiptValue where
  fee : Option (GoArray w64 4)
  unit : w8
  events : Option (List (Option EventValue))
  resources : Option ExecutionValue
  message : Option L1MessageValue
  messages : Option (List (Option L2MessageValue))
  hash : Option (GoArray w64 4)
  reverted : Bool
  reason : GoString

def receiptZero : ReceiptValue := ⟨none, W8 0, none, none, none, none, none, false, []⟩

inductive ReceiptField where
  | fee | unit | events | resources | message | messages | hash | reverted | reason
  deriving DecidableEq

def receiptFields : List ReceiptField := [.fee, .unit, .events, .resources, .message, .messages, .hash, .reverted, .reason]
def receiptFieldName : ReceiptField → GoString
  | .fee => go!"Fee"
  | .unit => go!"FeeUnit"
  | .events => go!"Events"
  | .resources => go!"ExecutionResources"
  | .message => go!"L1ToL2Message"
  | .messages => go!"L2ToL1Message"
  | .hash => go!"TransactionHash"
  | .reverted => go!"Reverted"
  | .reason => go!"RevertReason"
def fullReceiptNames : List GoString := [go!"Fee", go!"FeeUnit", go!"Events", go!"ExecutionResources", go!"L1ToL2Message", go!"L2ToL1Message", go!"TransactionHash", go!"Reverted", go!"RevertReason"]

theorem receiptField_names : receiptFields.map receiptFieldName = fullReceiptNames := rfl

inductive ReceiptFieldValid (d : Nat) : ReceiptField → ReceiptValue → List w8 → List w8 → ReceiptValue → Prop where
  | fee {old : ReceiptValue} {item rest value} (valid : FeltValid d item value) :
      ReceiptFieldValid d .fee old item rest {old with fee := value}
  | unit {old : ReceiptValue} {item rest value} (valid : FeeUnitValid old.unit item value) :
      ReceiptFieldValid d .unit old item rest {old with unit := value}
  | events {old : ReceiptValue} {item rest value} (valid : EventPointersSpec d (item ++ rest) (.success value rest)) :
      ReceiptFieldValid d .events old item rest {old with events := value}
  | resources {old : ReceiptValue} {item rest value} (valid : ExecutionPointerValid d item rest value) :
      ReceiptFieldValid d .resources old item rest {old with resources := value}
  | message {old : ReceiptValue} {item rest value} (valid : L1MessagePointerValid d item rest value) :
      ReceiptFieldValid d .message old item rest {old with message := value}
  | messages {old : ReceiptValue} {item rest value} (valid : L2MessagePointersSpec d (item ++ rest) (.success value rest)) :
      ReceiptFieldValid d .messages old item rest {old with messages := value}
  | hash {old : ReceiptValue} {item rest value} (valid : FeltValid d item value) :
      ReceiptFieldValid d .hash old item rest {old with hash := value}
  | reverted {old : ReceiptValue} {item rest value} (valid : BoolValid old.reverted item value) :
      ReceiptFieldValid d .reverted old item rest {old with reverted := value}
  | reason {old : ReceiptValue} {item rest value} (valid : StringValid old.reason item value) :
      ReceiptFieldValid d .reason old item rest {old with reason := value}

inductive ReceiptFieldFailure where
  | felt (reason : FeltFailureReason)
  | unit (reason : FeeUnitFailureReason)
  | events (reason : EventPointersFailureReason)
  | resources (retained : ExecutionValue) (reason : ExecutionFailureReason)
  | message (retained : L1MessageValue) (reason : L1MessageFailureReason)
  | messages (reason : L2MessagePointersFailureReason)
  | boolean (reason : BoolFailureReason)
  | string (reason : StringFailureReason)

/-- Every failed field leaves the receipt destination unchanged. Private child
    allocations may be partial; their retained values are explicit for pointers.
    Offset denotes a diagnostic position except for FeeUnit overflow, which has
    no diagnostic: there it records the consumed-value boundary, not an error offset. -/
inductive ReceiptFieldRejected (d : Nat) : ReceiptField → List w8 → Nat → Nat → Nat → ReceiptFieldFailure → Prop where
  | fee {wire offset reason} (failure : FeltFailure d wire offset reason) :
      ReceiptFieldRejected d .fee wire 0 offset 0 (.felt reason)
  | hash {wire offset reason} (failure : FeltFailure d wire offset reason) :
      ReceiptFieldRejected d .hash wire 0 offset 0 (.felt reason)
  | unit {wire consumed reason} (failure : FeeUnitRejected wire consumed reason) :
      ReceiptFieldRejected d .unit wire consumed consumed 0 (.unit reason)
  | events {wire consumed offset extraDepth reason}
      (failure : EventPointersSpec d wire (.failure consumed offset extraDepth reason)) :
      ReceiptFieldRejected d .events wire consumed offset extraDepth (.events reason)
  | resources {wire retained consumed offset extraDepth reason}
      (failure : ExecutionPointerRejected d wire retained consumed offset extraDepth reason) :
      ReceiptFieldRejected d .resources wire consumed offset extraDepth (.resources retained reason)
  | message {wire retained consumed offset entered reason}
      (failure : L1MessagePointerRejected d wire retained consumed offset entered reason) :
      ReceiptFieldRejected d .message wire consumed offset (if entered then 1 else 0) (.message retained reason)
  | messages {wire consumed offset extraDepth reason}
      (failure : L2MessagePointersSpec d wire (.failure consumed offset extraDepth reason)) :
      ReceiptFieldRejected d .messages wire consumed offset extraDepth (.messages reason)
  | reverted {wire reason} (failure : BoolRejected wire reason) :
      ReceiptFieldRejected d .reverted wire 0 0 0 (.boolean reason)
  | reason {wire reason} (failure : StringRejected wire reason) :
      ReceiptFieldRejected d .reason wire 0 0 0 (.string reason)

theorem receiptField_classify (d : Nat) (field : ReceiptField) (old : ReceiptValue) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ ReceiptFieldValid d field old item rest final) ∨
    (∃ consumed offset extraDepth reason, ReceiptFieldRejected d field wire consumed offset extraDepth reason) := by
  cases field with
  | fee =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .fee valid⟩
    · exact Or.inr ⟨0, offset, 0, _, .fee failure⟩
  | hash =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .hash valid⟩
    · exact Or.inr ⟨0, offset, 0, _, .hash failure⟩
  | unit =>
    rcases feeUnit_classify old.unit wire with ⟨item, rest, value, split, valid⟩ | ⟨consumed, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .unit valid⟩
    · exact Or.inr ⟨consumed, consumed, 0, _, .unit failure⟩
  | reverted =>
    rcases bool_classify old.reverted wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .reverted valid⟩
    · exact Or.inr ⟨0, 0, 0, _, .reverted failure⟩
  | reason =>
    rcases string_classify old.reason wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .reason valid⟩
    · exact Or.inr ⟨0, 0, 0, _, .reason failure⟩
  | events =>
    obtain ⟨outcome, spec⟩ := eventPointers_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := eventPointers_success_suffix d wire rest value spec
      rw [split] at spec
      exact Or.inl ⟨item, rest, _, split, .events spec⟩
    | failure consumed offset extraDepth reason => exact Or.inr ⟨consumed, offset, extraDepth, _, .events spec⟩
  | messages =>
    obtain ⟨outcome, spec⟩ := l2MessagePointers_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := l2MessagePointers_success_suffix d wire rest value spec
      rw [split] at spec
      exact Or.inl ⟨item, rest, _, split, .messages spec⟩
    | failure consumed offset extraDepth reason => exact Or.inr ⟨consumed, offset, extraDepth, _, .messages spec⟩
  | resources =>
    rcases executionPointer_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨retained, consumed, offset, extraDepth, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .resources valid⟩
    · exact Or.inr ⟨consumed, offset, _, _, .resources failure⟩
  | message =>
    rcases l1MessagePointer_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨retained, consumed, offset, extraDepth, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .message valid⟩
    · exact Or.inr ⟨consumed, offset, _, _, .message failure⟩

theorem receiptField_nonempty (d : Nat) (field : ReceiptField) (old final : ReceiptValue)
    (item rest : List w8) (valid : ReceiptFieldValid d field old item rest final) : 0 < item.length := by
  cases valid with
  | fee valid => cases valid <;> simp
  | hash valid => cases valid <;> simp
  | unit valid => exact feeUnitValid_nonempty _ _ _ valid
  | reverted valid => rw [bool_valid_length _ _ _ valid]; decide
  | reason valid => exact stringValid_nonempty _ _ _ valid
  | resources valid => exact executionPointer_nonempty _ _ _ _ valid
  | message valid => exact l1MessagePointer_nonempty _ _ _ _ valid
  | events valid =>
    obtain ⟨other, same, positive⟩ := eventPointers_success_suffix d (item ++ rest) rest _ valid
    have eq := List.append_cancel_right same
    subst other
    exact positive
  | messages valid =>
    obtain ⟨other, same, positive⟩ := l2MessagePointers_success_suffix d (item ++ rest) rest _ valid
    have eq := List.append_cancel_right same
    subst other
    exact positive

theorem receiptField_prefix_unique (d : Nat) (field : ReceiptField) (old a b : ReceiptValue)
    (left right restA restB : List w8)
    (first : ReceiptFieldValid d field old left restA a) (second : ReceiptFieldValid d field old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | fee valid => cases second with
    | fee valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | hash valid => cases second with
    | hash valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | unit valid => cases second with
    | unit valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feeUnitValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | reverted valid => cases second with
    | reverted valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := bool_valid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | reason valid => cases second with
    | reason valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := stringValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | resources valid => cases second with
    | resources valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := executionPointer_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | message valid => cases second with
    | message valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := l1MessagePointer_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | events valid => cases second with
    | events valid2 =>
      rw [← same] at valid2
      have eq := eventPointersSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      have eq := List.append_cancel_right same
      subst right
      exact ⟨rfl, rfl, rfl⟩
  | messages valid => cases second with
    | messages valid2 =>
      rw [← same] at valid2
      have eq := l2MessagePointersSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      have eq := List.append_cancel_right same
      subst right
      exact ⟨rfl, rfl, rfl⟩

theorem receiptField_not_rejected (d : Nat) (field : ReceiptField) (old final : ReceiptValue)
    (item rest : List w8) (consumed offset extraDepth : Nat) (reason : ReceiptFieldFailure)
    (valid : ReceiptFieldValid d field old item rest final)
    (failure : ReceiptFieldRejected d field (item ++ rest) consumed offset extraDepth reason) : False := by
  cases valid with
  | fee valid => cases failure with
    | fee failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure
  | hash valid => cases failure with
    | hash failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure
  | unit valid => cases failure with
    | unit failure => exact feeUnitValid_not_rejected _ _ _ _ _ _ valid failure
  | reverted valid => cases failure with
    | reverted failure => exact bool_valid_not_rejected _ _ _ _ _ valid failure
  | reason valid => cases failure with
    | reason failure => exact stringValid_not_rejected _ _ _ _ _ valid failure
  | resources valid => cases failure with
    | resources failure => exact executionPointer_not_rejected _ _ _ _ _ _ _ _ _ valid failure
  | message valid => cases failure with
    | message failure => exact l1MessagePointer_not_rejected _ _ _ _ _ _ _ _ _ valid failure
  | events valid => cases failure with
    | events failure => have eq := eventPointersSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | messages valid => cases failure with
    | messages failure => have eq := l2MessagePointersSpec_unique d (item ++ rest) _ _ valid failure; cases eq

theorem receiptField_rejected_unique (d : Nat) (field : ReceiptField) (wire : List w8)
    (consumedA consumedB offsetA offsetB depthA depthB : Nat) (a b : ReceiptFieldFailure)
    (first : ReceiptFieldRejected d field wire consumedA offsetA depthA a)
    (second : ReceiptFieldRejected d field wire consumedB offsetB depthB b) :
    consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b := by
  cases first with
  | fee failure => cases second with
    | fee failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
  | hash failure => cases second with
    | hash failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
  | unit failure => cases second with
    | unit failure2 =>
      obtain ⟨rfl, rfl⟩ := feeUnitRejected_unique _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
  | reverted failure => cases second with
    | reverted failure2 =>
      obtain rfl := bool_rejected_unique _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
  | reason failure => cases second with
    | reason failure2 =>
      obtain rfl := stringRejected_reason_unique _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
  | resources failure => cases second with
    | resources failure2 =>
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := executionPointerRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
  | message failure => cases second with
    | message failure2 =>
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := l1MessagePointerRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
  | events failure => cases second with
    | events failure2 =>
      have eq := eventPointersSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl⟩
  | messages failure => cases second with
    | messages failure2 =>
      have eq := l2MessagePointersSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl⟩

theorem receiptField_rejected_bounds (d : Nat) (field : ReceiptField) (wire : List w8)
    (consumed offset extraDepth : Nat) (reason : ReceiptFieldFailure)
    (failure : ReceiptFieldRejected d field wire consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | fee failure => exact ⟨Nat.zero_le _, feltFailure_offset _ _ _ _ failure⟩
  | hash failure => exact ⟨Nat.zero_le _, feltFailure_offset _ _ _ _ failure⟩
  | unit failure => exact ⟨Nat.le_refl _, feeUnitRejected_bound _ _ _ failure⟩
  | events failure => exact eventPointers_failure_bounds _ _ _ _ _ _ failure
  | messages failure => exact l2MessagePointers_failure_bounds _ _ _ _ _ _ failure
  | resources failure => exact executionPointerRejected_bounds _ _ _ _ _ _ _ failure
  | message failure => exact l1MessagePointerRejected_bounds _ _ _ _ _ _ _ failure
  | reverted => omega
  | reason => omega

theorem receiptField_rejected_depth (d : Nat) (field : ReceiptField) (wire : List w8)
    (consumed offset extraDepth : Nat) (reason : ReceiptFieldFailure) (depth : d ≤ 32)
    (failure : ReceiptFieldRejected d field wire consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  cases failure with
  | fee => omega
  | hash => omega
  | unit => omega
  | reverted => omega
  | reason => omega
  | events failure => exact eventPointers_failure_depth _ _ _ _ _ _ depth failure
  | messages failure => exact l2MessagePointers_failure_depth _ _ _ _ _ _ depth failure
  | resources failure => cases failure with
    | record hn hu failure => exact executionRecordRejected_depth _ _ _ _ _ _ _ _ depth failure
  | message failure => cases failure with
    | record hn hu failure => cases failure <;> simp_all <;> omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptField_names
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptField_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptField_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptField_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptField_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptField_rejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptField_rejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptField_rejected_depth
